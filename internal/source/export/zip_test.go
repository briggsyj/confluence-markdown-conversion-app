package export

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveInput_Directory(t *testing.T) {
	dir := t.TempDir()
	resolved, cleanup, err := ResolveInput(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if resolved != dir {
		t.Errorf("expected directory input returned unchanged, got %q", resolved)
	}
}

func TestResolveInput_Zip(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "export.zip")
	writeZip(t, zipPath, map[string]string{
		"MFS/index.html":            indexHTML,
		"MFS/Child Page_123.html":   childHTML,
		"MFS/attachments/1/pic.png": "fake-bytes",
	})

	resolved, cleanup, err := ResolveInput(zipPath)
	if err != nil {
		t.Fatalf("ResolveInput: %v", err)
	}
	defer cleanup()

	// The zip wraps content in a top-level "MFS/" folder (matching real
	// Confluence exports, keyed to the space key) - it should still be
	// there after extraction, since Load()'s space-name detection relies
	// on each page's immediate parent directory name.
	if filepath.Base(resolved) == "MFS" {
		t.Fatalf("expected resolved dir to be the temp extraction root, not MFS itself: %q", resolved)
	}
	if _, err := os.Stat(filepath.Join(resolved, "MFS", "index.html")); err != nil {
		t.Fatalf("expected MFS/index.html under extracted dir: %v", err)
	}

	result, err := New(resolved).Load(context.Background())
	if err != nil {
		t.Fatalf("Load after zip extraction: %v", err)
	}
	if len(result.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(result.Pages))
	}
	for _, pg := range result.Pages {
		if pg.Space != "MFS" {
			t.Errorf("expected space MFS (from the zip's wrapping folder), got %q", pg.Space)
		}
	}
}

func TestResolveInput_ZipCleanupRemovesTempDir(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "export.zip")
	writeZip(t, zipPath, map[string]string{"MFS/index.html": indexHTML})

	resolved, cleanup, err := ResolveInput(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := os.Stat(resolved); !os.IsNotExist(err) {
		t.Errorf("expected temp extraction dir removed after cleanup")
	}
}

func TestExtractZip_RejectsZipSlip(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "malicious.zip")
	writeZip(t, zipPath, map[string]string{
		"../../etc/evil.html": "<html></html>",
	})

	_, _, err := extractZip(zipPath)
	if err == nil {
		t.Fatal("expected an error for a zip entry that escapes the destination directory")
	}
	if !strings.Contains(err.Error(), "illegal file path") {
		t.Errorf("expected a zip-slip error, got: %v", err)
	}
}
