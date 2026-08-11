// Package api implements source.Source over the live Confluence Cloud REST
// API (via go-atlassian), for users without a static HTML space export to
// hand - fetches Storage Format content directly, which is more stable
// than the export source's rendered-HTML scraping (see this project's
// plan doc for why both source modes exist).
//
// Unvalidated against a real live API pull as of writing (only exercised
// against a mocked httptest server in api_test.go) - the pagination
// cursor-extraction and ancestor-walk logic in particular need confirming
// against a real multi-page, multi-level space.
package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	v2 "github.com/ctreminiom/go-atlassian/v2/confluence/v2"
	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"

	"github.com/briggsyj/confluence-to-markdown/internal/page"
	"github.com/briggsyj/confluence-to-markdown/internal/source"
)

// Config holds the connection details for a single space pull.
type Config struct {
	// Site is the Confluence Cloud base URL, e.g.
	// "https://yourdomain.atlassian.net".
	Site string
	// Email is the Atlassian account email for basic-auth API token login.
	Email string
	// APIToken is the Atlassian API token (not the account password).
	APIToken string
	// SpaceKey is the key of the space to export, e.g. "MYSPACE".
	SpaceKey string
}

const pageLimit = 100

// Source pulls a single Confluence space's pages via the REST API.
type Source struct {
	cfg    Config
	client *v2.Client
}

// New builds an API source. It does not make any network calls itself.
func New(cfg Config) (*Source, error) {
	client, err := v2.New(nil, cfg.Site)
	if err != nil {
		return nil, fmt.Errorf("creating confluence client: %w", err)
	}
	client.Auth.SetBasicAuth(cfg.Email, cfg.APIToken)
	return &Source{cfg: cfg, client: client}, nil
}

// Load resolves the configured space, lists every page in it, fetches each
// page's Storage Format body, resolves its ancestor titles, and downloads
// its attachments to a temp directory.
func (s *Source) Load(ctx context.Context) (source.Result, error) {
	space, err := s.resolveSpace(ctx)
	if err != nil {
		return source.Result{}, fmt.Errorf("resolving space %q: %w", s.cfg.SpaceKey, err)
	}
	spaceID, err := strconv.Atoi(space.ID)
	if err != nil {
		return source.Result{}, fmt.Errorf("space ID %q is not numeric: %w", space.ID, err)
	}

	summaries, err := s.listAllPages(ctx, spaceID)
	if err != nil {
		return source.Result{}, fmt.Errorf("listing pages in space %q: %w", s.cfg.SpaceKey, err)
	}
	byID := make(map[string]*model.PageScheme, len(summaries))
	for _, p := range summaries {
		byID[p.ID] = p
	}

	tmpDir, err := os.MkdirTemp("", "confluence-md-attachments-*")
	if err != nil {
		return source.Result{}, err
	}

	result := source.Result{RootSpace: space.Key}
	for _, summary := range summaries {
		id, err := strconv.Atoi(summary.ID)
		if err != nil {
			return source.Result{}, fmt.Errorf("page ID %q is not numeric: %w", summary.ID, err)
		}

		full, _, err := s.client.Page.Get(ctx, id, "storage", false, 0)
		if err != nil {
			return source.Result{}, fmt.Errorf("fetching page %q body: %w", summary.ID, err)
		}
		var html string
		if full.Body != nil && full.Body.Storage != nil {
			html = full.Body.Storage.Value
		}

		attachments, err := s.downloadAttachments(ctx, id, tmpDir)
		if err != nil {
			return source.Result{}, fmt.Errorf("downloading attachments for page %q: %w", summary.ID, err)
		}

		result.Pages = append(result.Pages, page.Page{
			ConfluenceID: summary.ID,
			Title:        summary.Title,
			Space:        space.Key,
			IsIndex:      summary.ID == space.HomepageID,
			Ancestors:    ancestorTitles(byID, summary.ParentID, space.HomepageID),
			HTML:         html,
			Attachments:  attachments,
		})
	}
	return result, nil
}

func (s *Source) resolveSpace(ctx context.Context) (*model.SpaceSchemeV2, error) {
	chunk, _, err := s.client.Space.Bulk(ctx, &model.GetSpacesOptionSchemeV2{Keys: []string{s.cfg.SpaceKey}}, "", 1)
	if err != nil {
		return nil, err
	}
	if len(chunk.Results) == 0 {
		return nil, fmt.Errorf("no space found with key %q", s.cfg.SpaceKey)
	}
	return chunk.Results[0], nil
}

func (s *Source) listAllPages(ctx context.Context, spaceID int) ([]*model.PageScheme, error) {
	var all []*model.PageScheme
	cursor := ""
	for {
		chunk, _, err := s.client.Page.GetsBySpace(ctx, spaceID, cursor, pageLimit)
		if err != nil {
			return nil, err
		}
		all = append(all, chunk.Results...)

		next := ""
		if chunk.Links != nil {
			next = chunk.Links.Next
		}
		if next == "" {
			return all, nil
		}
		cursor, err = extractCursor(next)
		if err != nil {
			return nil, fmt.Errorf("parsing pagination cursor from %q: %w", next, err)
		}
		if cursor == "" {
			return all, nil
		}
	}
}

// extractCursor pulls the "cursor" query parameter out of a "_links.next"
// value, which go-atlassian's v2 API returns as a path-and-query fragment
// rather than a bare cursor token.
func extractCursor(next string) (string, error) {
	u, err := url.Parse(next)
	if err != nil {
		return "", err
	}
	return u.Query().Get("cursor"), nil
}

// ancestorTitles walks the parent chain from a page's own parentID up to
// (but not including) the space's homepage, collecting titles root-first -
// the API equivalent of the export source's sliced breadcrumb list.
func ancestorTitles(byID map[string]*model.PageScheme, parentID, homepageID string) []string {
	var reversed []string
	for id := parentID; id != "" && id != homepageID; {
		parent, ok := byID[id]
		if !ok {
			break // parent outside this space's page set (e.g. a folder we didn't index)
		}
		reversed = append(reversed, parent.Title)
		id = parent.ParentID
	}
	ancestors := make([]string, len(reversed))
	for i, t := range reversed {
		ancestors[len(reversed)-1-i] = t
	}
	return ancestors
}

func (s *Source) downloadAttachments(ctx context.Context, pageID int, tmpDir string) ([]page.Attachment, error) {
	list, _, err := s.client.Attachment.Gets(ctx, pageID, "pages", nil, "", pageLimit)
	if err != nil {
		return nil, err
	}

	var attachments []page.Attachment
	for _, a := range list.Results {
		rc, err := s.client.Attachment.Download(ctx, a.ID)
		if err != nil {
			return nil, err
		}
		localPath := filepath.Join(tmpDir, a.ID+"_"+a.Title)
		if err := writeReaderToFile(rc, localPath); err != nil {
			return nil, err
		}
		attachments = append(attachments, page.Attachment{
			FileName:  a.Title,
			LocalPath: localPath,
		})
	}
	return attachments, nil
}

func writeReaderToFile(rc io.ReadCloser, path string) error {
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rc); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
