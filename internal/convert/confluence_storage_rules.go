package convert

import (
	"bytes"
	"strings"

	"github.com/JohannesKaufmann/dom"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"golang.org/x/net/html"
)

// RegisterConfluenceStorageRules registers renderers for Confluence
// Storage Format's "ac:"/"ri:" namespaced elements. Storage format has no
// Go equivalent library (this replaces turndown-plugin-confluence-to-gfm),
// so these are hand-written from samples of real storage-format output
// captured via the Atlassian MCP tools in development (see confluence-md-go
// README/plan for provenance) - unlike the export-source cleaner, this is
// grounded in confirmed real markup, not guesses.
func RegisterConfluenceStorageRules(conv *converter.Converter) {
	// Must run before the base plugin's own PriorityEarly pre-renderer,
	// which unconditionally removes every "#comment" node from the tree -
	// including our CDATA-disguised-as-comment nodes (see unwrapCDATA).
	conv.Register.PreRenderer(unwrapCDATA, converter.PriorityEarly-1)

	conv.Register.RendererFor("ac:structured-macro", converter.TagTypeInline, renderStructuredMacro, converter.PriorityStandard)
	conv.Register.RendererFor("ac:adf-extension", converter.TagTypeBlock, renderAdfExtension, converter.PriorityStandard)
	conv.Register.RendererFor("ac:adf-fallback", converter.TagTypeRemove, renderRemove, converter.PriorityStandard)
	conv.Register.RendererFor("ac:task-list", converter.TagTypeBlock, renderTaskList, converter.PriorityStandard)
	conv.Register.RendererFor("ac:link", converter.TagTypeInline, renderConfluenceLink, converter.PriorityStandard)
	conv.Register.RendererFor("ac:image", converter.TagTypeInline, renderConfluenceImage, converter.PriorityStandard)
	conv.Register.RendererFor("time", converter.TagTypeInline, renderTime, converter.PriorityStandard)

	// ac:layout / ac:layout-section / ac:layout-cell have no bespoke
	// renderer: Markdown has no multi-column layout, so the generic
	// fallback (render children, block-separated) is the reasonable
	// degradation - each cell's content becomes sequential blocks.
	conv.Register.TagType("ac:layout", converter.TagTypeBlock, converter.PriorityStandard)
	conv.Register.TagType("ac:layout-section", converter.TagTypeBlock, converter.PriorityStandard)
	conv.Register.TagType("ac:layout-cell", converter.TagTypeBlock, converter.PriorityStandard)
}

func renderRemove(_ converter.Context, _ converter.Writer, _ *html.Node) converter.RenderStatus {
	return converter.RenderSuccess
}

// -- helpers -----------------------------------------------------------

// directChild returns the first direct child element with the given tag
// name, or nil.
func directChild(n *html.Node, tagName string) *html.Node {
	for _, c := range dom.AllChildElements(n) {
		if dom.NodeName(c) == tagName {
			return c
		}
	}
	return nil
}

// macroParam returns the text of a "<ac:parameter ac:name=\"...\">" direct
// child matching name, matching the ac:structured-macro parameter
// convention (e.g. code language, panel/expand titles).
func macroParam(macro *html.Node, name string) string {
	for _, c := range dom.AllChildElements(macro) {
		if dom.NodeName(c) == "ac:parameter" && dom.GetAttributeOr(c, "ac:name", "") == name {
			return dom.CollectText(c)
		}
	}
	return ""
}

// adfAttribute returns the text of an "<ac:adf-attribute key=\"...\">"
// direct child matching key, matching the ac:adf-extension/ac:adf-node
// attribute convention (e.g. decision state, panel type).
func adfAttribute(n *html.Node, key string) string {
	for _, c := range dom.AllChildElements(n) {
		if dom.NodeName(c) == "ac:adf-attribute" && dom.GetAttributeOr(c, "key", "") == key {
			return dom.CollectText(c)
		}
	}
	return ""
}

// plainTextBody extracts the code content of an "<ac:plain-text-body>"
// element. By render time unwrapCDATA has already turned its CDATA section
// into a real text node, so a plain CollectText call is enough.
func plainTextBody(n *html.Node) string {
	body := directChild(n, "ac:plain-text-body")
	if body == nil {
		return ""
	}
	return dom.CollectText(body)
}

