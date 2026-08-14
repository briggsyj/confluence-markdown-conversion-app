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
	w.Resize(fyne.NewSize(560, 420))

	inputEntry := widget.NewEntry()
	inputEntry.SetPlaceHolder("Drop a Confluence export .zip or folder here, or browse...")

	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("Where to write the converted Markdown")

	confluenceURLEntry := widget.NewEntry()
	confluenceURLEntry.SetPlaceHolder("https://yourdomain.atlassian.net/wiki/spaces/ (optional)")

	postProcessCheck := widget.NewCheck("Rewrite internal links and reorganize attachments after conversion", nil)
	postProcessCheck.SetChecked(true)
	noArticleDirCheck := widget.NewCheck("Don't move each page into its own <Title>/<Title>.md directory", nil)
	noFileSizeLimitCheck := widget.NewCheck("Don't skip attachments over 10MB", nil)

	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

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
		status.SetText("Converting...")
		progress := dialog.NewCustomWithoutButtons("Converting", container.NewVBox(
			widget.NewLabel("Converting the Confluence export to Markdown..."),
			widget.NewProgressBarInfinite(),
		), w)
		progress.Show()

		opts := runOptions{
			input:           input,
			output:          output,
			confluenceURL:   confluenceURLEntry.Text,
			postProcess:     postProcessCheck.Checked,
			noArticleDir:    noArticleDirCheck.Checked,
			noFileSizeLimit: noFileSizeLimitCheck.Checked,
		}

		go func() {
			pageCount, err := runConversion(opts)
			fyne.Do(func() {
				progress.Hide()
				convertBtn.Enable()
				if err != nil {
					status.SetText("Failed: " + err.Error())
					dialog.ShowError(err, w)
					return
				}
				msg := fmt.Sprintf("Converted %d pages to %s", pageCount, output)
				status.SetText(msg)
				dialog.ShowInformation("Done", msg, w)
			})
		}()
	})

	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) == 0 {
			return
		}
		inputEntry.SetText(uris[0].Path())
	})

	form := container.NewVBox(
		widget.NewLabelWithStyle("Input export", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, container.NewHBox(browseZip, browseFolder), inputEntry),
		widget.NewLabelWithStyle("Output directory", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, browseOutput, outputEntry),
		widget.NewLabelWithStyle("Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		confluenceURLEntry,
		postProcessCheck,
		noArticleDirCheck,
		noFileSizeLimitCheck,
		convertBtn,
		status,
	)

	w.SetContent(container.NewPadded(form))
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
// pipeline as "confluence-to-markdown export", returning the number of
// pages converted.
func runConversion(opts runOptions) (int, error) {
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

	if err := writer.Write(result, opts.output, convert.New()); err != nil {
		return 0, fmt.Errorf("writing markdown: %w", err)
	}

	if opts.postProcess {
		err := linkfix.Run(opts.output, result.RootSpace, opts.confluenceURL, linkfix.Options{
			ArticleDir:      !opts.noArticleDir,
			NoFileSizeLimit: opts.noFileSizeLimit,
		})
		if err != nil {
			return 0, fmt.Errorf("post-processing: %w", err)
		}
	}

	return len(result.Pages), nil
}
