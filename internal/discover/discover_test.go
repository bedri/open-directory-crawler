package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	f := New()
	if f == nil {
		t.Fatal("New() returned nil")
	}
	if f.client == nil {
		t.Error("client should not be nil")
	}
	if f.client.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", f.client.Timeout)
	}
	if len(f.userAgents) == 0 {
		t.Error("userAgents should not be empty")
	}
}

func TestRandomUA(t *testing.T) {
	f := New()
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		ua := f.randomUA()
		if ua == "" {
			t.Error("randomUA returned empty")
		}
		seen[ua] = true
	}
	if len(seen) < 2 {
		t.Errorf("randomUA doesn't vary enough, got %d unique", len(seen))
	}
}

func TestTypeDorks(t *testing.T) {
	expectedTypes := []string{"audio", "video", "image", "document", "archive", "code"}
	for _, typ := range expectedTypes {
		dorks, ok := TypeDorks[typ]
		if !ok {
			t.Errorf("TypeDorks missing key %q", typ)
			continue
		}
		if len(dorks) == 0 {
			t.Errorf("TypeDorks[%q] is empty", typ)
		}
	}
	if len(TypeDorks) != len(expectedTypes) {
		t.Errorf("TypeDorks has %d keys, want %d", len(TypeDorks), len(expectedTypes))
	}
}

func TestDorksNotEmpty(t *testing.T) {
	if len(Dorks) == 0 {
		t.Error("Dorks should not be empty")
	}
	for i, d := range Dorks {
		if d == "" {
			t.Errorf("Dorks[%d] is empty", i)
		}
	}
}

func TestSaveResults(t *testing.T) {
	f := New()
	dir := t.TempDir()
	path := filepath.Join(dir, "results.json")

	results := []Result{
		{URL: "http://example.com/", Source: "test", Verified: true},
		{URL: "http://example2.com/", Source: "test", Verified: false},
	}

	if err := f.SaveResults(results, path); err != nil {
		t.Fatalf("SaveResults: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "example.com") {
		t.Error("saved file missing expected data")
	}
}

func TestResultStruct(t *testing.T) {
	r := Result{
		URL:      "http://test.com/",
		Title:    "Test",
		Source:   "google",
		Verified: true,
	}
	if r.URL != "http://test.com/" || r.Title != "Test" || r.Source != "google" || !r.Verified {
		t.Errorf("Result fields mismatch: %+v", r)
	}
}

func TestParseGoogleResults(t *testing.T) {
	body := `<html><body>
<a href="/url?q=http://example.com/files/&sa=U&ved=2ahUKEwjRgNf7hqSJAxWpE1kFHYTzA3sQFnoECAcQAQ"></a>
<a href="/url?q=http://other.com/&sa=U&ved=..."></a>
</body></html>`

	results := parseGoogleResults(body, "test query")
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].URL != "http://example.com/files/" {
		t.Errorf("result[0].URL = %q", results[0].URL)
	}
	if results[0].Source != "google" {
		t.Errorf("result[0].Source = %q", results[0].Source)
	}
}

func TestParseGoogleResultsEmpty(t *testing.T) {
	results := parseGoogleResults("<html></html>", "test")
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestParseGoogleResultsNoMatch(t *testing.T) {
	results := parseGoogleResults("<html><a href='http://direct.com/'>link</a></html>", "test")
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestTypeDorksContent(t *testing.T) {
	for typ, dorks := range TypeDorks {
		for i, d := range dorks {
			if !strings.Contains(d, "index of") && !strings.Contains(d, "Index of") {
				t.Errorf("TypeDorks[%q][%d] doesn't contain 'index of': %q", typ, i, d)
			}
		}
	}
}
