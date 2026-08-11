// Package source defines the interface both the export and API sources
// implement to produce pages for conversion.
package source

import (
	"context"

	"github.com/briggsyj/confluence-to-markdown/internal/page"
)

// AssetDir is a directory-level asset copy to perform after conversion,
// matching legacy-cs Utils.copyAssets: for a given space/localDir, copy
// any "images"/"attachments" subfolders found under SourcePath.
type AssetDir struct {
	Space      string
	LocalDir   string
	SourcePath string
}

// Result is everything a Source produces for one conversion run.
type Result struct {
	Pages     []page.Page
	AssetDirs []AssetDir
	// RootSpace is the Confluence space key read from the export's index
	// page detail table, matching legacy-cs App.convert's "rootSpace"
	// (passed on to the link-fix pass). Empty for the API source, which
	// already knows its space key directly.
	RootSpace string
}

// Source produces the pages (and any directory-level assets) to convert.
type Source interface {
	Load(ctx context.Context) (Result, error)
}
