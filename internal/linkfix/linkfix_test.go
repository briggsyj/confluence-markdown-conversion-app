package linkfix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// frontMatter builds a realistic frontmatter block: each "<div>" line
// App.coffee's writeMarkdownFile emits becomes its own line once turndown
// (a block-level element) has rendered it - NOT all crammed onto one line.
func frontMatter(id string) string {
	return "---\nconfluence-id: " + id + "\nconfluence-space: %%CONFLUENCE-SPACE%%\n---\n"
}

func TestExtractConfluenceID(t *testing.T) {
	content := frontMatter("123456789") + "\n# Heading"
	if id := extractConfluenceID(content); id != "123456789" {
		t.Errorf("got %q, want 123456789", id)
	}
}

func TestExtractConfluenceID_IgnoresMatchOutsideFrontMatter(t *testing.T) {
	// A later line that happens to contain "confluence-id:" (e.g. quoted
	// in body prose) must not be picked up once the closing "---" has
	// been seen.
	content := frontMatter("123") + "\nSee confluence-id: 999 mentioned in body text."
	if id := extractConfluenceID(content); id != "123" {
		t.Errorf("got %q, want 123 (from frontmatter, not body)", id)
	}
}

func TestMoveRootPagesIntoDirectories(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "Parent.md"), "parent content")
	if err := os.MkdirAll(filepath.Join(root, "Parent"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "Parent", "Child.md"), "child content")

	if err := moveRootPagesIntoDirectories(root); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "Parent.md")); !os.IsNotExist(err) {
		t.Errorf("expected Parent.md to be moved away")
	}
	if got := read(t, filepath.Join(root, "Parent", "Parent.md")); got != "parent content" {
		t.Errorf("got %q", got)
	}
}

func TestCreateArticleDirectories(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "My Page.md"), "content")

	if err := createArticleDirectories(root); err != nil {
		t.Fatal(err)
	}

	if got := read(t, filepath.Join(root, "My Page", "My Page.md")); got != "content" {
		t.Errorf("got %q", got)
	}
}

func TestCreateArticleDirectories_SkipsAlreadyDedicated(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "My Page", "My Page.md"), "content")

	if err := createArticleDirectories(root); err != nil {
		t.Fatal(err)
	}

	// Should not have been wrapped again into My Page/My Page/My Page.md.
	if _, err := os.Stat(filepath.Join(root, "My Page", "My Page", "My Page.md")); !os.IsNotExist(err) {
		t.Errorf("expected no double-nesting")
	}
	if got := read(t, filepath.Join(root, "My Page", "My Page.md")); got != "content" {
		t.Errorf("got %q", got)
	}
}

func TestRewriteLinks_RawConfluenceURL(t *testing.T) {
	root := t.TempDir()
	confluenceURL := "https://example.atlassian.net/wiki/spaces/"
	write(t, filepath.Join(root, "A.md"), frontMatter("111")+"\nSee [B](https://example.atlassian.net/wiki/spaces/MYSPACE/pages/222/B)")
	write(t, filepath.Join(root, "B.md"), frontMatter("222")+"\nBody")

	idIndex, err := buildConfluenceIDIndex(root, "MYSPACE")
	if err != nil {
		t.Fatal(err)
	}
	if err := rewriteLinks(root, "MYSPACE", confluenceURL, idIndex); err != nil {
		t.Fatal(err)
	}

	got := read(t, filepath.Join(root, "A.md"))
	if !strings.Contains(got, "[B](<B.md>)") {
		t.Errorf("expected resolved link to B.md, got:\n%s", got)
	}
}

