// Command confluence-to-markdown-gui is a desktop GUI (Fyne) for the
// export-source conversion flow: drop a Confluence HTML space export
// (either the raw .zip or an already-extracted folder) onto the window, or
// browse for one, pick an output directory, adjust options, and convert.
//
// This wraps the same internal packages the CLI (confluence-to-markdown
// export) uses - it's a thin UI layer, not a separate implementation.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/briggsyj/confluence-to-markdown/internal/convert"
	"github.com/briggsyj/confluence-to-markdown/internal/linkfix"
	exportsource "github.com/briggsyj/confluence-to-markdown/internal/source/export"
	"github.com/briggsyj/confluence-to-markdown/internal/writer"
)

func main() {
	a := app.NewWithID("dev.briggsyj.confluence-to-markdown-gui")
	w := a.NewWindow("Confluence Markdown Conversion App")
	w.Resize(fyne.NewSize(620, 660))

	inputEntry := widget.NewEntry()
	inputEntry.SetPlaceHolder("Drop a Confluence export .zip or folder here, or browse...")

	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("Where to write the converted Markdown")

	confluenceURLEntry := widget.NewEntry()
	confluenceURLEntry.SetPlaceHolder("https://yourdomain.atlassian.net/wiki/spaces/")

	confluenceURLHelp := widget.NewLabel(
		"Optional. Your Confluence space URL prefix. When set, absolute Confluence links in " +
			"the exported HTML (full https://...atlassian.net/wiki/spaces/... URLs) are recognized " +
			"and rewritten into local links between the converted Markdown pages. Leave blank if the " +
			"export contains no such absolute links.")
	confluenceURLHelp.Wrapping = fyne.TextWrapWord
	confluenceURLHelp.TextStyle = fyne.TextStyle{Italic: true}

	postProcessCheck := widget.NewCheck("Rewrite internal links and reorganize attachments after conversion", nil)
	postProcessCheck.SetChecked(true)
	// Positive-phrased, off by default: leave pages where they are converted
	// unless the user opts into the per-article directory layout.
	articleDirCheck := widget.NewCheck("Move each page to its own /Title/ directory", nil)
	// Positive-phrased, on by default: enforce the 10MB attachment guard
	// (skipped files are reported as warnings in the log below).
	skipLargeCheck := widget.NewCheck("Skip attachments over 10MB", nil)
	skipLargeCheck.SetChecked(true)

	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	progress := widget.NewProgressBarInfinite()
	progress.Hide()

	logView := widget.NewLabel("")
	logView.Wrapping = fyne.TextWrapWord
	logView.TextStyle = fyne.TextStyle{Monospace: true}
	logScroll := container.NewVScroll(logView)
	logScroll.SetMinSize(fyne.NewSize(0, 200))

	// appendLog marshals every log line onto Fyne's UI thread, since
	// conversion (and therefore the logger) runs on a worker goroutine.
	appendLog := func(line string) {
		fyne.Do(func() {
			if logView.Text == "" {
				logView.SetText(line)
			} else {
				logView.SetText(logView.Text + "\n" + line)
			}
			logScroll.ScrollToBottom()
		})
	}
	logger := slog.New(guiLogHandler{sink: appendLog})

	browseZip := widget.NewButton("Browse Zip...", func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			defer r.Close()
			inputEntry.SetText(r.URI().Path())
		}, w)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".zip"}))
		d.Show()
	})

	browseFolder := widget.NewButton("Browse Folder...", func() {
		dialog.ShowFolderOpen(func(u fyne.ListableURI, err error) {
			if err != nil || u == nil {
				return
			}
			inputEntry.SetText(u.Path())
		}, w)
	})

	browseOutput := widget.NewButton("Browse...", func() {
		dialog.ShowFolderOpen(func(u fyne.ListableURI, err error) {
			if err != nil || u == nil {
				return
			}
			outputEntry.SetText(u.Path())
		}, w)
	})

	var convertBtn *widget.Button
	convertBtn = widget.NewButton("Convert", func() {
		input := inputEntry.Text
		output := outputEntry.Text
		if input == "" || output == "" {
			dialog.ShowInformation("Missing input", "Please choose both an input export and an output directory.", w)
			return
		}

		convertBtn.Disable()
		logView.SetText("")
		status.SetText("Converting...")
		progress.Show()

		opts := runOptions{
			input:           input,
			output:          output,
			confluenceURL:   confluenceURLEntry.Text,
			postProcess:     postProcessCheck.Checked,
			noArticleDir:    !articleDirCheck.Checked,
			noFileSizeLimit: !skipLargeCheck.Checked,
		}

		go func() {
			pageCount, err := runConversion(opts, logger)
			fyne.Do(func() {
				progress.Hide()
				convertBtn.Enable()
				if err != nil {
					status.SetText("Failed: " + err.Error())
					dialog.ShowError(err, w)
					return
				}
				// Report completion inline (status label + log) rather than
				// interrupting with a popup.
				status.SetText(fmt.Sprintf("Converted %d pages to %s", pageCount, output))
			})
		}()
	})

	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) == 0 {
			return
		}
		inputEntry.SetText(uris[0].Path())
	})

	controls := container.NewVBox(
		widget.NewLabelWithStyle("Input export", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, container.NewHBox(browseZip, browseFolder), inputEntry),
		widget.NewLabelWithStyle("Output directory", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, browseOutput, outputEntry),
		widget.NewLabelWithStyle("Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Confluence space URL (optional)"),
		confluenceURLEntry,
		confluenceURLHelp,
		postProcessCheck,
		articleDirCheck,
		skipLargeCheck,
		convertBtn,
		progress,
		status,
		widget.NewLabelWithStyle("Log", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	// Border layout so the log panel expands to fill the remaining space
	// below the fixed-height controls.
	w.SetContent(container.NewPadded(container.NewBorder(controls, nil, nil, nil, logScroll)))
	w.ShowAndRun()
}

type runOptions struct {
	input           string
	output          string
	confluenceURL   string
	postProcess     bool
	noArticleDir    bool
	noFileSizeLimit bool
}

// runConversion runs the same export -> convert -> write -> link-fix
// pipeline as "confluence-to-markdown export", logging progress to logger
// and returning the number of pages converted.
func runConversion(opts runOptions, logger *slog.Logger) (int, error) {
	ctx := context.Background()

	resolvedInput, cleanup, err := exportsource.ResolveInput(opts.input)
	if err != nil {
		return 0, fmt.Errorf("resolving input: %w", err)
	}
	defer cleanup()

	result, err := exportsource.New(resolvedInput).Load(ctx)
	if err != nil {
		return 0, fmt.Errorf("loading export: %w", err)
	}
	logger.Info("loaded pages from export", "count", len(result.Pages))

	if err := writer.Write(result, opts.output, convert.New(), logger); err != nil {
		return 0, fmt.Errorf("writing markdown: %w", err)
	}

	if opts.postProcess {
		logger.Info("rewriting internal links and reorganizing attachments")
		err := linkfix.Run(opts.output, result.RootSpace, opts.confluenceURL, linkfix.Options{
			ArticleDir:      !opts.noArticleDir,
			NoFileSizeLimit: opts.noFileSizeLimit,
		}, logger)
		if err != nil {
			return 0, fmt.Errorf("post-processing: %w", err)
		}
	}

	logger.Info("conversion complete", "pages", len(result.Pages), "output", opts.output)
	return len(result.Pages), nil
}

// logSink appends one formatted line to the GUI log panel. It is always
// invoked on Fyne's UI thread by guiLogHandler.
type logSink func(string)

// guiLogHandler is a slog.Handler that renders each record as a single line
// and hands it to a sink for display in the GUI's log panel. WARN and above
// are prefixed with the level so skipped-attachment warnings stand out.
type guiLogHandler struct {
	sink logSink
}

func (h guiLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h guiLogHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	if r.Level >= slog.LevelWarn {
		b.WriteString(r.Level.String())
		b.WriteString(": ")
	}
	b.WriteString(r.Message)
	r.Attrs(func(attr slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", attr.Key, attr.Value.Any())
		return true
	})
	h.sink(b.String())
	return nil
}

func (h guiLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h guiLogHandler) WithGroup(string) slog.Handler      { return h }
