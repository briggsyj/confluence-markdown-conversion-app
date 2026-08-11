package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/briggsyj/confluence-to-markdown/internal/convert"
	"github.com/briggsyj/confluence-to-markdown/internal/linkfix"
	exportsource "github.com/briggsyj/confluence-to-markdown/internal/source/export"
	"github.com/briggsyj/confluence-to-markdown/internal/writer"
)

// Real Confluence Cloud export of the "Kitchen Sink" test space (see
// this project's plan doc / project memory), provided both as the raw
// zip Confluence's "Export Space" produces and pre-extracted, so both
// input modes get exercised against real data, not just synthetic
// fixtures.
const (
	testdataZip      = "../../testdata/kitchen-sink-export/export.zip"
	testdataUnzipped = "../../testdata/kitchen-sink-export/unzipped"
)

func convertExport(t *testing.T, input string) string {
	t.Helper()
	outDir := t.TempDir()

	resolved, cleanup, err := exportsource.ResolveInput(input)
	if err != nil {
		t.Fatalf("ResolveInput(%q): %v", input, err)
	}
	defer cleanup()

	result, err := exportsource.New(resolved).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(result.Pages) != 10 {
		t.Fatalf("expected 10 pages in the Kitchen Sink export, got %d", len(result.Pages))
	}

	if err := writer.Write(result, outDir, convert.New()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := linkfix.Run(outDir, result.RootSpace, "https://briggsyj.atlassian.net/wiki/spaces/", linkfix.Options{ArticleDir: true}); err != nil {
		t.Fatalf("linkfix.Run: %v", err)
	}
	return outDir
}

func TestRealExport_Zip(t *testing.T) {
	requireTestdata(t, testdataZip)
	assertKitchenSinkOutput(t, convertExport(t, testdataZip))
}

func TestRealExport_Unzipped(t *testing.T) {
	requireTestdata(t, testdataUnzipped)
	assertKitchenSinkOutput(t, convertExport(t, testdataUnzipped))
}

func requireTestdata(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("test fixture not available at %s: %v", path, err)
	}
}

func assertKitchenSinkOutput(t *testing.T, outDir string) {
	t.Helper()

	linksMedia := findFile(t, outDir, "Links Media.md")
	content := string(readFile(t, linksMedia))

	// Regression guard: legacy-cs's malformed-local-link regex hardcoded a
	// 10-digit page ID and never matched this space's real (shorter) IDs,
	// leaving internal links as dead ".html" references. Confirm they
	// resolve to real relative Markdown paths instead.
	if !strings.Contains(content, "Text Formatting.md") {
		t.Errorf("expected internal link to Text Formatting resolved to a real path, got:\n%s", content)
	}
	if strings.Contains(content, ".html)") {
		t.Errorf("expected no dangling .html links left in output, got:\n%s", content)
	}

	tables := string(readFile(t, findFile(t, outDir, "Tables.md")))
	if !strings.Contains(tables, "| Ada Lovelace | Mathematician") {
		t.Errorf("expected Tables page's basic table converted correctly, got:\n%s", tables)
	}
}

func findFile(t *testing.T, root, name string) string {
	t.Helper()
	var found string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == name {
			found = path
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == "" {
		t.Fatalf("no file named %q found under %s", name, root)
	}
	return found
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
