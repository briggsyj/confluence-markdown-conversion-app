// Package page defines the source-agnostic page model that both the export
// and API sources produce, and that the conversion/link-fix stages consume.
package page

// Attachment is a file referenced by a Page (image, upload, etc.).
type Attachment struct {
	// FileName is the attachment's file name as it should appear on disk.
	FileName string
	// LocalPath is where the attachment content can currently be read from
	// (already-downloaded temp path for the API source, or the export
	// directory's attachments/images folder for the export source).
	LocalPath string
}

// Page is the common representation of a single Confluence page, regardless
// of whether it came from a static HTML export or a live API call.
type Page struct {
	// ConfluenceID is the page's Confluence content ID, empty for the
	// space's root/index page.
	ConfluenceID string
	// Title is the page's display title.
	Title string
	// Space is the Confluence space key the page belongs to.
	Space string
	// IsIndex marks the space's root/home page.
	IsIndex bool
	// Ancestors are the titles of this page's parent pages, root first,
	// not including the page itself. Used to derive the output directory.
	Ancestors []string
	// HTML is the page's content, already cleaned/normalized by the
	// source-specific cleaner and ready for markdown conversion.
	HTML string
	// Attachments are files referenced by this page.
	Attachments []Attachment
}
