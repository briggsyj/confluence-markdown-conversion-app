// Package writer converts a source.Result into Markdown files on disk,
// matching legacy-cs App.convert's per-page write loop and asset copying,
// with one deliberate improvement: frontmatter is written directly as
// plain YAML rather than legacy-cs's "<div>" HTML hack (which existed only
// to smuggle metadata through turndown's HTML-in/MD-out pipeline - with
// the space/ID already known at write time here, there's no need for it,
// and it removes the need for the "\---" escape/unescape dance entirely).
package writer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/otiai10/copy"

	"github.com/briggsyj/confluence-to-markdown/internal/convert"
	"github.com/briggsyj/confluence-to-markdown/internal/page"
	"github.com/briggsyj/confluence-to-markdown/internal/source"
)

// Write converts every page in result to a Markdown file under outDir and
// copies its attachments/asset directories alongside it.
func Write(result source.Result, outDir string, conv *convert.Converter) error {
	for _, pg := range result.Pages {
		if err := writePage(pg, outDir, conv); err != nil {
			return fmt.Errorf("writing page %q (%s): %w", pg.Title, pg.ConfluenceID, err)
		}
	}

	for _, ad := range result.AssetDirs {
		if err := copyAssetDir(ad, outDir); err != nil {
			return fmt.Errorf("copying assets for space %q: %w", ad.Space, err)
		}
	}

	return nil
}

func writePage(pg page.Page, outDir string, conv *convert.Converter) error {
	md, err := conv.Convert(pg.HTML)
	if err != nil {
		return err
	}

	localDir := page.LocalDirFromSegments(pg.Ancestors)
	fileName := page.FileNameNew(pg.IsIndex, pg.Title)
	outPath := filepath.Join(outDir, pg.Space, localDir, fileName)

	content := frontMatter(pg.ConfluenceID, pg.Space) + "\n" + md

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return err
	}

	for _, att := range pg.Attachments {
		dst := filepath.Join(filepath.Dir(outPath), att.FileName)
		if err := copy.Copy(att.LocalPath, dst); err != nil {
			return fmt.Errorf("copying attachment %q: %w", att.FileName, err)
		}
	}
	return nil
}

func frontMatter(confluenceID, space string) string {
	return "---\nconfluence-id: " + confluenceID + "\nconfluence-space: " + space + "\n---\n"
}

// copyAssetDir replicates legacy-cs Utils.copyAssets's exact destination
// computation, including its "dirname" quirk: when a space's index page
// has no local-dir segments (the common single-space-export case), assets
// land in outDir itself rather than outDir/<space> - carried over
// faithfully rather than silently changed, since existing consumers of a
// legacy-cs-produced tree may already depend on it. See linkfix's docs for
// the analogous root-relative-path convention.
func copyAssetDir(ad source.AssetDir, outDir string) error {
	assetDir := filepath.Join(outDir, ad.Space, ad.LocalDir)
	dst := filepath.Dir(assetDir)

	for _, sub := range []string{"images", "attachments"} {
		src := filepath.Join(ad.SourcePath, sub)
		info, err := os.Stat(src)
		if err != nil || !info.IsDir() {
			continue
		}
		if err := copy.Copy(src, filepath.Join(dst, sub)); err != nil {
			return err
		}
	}
	return nil
}
