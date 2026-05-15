package cmd

import (
	"testing"

	"github.com/bedri/open-directory-crawler/internal/models"
)

func TestUrlToID(t *testing.T) {
	id := urlToID("http://example.com/files/")
	if id == "" {
		t.Fatal("urlToID returned empty")
	}
	if len(id) != 16 {
		t.Errorf("urlToID len = %d, want 16", len(id))
	}
}

func TestUrlToIDConsistency(t *testing.T) {
	id1 := urlToID("http://example.com/files/")
	id2 := urlToID("http://example.com/files/")
	if id1 != id2 {
		t.Errorf("not consistent: %s vs %s", id1, id2)
	}
}

func TestUrlToIDDiffURLs(t *testing.T) {
	ids := make(map[string]bool)
	urls := []string{
		"http://a.com/",
		"http://b.com/",
		"http://a.com/dir1/",
		"http://a.com/dir2/",
	}
	for _, u := range urls {
		id := urlToID(u)
		if ids[id] {
			t.Errorf("duplicate id for %s: %s", u, id)
		}
		ids[id] = true
	}
}

func TestDirFromURL(t *testing.T) {
	d := dirFromURL("http://example.com/files/")
	if d == nil {
		t.Fatal("dirFromURL returned nil")
	}
	if d.ID == "" {
		t.Error("ID should not be empty")
	}
	if d.URL != "http://example.com/files/" {
		t.Errorf("URL = %q, want %q", d.URL, "http://example.com/files/")
	}
	if d.Status != models.StatusPending {
		t.Errorf("Status = %q, want %q", d.Status, models.StatusPending)
	}
}

func TestDirFromURLTrimsFragment(t *testing.T) {
	d := dirFromURL("http://example.com/files/")
	if d.ID != urlToID("http://example.com/files/") {
		t.Error("dirFromURL id should match urlToID")
	}
}
