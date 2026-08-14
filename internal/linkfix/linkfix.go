// Package linkfix is a native port of legacy-cs's update-links.sh: it
// rewrites Confluence-URL/placeholder links into relative paths across an
// already-converted directory of Markdown files, and reorganizes each page
// (and its attachments) into its own per-article directory.
//
// Two regexes in the original bash script have a bug worth noting: the
// title-matching character classes in both link-rewrite rules were written
// as "[\^/]"/"[\^(/]" (an escaped literal caret) where a negated class
// "[^/]"/"[^(/]" was clearly intended (matching "any char except slash",
// the standard idiom for a URL path segment). As escaped, those classes
// only match literal "^" characters, which makes both rules effectively
// inert against any real page title. This port uses the evidently-intended
// negated classes instead of reproducing the bug.
package linkfix

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Options mirrors update-links.sh's flags.
type Options struct {
	// ArticleDir mirrors the default-true "--no-article-dir" flag: move
	// each page into its own "<Title>/<Title>.md" directory and reorganize
	// attachments alongside it.
	ArticleDir bool
	// NoFileSizeLimit mirrors "--no-file-size-limit": skip the 10MB
	// attachment size guard.
	NoFileSizeLimit bool
}

// Run performs the full post-process pass over targetDir, matching
// legacy-cs's update-links.sh end to end.
func Run(targetDir, space, confluenceURL string, opts Options) error {
	if err := moveRootPagesIntoDirectories(targetDir); err != nil {
		return fmt.Errorf("moving root pages into directories: %w", err)
	}

	if opts.ArticleDir {
		if err := createArticleDirectories(targetDir); err != nil {
			return fmt.Errorf("creating article directories: %w", err)
		}
	}

	idIndex, err := buildConfluenceIDIndex(targetDir, space)
	if err != nil {
		return fmt.Errorf("indexing confluence IDs: %w", err)
	}

	if err := rewriteLinks(targetDir, space, confluenceURL, idIndex); err != nil {
		return fmt.Errorf("rewriting links: %w", err)
	}

	if opts.ArticleDir {
		if err := reorganizeAttachments(targetDir, opts.NoFileSizeLimit); err != nil {
			return fmt.Errorf("reorganizing attachments: %w", err)
		}
	}

	return nil
}

// -- step 1: move root pages into pre-existing same-named directories ----

func moveRootPagesIntoDirectories(targetDir string) error {
	files, err := listMarkdownFiles(targetDir)
	if err != nil {
		return err
	}
	for _, f := range files {
		matchingDir := strings.TrimSuffix(f, filepath.Ext(f))
		info, err := os.Stat(matchingDir)
		if err == nil && info.IsDir() {
			if err := os.Rename(f, filepath.Join(matchingDir, filepath.Base(f))); err != nil {
				return err
			}
		}
	}
	return nil
}

// -- step 2: give every article its own "<Title>/<Title>.md" directory --

func createArticleDirectories(targetDir string) error {
	files, err := listMarkdownFiles(targetDir)
	if err != nil {
		return err
	}
	for _, f := range files {
		baseName := strings.TrimSuffix(filepath.Base(f), ".md")
		dir := filepath.Dir(f)
		directDir := filepath.Base(dir)

		if directDir == baseName {
			count, err := countMarkdownFilesInDir(dir)
			if err != nil {
				return err
			}
			if count == 1 {
				continue
			}
		}

		newDir := filepath.Join(dir, baseName)
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			return err
		}
		if err := os.Rename(f, filepath.Join(newDir, baseName+".md")); err != nil {
			return err
		}
	}
	return nil
}

func countMarkdownFilesInDir(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			count++
		}
	}
	return count, nil
}

// -- step 3: index each file's own Confluence ID from its frontmatter ----

// frontMatterCloseDelim matches a frontmatter delimiter line, escaped or
// not, for FSM state tracking - matched against one line at a time (see
// extractConfluenceID), so it doesn't need a multiline flag.
var frontMatterCloseDelim = regexp.MustCompile(`^\\?---$`)
var confluenceIDLine = regexp.MustCompile(`confluence-id:`)

