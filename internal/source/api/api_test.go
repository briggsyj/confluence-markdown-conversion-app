package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// pageFixture is a minimal stand-in for model.PageScheme's JSON shape.
type pageFixture struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	SpaceID    string `json:"spaceId"`
	ParentID   string `json:"parentId,omitempty"`
	ParentType string `json:"parentType,omitempty"`
	Body       *struct {
		Storage *struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body,omitempty"`
}

func writeJSON(t *testing.T, w http.ResponseWriter, v interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatal(err)
	}
}

// newMockServer sets up a Confluence v2 API surface for one space (id 111,
// key MYSPACE, homepage id 1) with three pages: Home (1, the homepage),
// Parent (2, child of Home), Child (3, child of Parent). Child has one
// attachment. Page listing is split across two pages of results to
// exercise cursor-based pagination.
func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	pages := map[string]pageFixture{
		"1": {ID: "1", Title: "Home", SpaceID: "111"},
		"2": {ID: "2", Title: "Parent", SpaceID: "111", ParentID: "1", ParentType: "page"},
		"3": {ID: "3", Title: "Child", SpaceID: "111", ParentID: "2", ParentType: "page"},
	}
	for id, p := range pages {
		p := p
		p.Body = &struct {
			Storage *struct {
				Value string `json:"value"`
			} `json:"storage"`
		}{Storage: &struct {
			Value string `json:"value"`
		}{Value: "<p>Body of " + p.Title + "</p>"}}
		pages[id] = p
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/wiki/api/v2/spaces", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]interface{}{
			"results": []map[string]string{
				{"id": "111", "key": "MYSPACE", "homepageId": "1"},
			},
		})
	})

	mux.HandleFunc("/wiki/api/v2/spaces/111/pages", func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		if cursor == "" {
			writeJSON(t, w, map[string]interface{}{
				"results": []pageFixture{pages["1"], pages["2"]},
				"_links":  map[string]string{"next": "/wiki/api/v2/spaces/111/pages?cursor=page2"},
			})
			return
		}
		if cursor == "page2" {
			writeJSON(t, w, map[string]interface{}{
				"results": []pageFixture{pages["3"]},
			})
			return
		}
		t.Fatalf("unexpected cursor %q", cursor)
	})

	mux.HandleFunc("/wiki/api/v2/pages/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/wiki/api/v2/pages/"):]
		p, ok := pages[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, p)
	})

	// Registered as exact paths so they take priority over the
	// "/wiki/api/v2/pages/" prefix pattern below (Go's http.ServeMux
	// prefers the more specific match).
	mux.HandleFunc("/wiki/api/v2/pages/3/attachments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]interface{}{
			"results": []map[string]string{
				{"id": "att1", "title": "diagram.png"},
			},
		})
	})
	mux.HandleFunc("/wiki/api/v2/pages/1/attachments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]interface{}{"results": []map[string]string{}})
	})
	mux.HandleFunc("/wiki/api/v2/pages/2/attachments", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]interface{}{"results": []map[string]string{}})
	})

	mux.HandleFunc("/wiki/api/v2/attachments/att1/download", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake-png-bytes"))
	})

	return httptest.NewServer(mux)
}

func TestLoad(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	src, err := New(Config{
		Site:     server.URL,
		Email:    "test@example.com",
		APIToken: "token",
		SpaceKey: "MYSPACE",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if result.RootSpace != "MYSPACE" {
		t.Errorf("RootSpace = %q, want MYSPACE", result.RootSpace)
	}
	if len(result.Pages) != 3 {
		t.Fatalf("expected 3 pages (pagination across 2 chunks), got %d", len(result.Pages))
	}

	byID := map[string]int{}
	for i, p := range result.Pages {
		byID[p.ConfluenceID] = i
	}

	home := result.Pages[byID["1"]]
	if !home.IsIndex {
		t.Errorf("expected Home (id 1) to be marked as the space index")
	}

	parent := result.Pages[byID["2"]]
	if len(parent.Ancestors) != 0 {
		t.Errorf("Parent's ancestors should stop at the homepage, got %v", parent.Ancestors)
	}

	child := result.Pages[byID["3"]]
	if len(child.Ancestors) != 1 || child.Ancestors[0] != "Parent" {
		t.Errorf("Child ancestors = %v, want [Parent]", child.Ancestors)
	}
	if child.HTML != "<p>Body of Child</p>" {
		t.Errorf("Child.HTML = %q", child.HTML)
	}
	if len(child.Attachments) != 1 || child.Attachments[0].FileName != "diagram.png" {
		t.Fatalf("Child.Attachments = %+v", child.Attachments)
	}

	data, err := os.ReadFile(child.Attachments[0].LocalPath)
	if err != nil {
		t.Fatalf("reading downloaded attachment: %v", err)
	}
	if string(data) != "fake-png-bytes" {
		t.Errorf("downloaded attachment content = %q", data)
	}
}
