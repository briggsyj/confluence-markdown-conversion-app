package convert

import (
	"strings"
	"testing"
)

// All HTML fixtures in this file are lifted verbatim (or near-verbatim,
// trimming ac:macro-id/ac:schema-version noise) from real Confluence
// Storage Format captured via the Atlassian MCP tools against a live
// Confluence Cloud site while building this project's test space - not
// hand-guessed markup.

func convertOrFail(t *testing.T, htmlInput string) string {
	t.Helper()
	out, err := New().Convert(htmlInput)
	if err != nil {
		t.Fatalf("Convert error: %v\ninput: %s", err, htmlInput)
	}
	return out
}

func TestInfoPanel(t *testing.T) {
	out := convertOrFail(t, `<ac:structured-macro ac:name="info"><ac:rich-text-body><p>Info panel: general information callout.</p></ac:rich-text-body></ac:structured-macro>`)
	if !strings.Contains(out, "> **Info**") || !strings.Contains(out, "> Info panel: general information callout.") {
		t.Errorf("got:\n%s", out)
	}
}

func TestLegacyPanelColorMapping(t *testing.T) {
	cases := []struct {
		macroName string
		wantLabel string
	}{
		{"tip", "Success"},   // panel-success round-trips as ac:name="tip"
		{"note", "Warning"},  // panel-warning round-trips as ac:name="note"
		{"warning", "Error"}, // panel-error round-trips as ac:name="warning"
	}
	for _, tc := range cases {
		html := `<ac:structured-macro ac:name="` + tc.macroName + `"><ac:rich-text-body><p>body text</p></ac:rich-text-body></ac:structured-macro>`
		out := convertOrFail(t, html)
		if !strings.Contains(out, "**"+tc.wantLabel+"**") {
			t.Errorf("macro %q: got:\n%s", tc.macroName, out)
		}
	}
}

func TestAdfNotePanel(t *testing.T) {
	out := convertOrFail(t, `<ac:adf-extension><ac:adf-node type="panel"><ac:adf-attribute key="panel-type">note</ac:adf-attribute><ac:adf-content><p>Note panel: a side note worth remembering.</p></ac:adf-content></ac:adf-node><ac:adf-fallback><div class="panel"><p>Note panel: a side note worth remembering.</p></div></ac:adf-fallback></ac:adf-extension>`)
	if !strings.Contains(out, "> **Note**") {
		t.Errorf("got:\n%s", out)
	}
	if strings.Count(out, "Note panel: a side note") != 1 {
		t.Errorf("expected fallback content NOT duplicated, got:\n%s", out)
	}
}

func TestCodeMacro(t *testing.T) {
	out := convertOrFail(t, `<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">js</ac:parameter><ac:plain-text-body><![CDATA[function greet(name) {
  return `+"`"+`Hello, ${name}!`+"`"+`;
}]]></ac:plain-text-body></ac:structured-macro>`)
	if !strings.Contains(out, "```js") {
		t.Errorf("expected fenced js code block, got:\n%s", out)
	}
	if !strings.Contains(out, "function greet(name)") {
		t.Errorf("expected code body preserved, got:\n%s", out)
	}
}

func TestStatusMacro(t *testing.T) {
	out := convertOrFail(t, `<p>Status lozenges: <ac:structured-macro ac:name="status"><ac:parameter ac:name="title">Done</ac:parameter><ac:parameter ac:name="colour">Green</ac:parameter></ac:structured-macro></p>`)
	if !strings.Contains(out, "**[Done]**") {
		t.Errorf("got:\n%s", out)
	}
}

func TestExpandMacro(t *testing.T) {
	out := convertOrFail(t, `<ac:structured-macro ac:name="expand"><ac:parameter ac:name="title">Click to expand basic details</ac:parameter><ac:rich-text-body><p>Hidden content revealed when expanded.</p></ac:rich-text-body></ac:structured-macro>`)
	if !strings.Contains(out, "<summary>Click to expand basic details</summary>") {
		t.Errorf("got:\n%s", out)
	}
	if !strings.Contains(out, "Hidden content revealed when expanded.") {
		t.Errorf("got:\n%s", out)
	}
}

func TestTaskList(t *testing.T) {
	out := convertOrFail(t, `<ac:task-list ac:task-list-id="6f618c069e02">
<ac:task>
<ac:task-id>1</ac:task-id>
<ac:task-uuid>027ec3b138da</ac:task-uuid>
<ac:task-status>complete</ac:task-status>
<ac:task-body><span class="placeholder-inline-tasks">Write kitchen-sink pages</span></ac:task-body>
</ac:task>
<ac:task>
<ac:task-id>2</ac:task-id>
<ac:task-uuid>abe7da189bfe</ac:task-uuid>
<ac:task-status>incomplete</ac:task-status>
<ac:task-body><span class="placeholder-inline-tasks">Export the space to HTML</span></ac:task-body>
</ac:task>
</ac:task-list>`)
	if !strings.Contains(out, "- [x] Write kitchen-sink pages") {
		t.Errorf("got:\n%s", out)
	}
	if !strings.Contains(out, "- [ ] Export the space to HTML") {
		t.Errorf("got:\n%s", out)
	}
}