func TestRewriteLinks_CrossSpaceLinkStaysUnresolved(t *testing.T) {
	// A link into a different space than this run's `space` argument
	// captures ITS OWN space from the URL (via the raw-URL rule), but the
	// ID index is only ever keyed to this run's single space - so it can
	// never resolve, by design (matches legacy-cs's single-space-per-run
	// architecture, not a bug introduced here).
	root := t.TempDir()
	confluenceURL := "https://example.atlassian.net/wiki/spaces/"
	write(t, filepath.Join(root, "A.md"), frontMatter("111")+"\nSee [Other](https://example.atlassian.net/wiki/spaces/OTHERSPACE/pages/999/Other)")

	idIndex, err := buildConfluenceIDIndex(root, "MYSPACE")
	if err != nil {
		t.Fatal(err)
	}
	if err := rewriteLinks(root, "MYSPACE", confluenceURL, idIndex); err != nil {
		t.Fatal(err)
	}

	got := read(t, filepath.Join(root, "A.md"))
	if !strings.Contains(got, "%%CONFLUENCE_OTHERSPACE_999%%") {
		t.Errorf("expected unresolved cross-space placeholder left in place, got:\n%s", got)
	}
}

func TestRewriteLinks_MalformedLocalLink(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "A.md"), frontMatter("111")+"\nSee [B](My Page_2222222222.html)")
	write(t, filepath.Join(root, "B.md"), frontMatter("2222222222")+"\nBody")

	idIndex, err := buildConfluenceIDIndex(root, "MYSPACE")
	if err != nil {
		t.Fatal(err)
	}
	if err := rewriteLinks(root, "MYSPACE", "https://example.atlassian.net/wiki/spaces/", idIndex); err != nil {
		t.Fatal(err)
	}

	got := read(t, filepath.Join(root, "A.md"))
	if !strings.Contains(got, "[B](<B.md>)") {
		t.Errorf("expected malformed .html link resolved, got:\n%s", got)
	}
}

func TestRewriteLinks_MalformedLocalLink_ShortRealWorldID(t *testing.T) {
	// Real Confluence Cloud page IDs aren't always 10 digits (legacy-cs
	// hardcoded that length and never matched this space's real 5-digit
	// IDs - see malformedLocalLinkPattern's doc comment).
	root := t.TempDir()
	write(t, filepath.Join(root, "A.md"), frontMatter("111")+"\nSee [B](Some-Page_98643.html) and [C](65706.html)")
	write(t, filepath.Join(root, "B.md"), frontMatter("98643")+"\nBody B")
	write(t, filepath.Join(root, "C.md"), frontMatter("65706")+"\nBody C")

	idIndex, err := buildConfluenceIDIndex(root, "MYSPACE")
	if err != nil {
		t.Fatal(err)
	}
	if err := rewriteLinks(root, "MYSPACE", "https://example.atlassian.net/wiki/spaces/", idIndex); err != nil {
		t.Fatal(err)
	}

	got := read(t, filepath.Join(root, "A.md"))
	if !strings.Contains(got, "[B](<B.md>)") {
		t.Errorf("expected 5-digit-ID link to B resolved, got:\n%s", got)
	}
	if !strings.Contains(got, "[C](<C.md>)") {
		t.Errorf("expected bare-ID (no title prefix) link to C resolved, got:\n%s", got)
	}
}

func TestRewriteLinks_SpacePlaceholderAndFrontMatterUnescape(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "A.md"), "<div>\\---</div><div>confluence-id: 111</div><div>confluence-space: %%CONFLUENCE-SPACE%%</div><div>\\---</div>\nBody")

	idIndex, err := buildConfluenceIDIndex(root, "MYSPACE")
	if err != nil {
		t.Fatal(err)
	}
	if err := rewriteLinks(root, "MYSPACE", "https://example.atlassian.net/wiki/spaces/", idIndex); err != nil {
		t.Fatal(err)
	}

	got := read(t, filepath.Join(root, "A.md"))
	if strings.Contains(got, "%%CONFLUENCE-SPACE%%") {
		t.Errorf("expected space placeholder resolved, got:\n%s", got)
	}
	if !strings.Contains(got, "confluence-space: MYSPACE") {
		t.Errorf("got:\n%s", got)
	}
}

