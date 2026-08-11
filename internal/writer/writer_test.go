package writer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/briggsyj/confluence-to-markdown/internal/convert"
	"github.com/briggsyj/confluence-to-markdown/internal/page"
	"github.com/briggsyj/confluence-to-markdown/internal/source"
)

func TestWrite_Page(t *testing.T) {
	outDir := t.TempDir()
	result := source.Result{
		Pages: []page.Page{
			{
				ConfluenceID: "123",
				Title:        "Child Page",
				Space:        "MYSPACE",
				Ancestors:    []string{"Parent"},
				HTML:         "<p>Hello <strong>world</strong></p>",
			},
		},
	}

	if err := Write(result, outDir, convert.New()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	outPath := filepath.Join(outDir, "MYSPACE", "Parent", "Child Page.md")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected file at %s: %v", outPath, err)
	}
	content := string(data)

	if !strings.HasPrefix(content, "---\nconfluence-id: 123\nconfluence-space: MYSPACE\n---\n") {
		t.Errorf("unexpected frontmatter, got:\n%s", content)
	}
	if !strings.Contains(content, "Hello **world**") {
		t.Errorf("expected converted markdown body, got:\n%s", content)
	}
}

func TestWrite_IndexPage(t *testing.T) {
	outDir := t.TempDir()
	result := source.Result{
		Pages: []page.Page{
			{ConfluenceID: "", Title: "Home", Space: "MYSPACE", IsIndex: true, HTML: "<p>Welcome</p>"},
		},
	}

	if err := Write(result, outDir, convert.New()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "MYSPACE", "index.md")); err != nil {
		t.Errorf("expected index.md at space root: %v", err)
	}
}

func TestWrite_PageAttachment(t *testing.T) {
	outDir := t.TempDir()
	attSrc := filepath.Join(t.TempDir(), "downloaded.png")
	if err := os.WriteFile(attSrc, []byte("fake-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := source.Result{
		Pages: []page.Page{
			{
				ConfluenceID: "1",
				Title:        "Page With Image",
				Space:        "MYSPACE",
				HTML:         "<p>See image</p>",
				Attachments:  []page.Attachment{{FileName: "pic.png", LocalPath: attSrc}},
			},
		},
	}

	if err := Write(result, outDir, convert.New()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "MYSPACE", "pic.png"))
	if err != nil {
		t.Fatalf("expected attachment copied next to page: %v", err)
	}
	if string(data) != "fake-bytes" {
		t.Errorf("got %q", data)
	}
}

func TestCopyAssetDir(t *testing.T) {
	outDir := t.TempDir()
	sourceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceRoot, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "images", "icon.png"), []byte("icon-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	ad := source.AssetDir{Space: "MYSPACE", LocalDir: "", SourcePath: sourceRoot}
	if err := copyAssetDir(ad, outDir); err != nil {
		t.Fatalf("copyAssetDir: %v", err)
	}

	// LocalDir == "" reproduces legacy-cs's dirname() quirk: assets land
	// in outDir directly, not outDir/MYSPACE.
	if _, err := os.Stat(filepath.Join(outDir, "images", "icon.png")); err != nil {
		t.Errorf("expected assets at outDir root (legacy dirname quirk): %v", err)
	}
}
