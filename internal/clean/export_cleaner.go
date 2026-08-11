// Package clean holds source-specific HTML cleanup passes that normalize
// raw Confluence markup into content ready for markdown conversion.
package clean

import (
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// GetRightContent selects the real content region of a parsed export page,
// matching legacy-cs Formatter.getRightContentByFileName. Both branches
// select "#content" today (the other legacy-cs selectors were already
// commented out), kept as two branches in case index pages need to diverge
// again once this is validated against a real export.
func GetRightContent(doc *goquery.Selection, fileName string) *goquery.Selection {
	if fileName == "index.html" {
		return doc.Find("#content")
	}
	return doc.Find("#content")
}

// GetLocalDirSegments returns the breadcrumb-derived ancestor titles for a
// page, with the first two breadcrumb entries dropped (legacy-cs: these are
// the fixed "Confluence" root and the space name itself, not real
// ancestors), matching legacy-cs Formatter.getLocalDir before its
// sanitize+join step (done separately by page.LocalDirFromSegments).
func GetLocalDirSegments(content *goquery.Selection) []string {
	var segments []string
	content.Find("#breadcrumbs a").Each(func(_ int, s *goquery.Selection) {
		segments = append(segments, s.Text())
	})
	if len(segments) <= 2 {
		return nil
	}
	return segments[2:]
}

// GetIndexSpace reads the space key from an index page's detail table,
// matching legacy-cs Page.getIndexSpace.
func GetIndexSpace(content *goquery.Selection) string {
	return strings.TrimSpace(content.Find(`th:contains("Key")`).Next().Text())
}

// replaceWithText replaces an element with its own text content, matching
// legacy-cs Formatter._removeElementLeaveText. The text is HTML-escaped
// before reinsertion so literal "<"/"&" characters in the original text
// can't be misread as markup - legacy-cs's cheerio-based .replaceWith(text)
// has this same risk, unescaped; this is a deliberate small hardening, not
// a behavior change for ordinary text.
func replaceWithText(s *goquery.Selection) {
	s.ReplaceWithHtml(html.EscapeString(s.Text()))
}

// FixIcon removes Confluence's decorative icon spans, keeping any text
// they wrap. Matches legacy-cs Formatter.fixIcon/fixHeadline (identical
// implementations in legacy-cs; unified here).
func FixIcon(content *goquery.Selection) {
	content.Find("span.aui-icon").Each(func(_ int, s *goquery.Selection) {
		replaceWithText(s)
	})
}

// FixEmptyLink removes anchors with no text and no image, matching
// legacy-cs Formatter.fixEmptyLink.
func FixEmptyLink(content *goquery.Selection) {
	content.Find("a").Each(func(_ int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == "" && s.Find("img").Length() == 0 {
			s.Remove()
		}
	})
}

// FixEmptyHeading removes empty h1-h6 elements, matching legacy-cs
// Formatter.fixEmptyHeading (cheerio's ":header" pseudo-selector expanded
// explicitly, since goquery/cascadia doesn't support it).
func FixEmptyHeading(content *goquery.Selection) {
	content.Find("h1,h2,h3,h4,h5,h6").Each(func(_ int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == "" {
			s.Remove()
		}
	})
}

var cssDeclPattern = regexp.MustCompile(`([\w-]+)\s*:\s*([^;]+)`)

// FixPreformattedText replaces the SyntaxHighlighter-plugin styling
// convention on <pre> blocks with a plain "brush" (language) class,
// matching legacy-cs Formatter.fixPreformattedText. Unvalidated against a
// real Confluence Cloud export - the "data-syntaxhighlighter-params"
// attribute is a legacy Confluence Server/classic-macro convention; Cloud
// exports may render code blocks differently and this needs confirming
// once we have a real export to test against.
func FixPreformattedText(content *goquery.Selection) {
	content.Find("pre").Each(func(_ int, s *goquery.Selection) {
		data, hasData := s.Attr("data-syntaxhighlighter-params")
		if hasData {
			s.SetAttr("style", data)
		}
		style, _ := s.Attr("style")
		var brush string
		for _, m := range cssDeclPattern.FindAllStringSubmatch(style, -1) {
			if strings.TrimSpace(m[1]) == "brush" {
				brush = strings.TrimSpace(m[2])
			}
		}
		s.RemoveAttr("class")
		if brush != "" {
			// html-to-markdown/v2's code-block renderer (internal/convert)
			// only recognizes a "language-<lang>"/"lang-<lang>" class, not
			// a bare language name - confirmed via an end-to-end CLI run,
			// not just unit tests, since this only shows up once the
			// cleaner's output actually reaches the converter.
			s.AddClass("language-" + brush)
		}
	})
}

// FixImageWithinSpan unwraps "<span><img></span>" wrappers that carry no
// text of their own, matching legacy-cs Formatter.fixImageWithinSpan.
func FixImageWithinSpan(content *goquery.Selection) {
	content.Find("span").Each(func(_ int, s *goquery.Selection) {
		if s.Find("img").Length() > 0 && strings.TrimSpace(s.Text()) == "" {
			inner, err := s.Html()
			if err == nil {
				s.ReplaceWithHtml(inner)
			}
		}
	})
}

// RemoveArbitraryElements strips <span> and .user-mention wrappers, keeping
// their text, matching legacy-cs Formatter.removeArbitraryElements. This
// reduces rendered macros (status lozenges, user mentions, etc.) to plain
// text rather than any Markdown equivalent - legacy-cs behavior, not a new
// limitation introduced here.
func RemoveArbitraryElements(content *goquery.Selection) {
	content.Find("span, .user-mention").Each(func(_ int, s *goquery.Selection) {
		replaceWithText(s)
	})
}

var arbitraryClassPattern = regexp.MustCompile(`(^|\s)(confluence-\S+|external-link|uri|tablesorter-header-inner|odd|even|header)`)

// FixArbitraryClasses strips Confluence-theme-specific CSS classes from
// every element, matching legacy-cs Formatter.fixArbitraryClasses.
func FixArbitraryClasses(content *goquery.Selection) {
	content.Find("*").Each(func(_ int, s *goquery.Selection) {
		class, exists := s.Attr("class")
		if !exists {
			return
		}
		var kept []string
		for _, c := range strings.Fields(class) {
			if !arbitraryClassPattern.MatchString(" " + c) {
				kept = append(kept, c)
			}
		}
		if len(kept) == 0 {
			s.RemoveAttr("class")
		} else {
			s.SetAttr("class", strings.Join(kept, " "))
		}
	})
}

// FixAttachmentWrapper removes attachment-management UI chrome, matching
// legacy-cs Formatter.fixAttachmentWraper.
func FixAttachmentWrapper(content *goquery.Selection) {
	content.Find(".attachment-buttons").Remove()
	content.Find(".plugin_attachments_upload_container").Remove()
	content.Find("table.attachments.aui").Remove()
}

var pageLogIDPattern = regexp.MustCompile(`(Recentspaceactivity|Spacecontributors)$`)

// FixPageLog removes the "recent space activity"/"space contributors"
// panels, matching legacy-cs Formatter.fixPageLog.
func FixPageLog(content *goquery.Selection) {
	content.Find("*").Each(func(_ int, s *goquery.Selection) {
		id, exists := s.Attr("id")
		if exists && pageLogIDPattern.MatchString(id) {
			s.Parent().Remove()
		}
	})
}

// AddPageHeading prepends an <h1> with the page's resolved title, matching
// legacy-cs Formatter.addPageHeading.
func AddPageHeading(content *goquery.Selection, headingText string) {
	content.PrependHtml("<h1>" + html.EscapeString(headingText) + "</h1>")
}

// Pipeline runs the full export cleanup pass in the same order as
// legacy-cs Page.getTextToConvert, then returns the resulting inner HTML.
func Pipeline(doc *goquery.Selection, fileName, heading string) (string, error) {
	content := GetRightContent(doc, fileName)
	FixIcon(content)
	FixEmptyLink(content)
	FixEmptyHeading(content)
	FixPreformattedText(content)
	FixImageWithinSpan(content)
	RemoveArbitraryElements(content)
	FixArbitraryClasses(content)
	FixAttachmentWrapper(content)
	FixPageLog(content)
	AddPageHeading(content, heading)

	var out strings.Builder
	var err error
	content.Each(func(_ int, s *goquery.Selection) {
		h, e := s.Html()
		if e != nil {
			err = e
			return
		}
		out.WriteString(h)
	})
	return out.String(), err
}