// escapedFrontMatterDelim un-escapes a frontmatter delimiter line that
// turndown rendered as "\---" (to avoid it reading as a Markdown thematic
// break), matching legacy-cs's "s/^\\---$/---/;" rule. Deliberately only
// matches the escaped form - a bare "---" line elsewhere in the document
// (e.g. a real thematic break) must not be touched.
var escapedFrontMatterDelim = regexp.MustCompile(`(?m)^\\---$`)

// buildConfluenceIDIndex scans every Markdown file's frontmatter for its
// "confluence-id:" field, matching legacy-cs's awk script (which assumes
// line 1 always opens the frontmatter block, since App.coffee's writer
// always emits it first). The index is keyed "<space>_<id>" using the run's
// single space argument for every file, matching legacy-cs's own
// placeholder convention - see rewriteLinks for why this means cross-space
// links can't resolve.
func buildConfluenceIDIndex(targetDir, space string) (map[string]string, error) {
	files, err := listMarkdownFiles(targetDir)
	if err != nil {
		return nil, err
	}

	index := make(map[string]string)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		id := extractConfluenceID(string(raw))
		if id == "" {
			continue
		}
		rel, err := filepath.Rel(targetDir, f)
		if err != nil {
			return nil, err
		}
		index[space+"_"+id] = filepath.ToSlash(rel)
	}
	return index, nil
}

func extractConfluenceID(content string) string {
	inFrontMatter := false
	for i, line := range strings.Split(content, "\n") {
		if frontMatterCloseDelim.MatchString(line) {
			inFrontMatter = false
		}
		if inFrontMatter && confluenceIDLine.MatchString(line) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
		if i == 0 {
			inFrontMatter = true
		}
	}
	return ""
}

// -- step 4: rewrite links -------------------------------------------------

// malformedLocalLinkPattern matches an internal link that converted to a
// bare local ".html" filename, e.g. "(Text-Formatting_98643.html)" or
// "(65706.html)" - confirmed against a real Confluence Cloud export to be
// the *normal* way static-export internal links render (not the rare
// "incorrectly converts" edge case legacy-cs's own comment suggested).
// legacy-cs hardcoded the ID as exactly 10 digits ("[0-9]{10}"), which
// never matches this space's real (shorter) page IDs - another bug found
// by actually running this against real export data, not just legacy-cs's
// own assumptions. The prefix must be lazy ("*?"), not greedy: since the
// title-prefix class doesn't exclude digits (titles can legitimately
// contain them), a greedy prefix would swallow all but the last digit of
// the ID before handing off to the capture group.
var malformedLocalLinkPattern = regexp.MustCompile(`\([^(/]*?([0-9]+)\.html\)`)

var confluenceSpacePlaceholder = "%%CONFLUENCE-SPACE%%"

