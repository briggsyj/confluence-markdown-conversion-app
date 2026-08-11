// Package convert wires html-to-markdown/v2 (replacing turndown) plus
// hand-written Confluence rules (replacing turndown-plugin-confluence-to-gfm,
// which has no Go equivalent) into a single Converter.
package convert

import (
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
)

// Converter converts cleaned page HTML into Markdown.
type Converter struct {
	conv *converter.Converter
}

// New builds a Converter with CommonMark + GFM tables/strikethrough
// (replacing turndown + joplin-turndown-plugin-gfm), plus the Confluence
// storage-format rules registered below (which include task-list handling
// - Confluence's real task lists are "ac:task-list", not GFM's plain
// "<ul><li><input>", so there was no separate generic rule to add).
func New() *Converter {
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
			strikethrough.NewStrikethroughPlugin(),
		),
	)
	RegisterConfluenceStorageRules(conv)
	return &Converter{conv: conv}
}

// Convert renders an HTML fragment to Markdown.
func (c *Converter) Convert(htmlInput string) (string, error) {
	return c.conv.ConvertString(htmlInput)
}
