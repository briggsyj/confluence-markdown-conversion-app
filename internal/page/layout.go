package page

import (
	"regexp"
	"strings"
)

// sanitizePattern matches the exact character set legacy-cs's
// Utils.sanitizeFilename replaces: whitespace plus the Windows-reserved
// filename characters (kept even though we're on POSIX, since this
// determines Markdown link/file compatibility, not just OS legality).
var sanitizePattern = regexp.MustCompile(`[\s<>()\[\]{}:;'` + "`" + `"/\\|?*~!@#$%^&,]`)

// collapseSpaces matches two-or-more consecutive spaces left behind by
// sanitizePattern, mirroring legacy-cs's second .replace(/  +/g, ' ') pass.
var collapseSpaces = regexp.MustCompile(` +`)

// SanitizeFilename replaces filesystem/URL-unsafe characters with a space
// and collapses runs of spaces, matching legacy-cs's Utils.sanitizeFilename
// byte-for-byte (including that it does not trim leading/trailing spaces).
func SanitizeFilename(name string) string {
	replaced := sanitizePattern.ReplaceAllString(name, " ")
	return collapseSpaces.ReplaceAllString(replaced, " ")
}

// confluenceIDPattern extracts a trailing run of digits from a page's file
// base name (e.g. "My Page_123456789" -> "123456789"), matching legacy-cs's
// Utils.getConfluenceIdFromName.
var confluenceIDPattern = regexp.MustCompile(`[0-9]+$`)

// ConfluenceIDFromBasename returns the Confluence content ID embedded in an
// export file's base name, and false for the space's index page or any
// basename with no trailing digits (legacy-cs throws in the latter case;
// we return false instead so a single malformed file name doesn't abort a
// whole-space conversion).
func ConfluenceIDFromBasename(basename string) (string, bool) {
	if basename == "index" {
		return "", false
	}
	match := confluenceIDPattern.FindString(basename)
	if match == "" {
		return "", false
	}
	return match, true
}

// LocalDirFromSegments sanitizes and joins already-resolved ancestor page
// titles into the relative output directory for a page, matching
// legacy-cs's Formatter.getLocalDir. Segment extraction is source-specific
// (breadcrumb parsing for the export source, the ancestors list for the API
// source) and happens before this call.
func LocalDirFromSegments(segments []string) string {
	sanitized := make([]string, len(segments))
	for i, s := range segments {
		sanitized[i] = SanitizeFilename(s)
	}
	return strings.Join(sanitized, "/")
}

// FileNameNew returns the output Markdown file name for a page, matching
// legacy-cs's Page.getFileNameNew/getFileNameNewRaw: "index.md" for the
// space's index page, otherwise the sanitized heading plus ".md".
func FileNameNew(isIndex bool, heading string) string {
	if isIndex {
		return "index.md"
	}
	return SanitizeFilename(heading) + ".md"
}

// SpacePath returns the relative path used to link to a page from a
// sibling space's output directory, matching legacy-cs's Page.getSpacePath.
func SpacePath(space, fileNameNew string) string {
	return "../" + SanitizeFilename(space) + "/" + fileNameNew
}