// unwrapCDATA turns "<ac:plain-text-body><![CDATA[...]]></ac:plain-text-body>"
// CDATA sections into real text nodes. Confluence wraps code-macro bodies
// this way, but golang.org/x/net/html parses "<![CDATA[...]]>" as a
// CommentNode (CDATA is only real markup in foreign/SVG content per the
// HTML5 spec) - registered as a converter.PreRenderer so it runs before the
// base plugin's pre-renderer strips every "#comment" node from the tree.
func unwrapCDATA(_ converter.Context, doc *html.Node) {
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if dom.NodeName(n) == "ac:plain-text-body" {
			if c := n.FirstChild; c != nil && c.Type == html.CommentNode &&
				strings.HasPrefix(c.Data, "[CDATA[") && strings.HasSuffix(c.Data, "]]") {
				text := c.Data[len("[CDATA[") : len(c.Data)-len("]]")]
				dom.ReplaceNode(c, &html.Node{Type: html.TextNode, Data: text})
			}
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
}

// renderChildrenTrimmed renders n's children to Markdown and trims
// surrounding whitespace, for embedding inside another block (panel body,
// decision text, etc).
func renderChildrenTrimmed(ctx converter.Context, n *html.Node) string {
	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	return strings.TrimSpace(buf.String())
}

// prefixLines prefixes every line of s with prefix, matching the
// blockquote convention used for admonition-style panels.
func prefixLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = strings.TrimRight(prefix, " ")
		} else {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// -- ac:structured-macro -------------------------------------------------

// panelLabels maps legacy (pre-ADF) Confluence panel macro names to a
// display label. Confirmed from real storage-format output: "success",
// "warning", "error" panels round-trip as ac:name "tip", "note", "warning"
// respectively (Confluence's older macro-key naming doesn't match its
// current panel color names).
var panelLabels = map[string]string{
	"info":    "Info",
	"tip":     "Success",
	"note":    "Warning",
	"warning": "Error",
}

func renderStructuredMacro(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	name := dom.GetAttributeOr(n, "ac:name", "")

	switch name {
	case "info", "tip", "note", "warning":
		body := directChild(n, "ac:rich-text-body")
		if body == nil {
			return converter.RenderSuccess
		}
		content := renderChildrenTrimmed(ctx, body)
		writeAdmonition(w, panelLabels[name], content)
		return converter.RenderSuccess

	case "code":
		lang := macroParam(n, "language")
		code := plainTextBody(n)
		w.WriteString("\n\n```" + lang + "\n" + code + "\n```\n\n")
		return converter.RenderSuccess

	case "status":
		title := macroParam(n, "title")
		w.WriteString("**[" + title + "]**")
		return converter.RenderSuccess

	case "expand":
		title := macroParam(n, "title")
		body := directChild(n, "ac:rich-text-body")
		content := ""
		if body != nil {
			content = renderChildrenTrimmed(ctx, body)
		}
		// Markdown has no native collapsible section; <details> is the
		// standard raw-HTML-passthrough convention GitHub-flavored
		// renderers support.
		w.WriteString("\n\n<details>\n<summary>" + title + "</summary>\n\n" + content + "\n\n</details>\n\n")
		return converter.RenderSuccess

	default:
		// Unknown macro: best effort, render its rich-text body (if
		// any) as a plain block rather than leaking raw ac:parameter text
		// via the generic fallback renderer.
		if body := directChild(n, "ac:rich-text-body"); body != nil {
			content := renderChildrenTrimmed(ctx, body)
			w.WriteString("\n\n" + content + "\n\n")
		}
		return converter.RenderSuccess
	}
}

func writeAdmonition(w converter.Writer, label, body string) {
	w.WriteString("\n\n> **" + label + "**\n>\n")
	w.WriteString(prefixLines(body, "> "))
	w.WriteString("\n\n")
}

// -- ac:adf-extension (decision lists, ADF-only panel colors) -----------

func renderAdfExtension(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	node := directChild(n, "ac:adf-node")
	if node == nil {
		return converter.RenderSuccess
	}

	switch dom.GetAttributeOr(node, "type", "") {
	case "decision-list":
		renderDecisionList(ctx, w, node)
	case "panel":
		renderAdfPanel(ctx, w, node)
	}
	return converter.RenderSuccess
}

func renderDecisionList(ctx converter.Context, w converter.Writer, list *html.Node) {
	w.WriteString("\n\n")
	for _, item := range dom.AllChildElements(list) {
		if dom.NodeName(item) != "ac:adf-node" || dom.GetAttributeOr(item, "type", "") != "decision-item" {
			continue
		}
		label := "Undecided"
		if adfAttribute(item, "state") == "DECIDED" {
			label = "Decided"
		}
		content := directChild(item, "ac:adf-content")
		text := ""
		if content != nil {
			text = renderChildrenTrimmed(ctx, content)
		}
		w.WriteString("- **" + label + ":** " + text + "\n")
	}
	w.WriteString("\n")
}

// renderAdfPanel handles ADF-only panel colors (currently just "note",
// Confluence's purple panel) that have no legacy ac:structured-macro
// equivalent to round-trip through.
func renderAdfPanel(ctx converter.Context, w converter.Writer, node *html.Node) {
	label := "Note"
	if panelType := adfAttribute(node, "panel-type"); panelType != "" {
		label = strings.ToUpper(panelType[:1]) + panelType[1:]
	}
	content := directChild(node, "ac:adf-content")
	text := ""
	if content != nil {
		text = renderChildrenTrimmed(ctx, content)
	}
	writeAdmonition(w, label, text)
}

// -- ac:task-list ---------------------------------------------------------

func renderTaskList(_ converter.Context, w converter.Writer, list *html.Node) converter.RenderStatus {
	w.WriteString("\n\n")
	for _, task := range dom.AllChildElements(list) {
		if dom.NodeName(task) != "ac:task" {
			continue
		}
		status := directChild(task, "ac:task-status")
		checked := " "
		if status != nil && strings.TrimSpace(dom.CollectText(status)) == "complete" {
			checked = "x"
		}
		body := directChild(task, "ac:task-body")
		text := ""
		if body != nil {
			text = strings.TrimSpace(dom.CollectText(body))
		}
		w.WriteString("- [" + checked + "] " + text + "\n")
	}
	w.WriteString("\n")
	return converter.RenderSuccess
}

// -- ac:link / ac:image --------------------------------------------------

// renderConfluenceLink handles internal page links, user mentions, and
// space links. Page links use a "%%CONFLUENCE_<space>_<id>%%" placeholder
// -- the same marker convention legacy-cs's update-links.sh substitutes --
// so the Go link-fix pass can resolve it to a real relative path once every
// page's output location is known; resolving it here, one page at a time,
// isn't possible.
func renderConfluenceLink(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	linkBodyNode := directChild(n, "ac:link-body")
	linkText := ""
	if linkBodyNode != nil {
		linkText = renderChildrenTrimmed(ctx, linkBodyNode)
	}

	if page := directChild(n, "ri:page"); page != nil {
		id := dom.GetAttributeOr(page, "ri:content-id", "")
		title := dom.GetAttributeOr(page, "ri:content-title", "")
		space := dom.GetAttributeOr(page, "ri:space-key", "")
		label := linkText
		if label == "" {
			label = title
		}
		if id != "" {
			w.WriteString("[" + label + "](%%CONFLUENCE_" + space + "_" + id + "%%)")
		} else {
			// No content ID to anchor a placeholder to - best effort,
			// plain text rather than a link that can never resolve.
			w.WriteString(label)
		}
		return converter.RenderSuccess
	}

	if user := directChild(n, "ri:user"); user != nil {
		accountID := dom.GetAttributeOr(user, "ri:account-id", "")
		label := linkText
		if label == "" {
			// Storage format only carries the account ID, not the
			// display name - resolving it to "@Real Name" needs an extra
			// user-lookup API call, not done at this per-node layer.
			label = "@" + accountID
		}
		w.WriteString(label)
		return converter.RenderSuccess
	}

	if space := directChild(n, "ri:space"); space != nil {
		label := linkText
		if label == "" {
			label = dom.GetAttributeOr(space, "ri:space-key", "")
		}
		w.WriteString(label)
		return converter.RenderSuccess
	}

	if linkText != "" {
		w.WriteString(linkText)
	}
	return converter.RenderSuccess
}

func renderConfluenceImage(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	alt := dom.GetAttributeOr(n, "ac:alt", "")

	if url := directChild(n, "ri:url"); url != nil {
		w.WriteString("![" + alt + "](" + dom.GetAttributeOr(url, "ri:value", "") + ")")
		return converter.RenderSuccess
	}
	if attachment := directChild(n, "ri:attachment"); attachment != nil {
		// Relative path: assumes the attachment gets downloaded alongside
		// the page's output (see internal/page.Attachment / source.AssetDir).
		w.WriteString("![" + alt + "](" + dom.GetAttributeOr(attachment, "ri:filename", "") + ")")
		return converter.RenderSuccess
	}
	return converter.RenderSuccess
}

// -- time -----------------------------------------------------------------

func renderTime(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if datetime := dom.GetAttributeOr(n, "datetime", ""); datetime != "" {
		w.WriteString(datetime)
		return converter.RenderSuccess
	}
	w.WriteString(dom.CollectText(n))
	return converter.RenderSuccess
}
