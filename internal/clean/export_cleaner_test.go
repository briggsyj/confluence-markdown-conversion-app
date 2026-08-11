package clean

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func parse(t *testing.T, htmlStr string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

func TestFixIcon(t *testing.T) {
	doc := parse(t, `<div id="content"><p><span class="aui-icon aui-icon-small">note</span>Title</p></div>`)
	FixIcon(doc.Find("#content"))
	text := doc.Find("#content").Text()
	if text != "noteTitle" {
		t.Errorf("got %q, want %q", text, "noteTitle")
	}
	if doc.Find("span.aui-icon").Length() != 0 {
		t.Errorf("expected icon span to be removed")
	}
}

func TestFixEmptyLink(t *testing.T) {
	doc := parse(t, `<div id="content"><a href="#"></a><a href="#">Keep</a><a href="#"><img src="x.png"></a></div>`)
	FixEmptyLink(doc.Find("#content"))
	links := doc.Find("#content a")
	if links.Length() != 2 {
		t.Fatalf("expected 2 links to remain, got %d", links.Length())
	}
}

func TestFixEmptyHeading(t *testing.T) {
	doc := parse(t, `<div id="content"><h2></h2><h3>Keep</h3></div>`)
	FixEmptyHeading(doc.Find("#content"))
	if doc.Find("#content h2").Length() != 0 {
		t.Errorf("expected empty heading removed")
	}
	if doc.Find("#content h3").Length() != 1 {
		t.Errorf("expected non-empty heading kept")
	}
}

func TestFixPreformattedText(t *testing.T) {
	doc := parse(t, `<div id="content"><pre class="syntaxhighlighter-pre" data-syntaxhighlighter-params="brush: python; gutter: false">print(1)</pre></div>`)
	FixPreformattedText(doc.Find("#content"))
	pre := doc.Find("#content pre")
	class, _ := pre.Attr("class")
	if class != "language-python" {
		t.Errorf("got class %q, want %q", class, "language-python")
	}
}

func TestFixImageWithinSpan(t *testing.T) {
	doc := parse(t, `<div id="content"><span><img src="x.png"></span></div>`)
	FixImageWithinSpan(doc.Find("#content"))
	if doc.Find("#content span").Length() != 0 {
		t.Errorf("expected wrapping span to be unwrapped")
	}
	if doc.Find("#content img").Length() != 1 {
		t.Errorf("expected image to survive unwrap")
	}
}

func TestRemoveArbitraryElements(t *testing.T) {
	doc := parse(t, `<div id="content"><p>Assigned to <span class="user-mention">Jane</span></p></div>`)
	RemoveArbitraryElements(doc.Find("#content"))
	if doc.Find("#content span").Length() != 0 {
		t.Errorf("expected span removed")
	}
	if got := doc.Find("#content").Text(); got != "Assigned to Jane" {
		t.Errorf("got %q, want %q", got, "Assigned to Jane")
	}
}

func TestFixArbitraryClasses(t *testing.T) {
	doc := parse(t, `<div id="content"><p class="confluence-embedded-file odd keep-me">x</p></div>`)
	FixArbitraryClasses(doc.Find("#content"))
	class, _ := doc.Find("#content p").Attr("class")
	if class != "keep-me" {
		t.Errorf("got class %q, want %q", class, "keep-me")
	}
}

func TestFixAttachmentWrapper(t *testing.T) {
	doc := parse(t, `<div id="content"><div class="attachment-buttons">x</div><table class="attachments aui"><tr><td>a</td></tr></table><p>Keep</p></div>`)
	FixAttachmentWrapper(doc.Find("#content"))
	if doc.Find("#content .attachment-buttons").Length() != 0 {
		t.Errorf("expected attachment-buttons removed")
	}
	if doc.Find("#content table.attachments").Length() != 0 {
		t.Errorf("expected attachments table removed")
	}
	if doc.Find("#content p").Length() != 1 {
		t.Errorf("expected unrelated content kept")
	}
}

func TestFixPageLog(t *testing.T) {
	doc := parse(t, `<div id="content"><div class="panel"><h4 id="SpaceHome-Recentspaceactivity">Recent activity</h4></div><p>Keep</p></div>`)
	FixPageLog(doc.Find("#content"))
	if doc.Find("#content .panel").Length() != 0 {
		t.Errorf("expected recent-activity panel removed")
	}
	if doc.Find("#content p").Length() != 1 {
		t.Errorf("expected unrelated content kept")
	}
}

func TestAddPageHeading(t *testing.T) {
	doc := parse(t, `<div id="content"><p>Body</p></div>`)
	AddPageHeading(doc.Find("#content"), "My Title")
	h1 := doc.Find("#content h1").First()
	if h1.Text() != "My Title" {
		t.Errorf("got %q, want %q", h1.Text(), "My Title")
	}
}

func TestGetLocalDirSegments(t *testing.T) {
	doc := parse(t, `<div id="breadcrumbs"><a>Confluence</a><a>My Space</a><a>Parent Page</a><a>Child Page</a></div>`)
	segments := GetLocalDirSegments(doc.Selection)
	want := []string{"Parent Page", "Child Page"}
	if len(segments) != len(want) {
		t.Fatalf("got %v, want %v", segments, want)
	}
	for i := range want {
		if segments[i] != want[i] {
			t.Errorf("segment %d: got %q, want %q", i, segments[i], want[i])
		}
	}
}

func TestPipeline(t *testing.T) {
	doc := parse(t, `<html><body><div id="content"><h2></h2><span class="aui-icon">i</span><p>Body <span class="user-mention">Jane</span></p></div></body></html>`)
	out, err := Pipeline(doc.Selection, "MyPage_123.html", "My Page")
	if err != nil {
		t.Fatalf("pipeline error: %v", err)
	}
	if !strings.Contains(out, "<h1>My Page</h1>") {
		t.Errorf("expected injected h1 heading, got: %s", out)
	}
	if strings.Contains(out, "<h2>") {
		t.Errorf("expected empty h2 removed, got: %s", out)
	}
	if strings.Contains(out, "aui-icon") {
		t.Errorf("expected icon span stripped, got: %s", out)
	}
	if !strings.Contains(out, "Jane") {
		t.Errorf("expected mention text preserved, got: %s", out)
	}
}
