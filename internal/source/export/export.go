// Package export implements source.Source over a static Confluence HTML
// space export directory, matching legacy-cs's App.convert/Page.coffee.
package export

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/briggsyj/confluence-to-markdown/internal/clean"
	"github.com/briggsyj/confluence-to-markdown/internal/page"
	"github.com/briggsyj/confluence-to-markdown/internal/source"
)

// Source parses an extracted Confluence "Export Space" HTML+attachments
// directory tree.
type Source struct {
	// RootPath is the directory (or single file) to convert, mirroring
	// legacy-cs's pathResource argument.
	RootPath string
}

func New(rootPath string) *Source {
	return &Source{RootPath: rootPath}
}

// Load walks RootPath, parses every export HTML page, and runs the export
// cleaner pipeline on each, matching legacy-cs App.convert's per-page loop.
func (s *Source) Load(_ context.Context) (source.Result, error) {
	paths, err := listHTMLFiles(s.RootPath)
	if err != nil {
		return source.Result{}, err
	}

	var result source.Result
	for _, p := range paths {
		pg, isIndex, indexSpaceKey, err := parsePage(p)
		if err != nil {
			return source.Result{}, fmt.Errorf("parsing %s: %w", p, err)
		}
		result.Pages = append(result.Pages, pg)

		if isIndex {
			localDir := page.LocalDirFromSegments(pg.Ancestors)
			result.AssetDirs = append(result.AssetDirs, source.AssetDir{
				Space:      pg.Space,
				LocalDir:   localDir,
				SourcePath: filepath.Dir(p),
			})
			if indexSpaceKey != "" {
				result.RootSpace = indexSpaceKey
			}
		}
	}
	return result, nil
}

// listHTMLFiles returns every ".html" file under root, excluding any path
// that mentions "attachments", matching legacy-cs Utils.readDirRecursive's
// use in App.convert: pages are matched by
// "!filePath.includes('attachments') && filePath.endsWith('.html')".
func listHTMLFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(root, ".html") && !strings.Contains(root, "attachments") {
			return []string{root}, nil
		}
		return nil, nil
	}

	var paths []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".html") && !strings.Contains(p, "attachments") {
			paths = append(paths, p)
		}
		return nil
	})
	return paths, err
}

// parsePage reads and cleans a single export HTML file into a page.Page,
// matching legacy-cs Page.coffee's init()/getHeading()/getTextToConvert().
// It also returns the space key read from the index page's detail table
// (empty for non-index pages), matching legacy-cs Page.getIndexSpace.
func parsePage(path string) (page.Page, bool, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return page.Page{}, false, "", err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return page.Page{}, false, "", err
	}

	fileName := filepath.Base(path)
	fileBaseName := strings.TrimSuffix(fileName, ".html")
	isIndex := fileName == "index.html"
	confluenceID, _ := page.ConfluenceIDFromBasename(fileBaseName)
	space := filepath.Base(filepath.Dir(path))

	heading := getHeading(doc.Selection, isIndex)
	segments := clean.GetLocalDirSegments(doc.Selection)

	var indexSpaceKey string
	if isIndex {
		indexSpaceKey = clean.GetIndexSpace(clean.GetRightContent(doc.Selection, fileName))
	}

	contentHTML, err := clean.Pipeline(doc.Selection, fileName, heading)
	if err != nil {
		return page.Page{}, false, "", err
	}

	return page.Page{
		ConfluenceID: confluenceID,
		Title:        heading,
		Space:        space,
		IsIndex:      isIndex,
		Ancestors:    segments,
		HTML:         contentHTML,
	}, isIndex, indexSpaceKey, nil
}

// getHeading resolves a page's title, stripping the
// "<space name> : " prefix Confluence prepends to non-index page <title>
// tags, matching legacy-cs Page.getHeading.
func getHeading(doc *goquery.Selection, isIndex bool) string {
	title := doc.Find("title").Text()
	if isIndex {
		return title
	}
	indexName := strings.TrimSpace(doc.Find("#breadcrumbs .first").Text())
	return strings.Replace(title, indexName+" : ", "", 1)
}