func TestDecisionList(t *testing.T) {
	out := convertOrFail(t, `<ac:adf-extension><ac:adf-node type="decision-list"><ac:adf-attribute key="local-id">fbfd9634bf96</ac:adf-attribute><ac:adf-node type="decision-item"><ac:adf-attribute key="local-id">3b5d2ad69f81</ac:adf-attribute><ac:adf-attribute key="state">DECIDED</ac:adf-attribute><ac:adf-content>Use "My first space" as the kitchen-sink test space</ac:adf-content></ac:adf-node><ac:adf-node type="decision-item"><ac:adf-attribute key="local-id">72526e11ed46</ac:adf-attribute><ac:adf-attribute key="state">UNDECIDED</ac:adf-attribute><ac:adf-content>Whether to add real image attachments manually</ac:adf-content></ac:adf-node></ac:adf-node><ac:adf-fallback><ul class="decision-list"><li>Use "My first space" as the kitchen-sink test space</li><li>Whether to add real image attachments manually</li></ul></ac:adf-fallback></ac:adf-extension>`)
	if !strings.Contains(out, "- **Decided:** Use \"My first space\" as the kitchen-sink test space") {
		t.Errorf("got:\n%s", out)
	}
	if !strings.Contains(out, "- **Undecided:** Whether to add real image attachments manually") {
		t.Errorf("got:\n%s", out)
	}
	if strings.Count(out, "kitchen-sink test space") != 1 {
		t.Errorf("expected fallback <ul> content NOT duplicated, got:\n%s", out)
	}
}

func TestLayout(t *testing.T) {
	out := convertOrFail(t, `<ac:layout><ac:layout-section ac:type="fixed-width"><ac:layout-cell><h2>Two-column layout</h2></ac:layout-cell></ac:layout-section><ac:layout-section ac:type="two_equal"><ac:layout-cell><p><strong>Left column.</strong> text</p></ac:layout-cell><ac:layout-cell><p><strong>Right column.</strong> text</p></ac:layout-cell></ac:layout-section></ac:layout>`)
	if !strings.Contains(out, "Two-column layout") || !strings.Contains(out, "Left column") || !strings.Contains(out, "Right column") {
		t.Errorf("got:\n%s", out)
	}
}

func TestInternalPageLinkWithContentID(t *testing.T) {
	// Real MCP-created links only carried ri:content-title, not
	// ri:content-id (see TestInternalPageLinkFallsBackToPlainText below)
	// - this covers the ri:content-id form Confluence's own UI is
	// documented to emit, unconfirmed against a real non-MCP sample.
	out := convertOrFail(t, `<ac:link><ri:page ri:space-key="MFS" ri:content-id="98643" ri:content-title="Text Formatting" /><ac:link-body>Text Formatting</ac:link-body></ac:link>`)
	if !strings.Contains(out, "[Text Formatting](%%CONFLUENCE_MFS_98643%%)") {
		t.Errorf("got:\n%s", out)
	}
}

func TestInternalPageLinkFallsBackToPlainText(t *testing.T) {
	// Real sample: MCP-created cross-page links only carry
	// ri:content-title, no ri:content-id - can't build a resolvable
	// placeholder, so this should degrade to plain text, not a dead link.
	out := convertOrFail(t, `<ac:link><ri:page ri:space-key="MFS" ri:content-title="Kitchen Sink - Text Formatting" /><ac:link-body>Text Formatting</ac:link-body></ac:link>`)
	if strings.Contains(out, "%%CONFLUENCE_") {
		t.Errorf("should not fabricate a placeholder without a content ID, got:\n%s", out)
	}
	if !strings.Contains(out, "Text Formatting") {
		t.Errorf("got:\n%s", out)
	}
}

func TestMention(t *testing.T) {
	out := convertOrFail(t, `<p>Assigned to <ac:link><ri:user ri:account-id="5a79622a5a1ae359c7041f39" /></ac:link>.</p>`)
	if !strings.Contains(out, "@5a79622a5a1ae359c7041f39") {
		t.Errorf("got:\n%s", out)
	}
}

func TestExternalImage(t *testing.T) {
	out := convertOrFail(t, `<ac:image ac:align="center" ac:layout="center" ac:alt="placeholder"><ri:url ri:value="https://upload.wikimedia.org/wikipedia/commons/6/6c/PICA_-_Placeholder_image.svg" /></ac:image>`)
	if !strings.Contains(out, "![placeholder](https://upload.wikimedia.org/wikipedia/commons/6/6c/PICA_-_Placeholder_image.svg)") {
		t.Errorf("got:\n%s", out)
	}
}

func TestAttachmentImage(t *testing.T) {
	out := convertOrFail(t, `<ac:image ac:alt="diagram"><ri:attachment ri:filename="diagram.png" /></ac:image>`)
	if !strings.Contains(out, "![diagram](diagram.png)") {
		t.Errorf("got:\n%s", out)
	}
}

func TestTimeElement(t *testing.T) {
	out := convertOrFail(t, `<p>due <time datetime="2026-08-31">August 31, 2026</time>.</p>`)
	if !strings.Contains(out, "2026-08-31") {
		t.Errorf("got:\n%s", out)
	}
}
