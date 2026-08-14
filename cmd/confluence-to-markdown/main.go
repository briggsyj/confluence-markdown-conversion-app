// Command confluence-to-markdown converts a Confluence space to Markdown,
// either from a static HTML space export or by pulling live from the
// Confluence Cloud REST API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/briggsyj/confluence-to-markdown/internal/convert"
	"github.com/briggsyj/confluence-to-markdown/internal/linkfix"
	apisource "github.com/briggsyj/confluence-to-markdown/internal/source/api"
	exportsource "github.com/briggsyj/confluence-to-markdown/internal/source/export"
	"github.com/briggsyj/confluence-to-markdown/internal/writer"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "confluence-to-markdown",
		Short: "Convert a Confluence space to Markdown",
	}
	root.AddCommand(exportCmd(), apiCmd())
	return root
}

// -- shared post-process flags -------------------------------------------

type postProcessFlags struct {
	postProcess     bool
	confluenceURL   string
	noArticleDir    bool
	noFileSizeLimit bool
}

func addPostProcessFlags(cmd *cobra.Command, f *postProcessFlags) {
	cmd.Flags().BoolVar(&f.postProcess, "post-process", true,
		"rewrite internal links and reorganize attachments after conversion (legacy-cs defaulted this off; here it defaults on)")
	cmd.Flags().StringVar(&f.confluenceURL, "confluence-url", "",
		"Confluence space URL prefix, used to recognize raw links in export-source HTML (e.g. https://yourdomain.atlassian.net/wiki/spaces/)")
	cmd.Flags().BoolVar(&f.noArticleDir, "no-article-dir", false, "don't move each page into its own <Title>/<Title>.md directory")
	cmd.Flags().BoolVar(&f.noFileSizeLimit, "no-file-size-limit", false, "don't skip attachments over 10MB")
}

func runPostProcess(f *postProcessFlags, outputPath, space string) error {
	if !f.postProcess {
		return nil
	}
	slog.Info("running link/attachment post-process")
	return linkfix.Run(outputPath, space, f.confluenceURL, linkfix.Options{
		ArticleDir:      !f.noArticleDir,
		NoFileSizeLimit: f.noFileSizeLimit,
	}, slog.Default())
}

// -- export subcommand ----------------------------------------------------

func exportCmd() *cobra.Command {
	var (
		inputPath  string
		outputPath string
		pp         postProcessFlags
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Convert a static Confluence HTML space export",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Transparently handles both a .zip (Confluence's raw "Export
			// Space" download) and an already-extracted directory; the
			// resolved dir must stay alive through Write too (attachment
			// copying reads from it), not just Load.
			resolvedInput, cleanup, err := exportsource.ResolveInput(inputPath)
			if err != nil {
				return fmt.Errorf("resolving input %q: %w", inputPath, err)
			}
			defer cleanup()

			src := exportsource.New(resolvedInput)
			result, err := src.Load(ctx)
			if err != nil {
				return fmt.Errorf("loading export: %w", err)
			}
			slog.Info("loaded pages from export", "count", len(result.Pages))

			if err := writer.Write(result, outputPath, convert.New(), slog.Default()); err != nil {
				return fmt.Errorf("writing markdown: %w", err)
			}

			space := result.RootSpace
			return runPostProcess(&pp, outputPath, space)
		},
	}

	cmd.Flags().StringVar(&inputPath, "input", "", "path to a Confluence HTML space export - either a .zip or an already-extracted directory (required)")
	cmd.Flags().StringVar(&outputPath, "output", "", "directory to write Markdown output to (required)")
	addPostProcessFlags(cmd, &pp)
	cmd.MarkFlagRequired("input")
	cmd.MarkFlagRequired("output")

	return cmd
}

// -- api subcommand ---------------------------------------------------------

func apiCmd() *cobra.Command {
	var (
		cfg        apisource.Config
		outputPath string
		pp         postProcessFlags
	)

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Convert a Confluence space by pulling live from the Cloud REST API",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if pp.confluenceURL == "" {
				pp.confluenceURL = cfg.Site
			}

			src, err := apisource.New(cfg)
			if err != nil {
				return fmt.Errorf("creating API source: %w", err)
			}
			result, err := src.Load(ctx)
			if err != nil {
				return fmt.Errorf("loading from API: %w", err)
			}
			slog.Info("loaded pages from API", "count", len(result.Pages))

			if err := writer.Write(result, outputPath, convert.New(), slog.Default()); err != nil {
				return fmt.Errorf("writing markdown: %w", err)
			}

			return runPostProcess(&pp, outputPath, result.RootSpace)
		},
	}

	cmd.Flags().StringVar(&cfg.Site, "site", "", "Confluence Cloud base URL, e.g. https://yourdomain.atlassian.net (required)")
	cmd.Flags().StringVar(&cfg.Email, "email", "", "Atlassian account email (required)")
	cmd.Flags().StringVar(&cfg.APIToken, "api-token", "", "Atlassian API token (required)")
	cmd.Flags().StringVar(&cfg.SpaceKey, "space", "", "Confluence space key to convert (required)")
	cmd.Flags().StringVar(&outputPath, "output", "", "directory to write Markdown output to (required)")
	addPostProcessFlags(cmd, &pp)
	for _, name := range []string{"site", "email", "api-token", "space", "output"} {
		cmd.MarkFlagRequired(name)
	}

	return cmd
}