// rewriteLinks normalizes raw Confluence URLs and malformed local links
// into "%%CONFLUENCE_<space>_<id>%%" placeholders, then resolves every
// placeholder matching a page in idIndex to a Markdown link target that is
// relative to the referencing file's own directory.
//
// The file-relative resolution is a deliberate divergence from legacy-cs,
// which emitted paths relative to targetDir (the space root) instead - a
// convention that only works in a wiki engine resolving links root-relative
// by page path, and produces broken links in ordinary Markdown editors
// (Typora, VS Code, Obsidian, static-site generators) that resolve a link
// relative to the file containing it. The ".md" extension is kept, since
// those same tools link to real files on disk.
//
// Links to a *different* space than this run's `space` argument are only
// ever turned into placeholders (via the raw-URL rule, which captures the
// space from the URL itself) but can never be resolved by idIndex (which is
// keyed to this run's single space) - they're left as literal
// "%%CONFLUENCE_...%%" text, matching legacy-cs's inherently
// single-space-per-run design.
func rewriteLinks(targetDir, space, confluenceURL string, idIndex map[string]string) error {
	rawURLPattern := regexp.MustCompile(regexp.QuoteMeta(confluenceURL) + `([^/]*)/pages/([^/]*)/[^)]*`)

	files, err := listMarkdownFiles(targetDir)
	if err != nil {
		return err
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		content := string(raw)

		content = rawURLPattern.ReplaceAllString(content, "%%CONFLUENCE_${1}_${2}%%")
		content = malformedLocalLinkPattern.ReplaceAllString(content, "(%%CONFLUENCE_"+space+"_${1}%%)")
		content = strings.ReplaceAll(content, confluenceSpacePlaceholder, space)
		content = escapedFrontMatterDelim.ReplaceAllString(content, "---")

		fromDir := filepath.Dir(f)
		for key, relPath := range idIndex {
			placeholder := "%%CONFLUENCE_" + key + "%%"
			if !strings.Contains(content, placeholder) {
				continue
			}
			// idIndex stores targetDir-relative slash paths; re-anchor each
			// to the file doing the linking so the result works in a plain
			// file-relative Markdown editor.
			target, err := filepath.Rel(fromDir, filepath.Join(targetDir, filepath.FromSlash(relPath)))
			if err != nil {
				return err
			}
			content = strings.ReplaceAll(content, placeholder, "<"+filepath.ToSlash(target)+">")
		}

		if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// -- step 5: reorganize attachments ---------------------------------------

// attachmentLinkPattern matches a Markdown link target under "attachments/".
// Simplified from legacy-cs's Perl pattern, which additionally tolerated a
// literal escaped ")" inside the path and relied on backtracking to
// separate a trailing "?query" suffix - a construct Go's RE2-based regexp
// package can't express (no backtracking). Attachment paths containing a
// literal, unescaped ")" character are the one known gap versus legacy-cs.
var attachmentLinkPattern = regexp.MustCompile(`\]\((attachments[^)]*)\)`)

func reorganizeAttachments(targetDir string, noFileSizeLimit bool) error {
	files, err := listMarkdownFiles(targetDir)
	if err != nil {
		return err
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		content := string(raw)

		attachments := uniqueAttachmentPaths(content)
		dir := filepath.Dir(f)

		for _, attachment := range attachments {
			relPath, _, _ := strings.Cut(attachment, "?")
			// Attachment paths in the Markdown are relative to targetDir
			// (the space root), not to the referencing file's own,
			// possibly-just-relocated directory - matching legacy-cs,
			// whose whole script runs from inside a "cd $targetDir".
			path := filepath.Join(targetDir, relPath)

			info, err := os.Stat(path)
			if err != nil {
				continue // matches legacy-cs's silent skip of a dangling reference
			}
			if !noFileSizeLimit && info.Size() > 10_000_000 {
				fmt.Fprintf(os.Stderr, "WARNING: Refusing to copy attachment %q from file %q (over 10MB)\n", path, f)
				continue
			}

			base := filepath.Base(path)
			if !strings.Contains(base, ".") {
				if ext := sniffExtension(path); ext != "" {
					base += ext
				}
			}

			if err := copyFile(path, filepath.Join(dir, base)); err != nil {
				return err
			}
			content = strings.ReplaceAll(content, attachment, "./"+base)
		}

		if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func uniqueAttachmentPaths(content string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, m := range attachmentLinkPattern.FindAllStringSubmatch(content, -1) {
		path := m[1]
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}

// sniffExtension guesses a file extension from content, replacing
// legacy-cs's shell-out to `file --extension`. Deliberately dependency-free
// (net/http+mime, both stdlib) rather than shelling out to the `file`
// binary, matching the point of a single-static-binary rewrite - at the
// cost of being less comprehensive than libmagic's detection.
func sniffExtension(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	// DetectContentType returns e.g. "image/png; charset=utf-8" -
	// mime.ExtensionsByType wants just the type.
	contentType, _, _ = strings.Cut(contentType, ";")

	exts, err := mime.ExtensionsByType(strings.TrimSpace(contentType))
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0]
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// -- shared helpers ---------------------------------------------------------

func listMarkdownFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}