func TestReorganizeAttachments(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "attachments", "123", "pic.png"), "fake-png-bytes")
	write(t, filepath.Join(root, "My Page", "My Page.md"), "See ![img](attachments/123/pic.png)")

	if err := reorganizeAttachments(root, false, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "My Page", "pic.png")); err != nil {
		t.Errorf("expected attachment copied alongside page: %v", err)
	}
	got := read(t, filepath.Join(root, "My Page", "My Page.md"))
	if !strings.Contains(got, "./pic.png") {
		t.Errorf("expected link rewritten to relative path, got:\n%s", got)
	}
}

func TestReorganizeAttachments_SkipsOversizedFile(t *testing.T) {
	root := t.TempDir()
	big := strings.Repeat("x", 10_000_001)
	write(t, filepath.Join(root, "attachments", "big.bin"), big)
	write(t, filepath.Join(root, "My Page", "My Page.md"), "See [file](attachments/big.bin)")

	if err := reorganizeAttachments(root, false, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "My Page", "big.bin")); !os.IsNotExist(err) {
		t.Errorf("expected oversized attachment NOT copied")
	}
	got := read(t, filepath.Join(root, "My Page", "My Page.md"))
	if !strings.Contains(got, "attachments/big.bin") {
		t.Errorf("expected link left untouched when attachment is skipped, got:\n%s", got)
	}
}

func TestRun_EndToEnd(t *testing.T) {
	root := t.TempDir()
	confluenceURL := "https://example.atlassian.net/wiki/spaces/"

	write(t, filepath.Join(root, "index.md"),
		frontMatter("")+"\n# Home\n\nSee [Child](https://example.atlassian.net/wiki/spaces/MYSPACE/pages/222/Child)")
	write(t, filepath.Join(root, "Child.md"),
		frontMatter("222")+"\n# Child\n\n![diagram](attachments/222/diagram.png)")
	write(t, filepath.Join(root, "attachments", "222", "diagram.png"), "fake-image-bytes")

	err := Run(root, "MYSPACE", confluenceURL, Options{ArticleDir: true}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	childMD := filepath.Join(root, "Child", "Child.md")
	got := read(t, childMD)
	if !strings.Contains(got, "./diagram.png") {
		t.Errorf("expected attachment relocated and relinked, got:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "Child", "diagram.png")); err != nil {
		t.Errorf("expected attachment copied next to relocated page: %v", err)
	}

	indexMD := filepath.Join(root, "index", "index.md")
	gotIndex := read(t, indexMD)
	// index.md lives in root/index/, Child.md in root/Child/, so the link
	// must step up out of index/ first ("../Child/Child.md") to resolve in a
	// file-relative Markdown editor.
	if !strings.Contains(gotIndex, "[Child](<../Child/Child.md>)") {
		t.Errorf("expected resolved link to relocated Child.md, got:\n%s", gotIndex)
	}
}

func TestRewriteLinks_ResolvesRelativeToReferencingFile(t *testing.T) {
	// The referencing page and its target live in different subdirectories,
	// so the resolved link must be relative to the referencing file's own
	// directory ("../Target/Target.md"), not to the space root - the latter
	// breaks in Typora and other file-relative Markdown editors.
	root := t.TempDir()
	confluenceURL := "https://example.atlassian.net/wiki/spaces/"
	write(t, filepath.Join(root, "Index", "Index.md"),
		frontMatter("111")+"\nSee [Target](https://example.atlassian.net/wiki/spaces/MYSPACE/pages/222/Target)")
	write(t, filepath.Join(root, "Target", "Target.md"), frontMatter("222")+"\nBody")

	idIndex, err := buildConfluenceIDIndex(root, "MYSPACE")
	if err != nil {
		t.Fatal(err)
	}
	if err := rewriteLinks(root, "MYSPACE", confluenceURL, idIndex); err != nil {
		t.Fatal(err)
	}

	got := read(t, filepath.Join(root, "Index", "Index.md"))
	if !strings.Contains(got, "[Target](<../Target/Target.md>)") {
		t.Errorf("expected link relative to referencing file, got:\n%s", got)
	}
}
