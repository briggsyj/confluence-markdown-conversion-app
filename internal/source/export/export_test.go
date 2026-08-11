package export

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/briggsyj/confluence-to-markdown/internal/page"
)

const indexHTML = `<html><head><title>My Space</title></head><body>
<div id="content">
<table><tr><th>Key</th><td>MYSPACE</td></tr></table>
</div>
</body></html>`

const childHTML = `<html><head><title>My Space : Child Page</title></head><body>
<div id="breadcrumbs"><a class="first">My Space</a><a>Confluence</a></div>
<div id="content"><p>Body text</p></div>
</body></html>`

// Note: legacy-cs's breadcrumb slicing (drop first two) assumes breadcrumbs
// are ordered [Confluence root, space name, ...ancestors], with the current
// page itself not among the breadcrumb links. ".first" marks the space-name
// breadcrumb specifically (used to strip the "<space> : " title prefix).
const grandchildHTML = `<html><head><title>My Space : Grandchild</title></head><body>
<div id="breadcrumbs"><a>Confluence</a><a class="first">My Space</a><a>Child Page</a></div>
<div id="content"><p>Grandchild body</p></div>
</body></html>`

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	spaceDir := filepath.Join(root, "MYSPACE")
	writeFile(t, filepath.Join(spaceDir, "index.html"), indexHTML)
	writeFile(t, filepath.Join(spaceDir, "Child Page_123456789.html"), childHTML)
	writeFile(t, filepath.Join(spaceDir, "Grandchild_987654321.html"), grandchildHTML)
	// Should be excluded from page listing.
	writeFile(t, filepath.Join(spaceDir, "attachments", "123456789", "1", "pic.png"), "not html")
	writeFile(t, filepath.Join(spaceDir, "images", "icon.png"), "not html")

	result, err := New(root).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(result.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d: %+v", len(result.Pages), result.Pages)
	}
	if result.RootSpace != "MYSPACE" {
		t.Errorf("RootSpace = %q, want MYSPACE", result.RootSpace)
	}
	if len(result.AssetDirs) != 1 {
		t.Fatalf("expected 1 asset dir (from index.html), got %d", len(result.AssetDirs))
	}
	if result.AssetDirs[0].SourcePath != spaceDir {
		t.Errorf("AssetDirs[0].SourcePath = %q, want %q", result.AssetDirs[0].SourcePath, spaceDir)
	}

	byTitle := map[string]int{}
	for i, pg := range result.Pages {
		byTitle[pg.Title] = i
	}

	childIdx, ok := byTitle["Child Page"]
	if !ok {
		t.Fatalf("no page titled %q among %v", "Child Page", titles(result.Pages))
	}
	child := result.Pages[childIdx]
	if child.ConfluenceID != "123456789" {
		t.Errorf("child ConfluenceID = %q, want 123456789", child.ConfluenceID)
	}
	if child.Space != "MYSPACE" {
		t.Errorf("child Space = %q, want MYSPACE", child.Space)
	}
	if child.IsIndex {
		t.Errorf("child page should not be marked as index")
	}

	grandchildIdx, ok := byTitle["Grandchild"]
	if !ok {
		t.Fatalf("no page titled %q among %v", "Grandchild", titles(result.Pages))
	}
	grandchild := result.Pages[grandchildIdx]
	wantAncestors := []string{"Child Page"}
	if len(grandchild.Ancestors) != len(wantAncestors) || grandchild.Ancestors[0] != wantAncestors[0] {
		t.Errorf("grandchild Ancestors = %v, want %v", grandchild.Ancestors, wantAncestors)
	}
}

func titles(pages []page.Page) []string {
	var out []string
	for _, pg := range pages {
		out = append(out, pg.Title)
	}
	return out
}

func TestListHTMLFilesExcludesAttachments(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "index.html"), indexHTML)
	writeFile(t, filepath.Join(root, "attachments", "1", "2", "notes.html"), "<html></html>")

	paths, err := listHTMLFiles(root)
	if err != nil {
		t.Fatalf("listHTMLFiles: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 html file, got %d: %v", len(paths), paths)
	}
}
