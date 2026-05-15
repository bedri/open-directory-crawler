package cmd

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bedri/open-directory-crawler/internal/crawler"
	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
)

func newTestStoreForAgent(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := storage.New(dir)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestImportURLs(t *testing.T) {
	store := newTestStoreForAgent(t)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "urls.txt")
	content := "http://example.com/files/\nhttp://other.com/data/\n# comment\n\nhttp://third.com/\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	importURLs(store, filePath, logger)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "Imported 3") {
		t.Errorf("log = %q, want 'Imported 3'", logOutput)
	}

	dirs, _ := store.ListDirectories()
	if len(dirs) != 3 {
		t.Fatalf("got %d dirs, want 3", len(dirs))
	}
}

func TestImportURLsSkipExisting(t *testing.T) {
	store := newTestStoreForAgent(t)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "urls.txt")

	existingURL := "http://example.com/files/"
	if err := os.WriteFile(filePath, []byte(existingURL+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	importURLs(store, filePath, logger)
	importURLs(store, filePath, logger)

	if !strings.Contains(buf.String(), "1 skipped") {
		t.Errorf("should have skipped existing: %q", buf.String())
	}

	dirs, _ := store.ListDirectories()
	if len(dirs) != 1 {
		t.Errorf("expected 1 dir after duplicate import, got %d", len(dirs))
	}
}

func TestImportURLsFileNotFound(t *testing.T) {
	store := newTestStoreForAgent(t)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	importURLs(store, "/nonexistent/file.txt", logger)

	if !strings.Contains(buf.String(), "import file error") {
		t.Errorf("expected error log, got: %q", buf.String())
	}
}

func TestImportURLsSkippedByStatus(t *testing.T) {
	store := newTestStoreForAgent(t)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "urls.txt")

	d1 := dirFromURL("http://a.com/")
	d1.Status = models.StatusDone
	store.SaveDirectory(d1)

	if err := os.WriteFile(filePath, []byte("http://a.com/\nhttp://b.com/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	importURLs(store, filePath, logger)

	dirs, _ := store.ListDirectories()
	if len(dirs) != 2 {
		t.Fatalf("expected 2 dirs, got %d", len(dirs))
	}

	existing, _ := store.GetDirectory(d1.ID)
	if existing.Status != models.StatusDone {
		t.Errorf("existing dir status overwritten: %q, want %q", existing.Status, models.StatusDone)
	}
}

func TestImportURLsEmptyLines(t *testing.T) {
	store := newTestStoreForAgent(t)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "urls.txt")
	if err := os.WriteFile(filePath, []byte("\n\n# comment\n\n"), 0644); err != nil {
		t.Fatal(err)
	}

	importURLs(store, filePath, logger)

	dirs, _ := store.ListDirectories()
	if len(dirs) != 0 {
		t.Errorf("expected 0 dirs for empty file, got %d", len(dirs))
	}
}

func TestImportURLsLargeFile(t *testing.T) {
	store := newTestStoreForAgent(t)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "large.txt")

	var lines []string
	for i := 0; i < 1000; i++ {
		lines = append(lines, fmt.Sprintf("http://example.com/dir%d/\n", i))
	}
	if err := os.WriteFile(filePath, []byte(strings.Join(lines, "")), 0644); err != nil {
		t.Fatal(err)
	}

	importURLs(store, filePath, logger)

	dirs, _ := store.ListDirectories()
	if len(dirs) != 1000 {
		t.Errorf("expected 1000 dirs, got %d", len(dirs))
	}
}

func TestProcessPendingEmpty(t *testing.T) {
	store := newTestStoreForAgent(t)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	processed := processPending(store, logger, crawler.Config{})
	if processed != 0 {
		t.Errorf("expected 0 processed, got %d", processed)
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{",,a,,b,", []string{"a", "b"}},
	}
	for _, tc := range tests {
		got := splitCSV(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("splitCSV(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitCSV(%q) = %v, want %v", tc.input, got, tc.want)
				break
			}
		}
	}
}

func TestCleanupStuckScanning(t *testing.T) {
	store := newTestStoreForAgent(t)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	store.SaveDirectory(&models.Directory{ID: "d1", URL: "http://a.com/", Status: models.StatusScanning})
	store.SaveDirectory(&models.Directory{ID: "d2", URL: "http://b.com/", Status: models.StatusDone})
	store.SaveDirectory(&models.Directory{ID: "d3", URL: "http://c.com/", Status: models.StatusPending})

	cleanupStuckScanning(store, logger)

	d1, _ := store.GetDirectory("d1")
	if d1.Status != models.StatusPending {
		t.Errorf("stuck scanning dir should be pending, got %q", d1.Status)
	}
	d2, _ := store.GetDirectory("d2")
	if d2.Status != models.StatusDone {
		t.Errorf("done dir should stay done, got %q", d2.Status)
	}
	d3, _ := store.GetDirectory("d3")
	if d3.Status != models.StatusPending {
		t.Errorf("pending dir should stay pending, got %q", d3.Status)
	}
	if !strings.Contains(buf.String(), "Reset 1") {
		t.Errorf("expected log about 1 reset, got %q", buf.String())
	}
}

func TestCleanupStuckScanningEmpty(t *testing.T) {
	store := newTestStoreForAgent(t)
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	cleanupStuckScanning(store, logger)
	if strings.Contains(buf.String(), "Reset") {
		t.Errorf("unexpected reset log for empty store")
	}
}

func TestAgentDirFromURL(t *testing.T) {
	d := dirFromURL("http://example.com/path/to/dir/")
	if d.ID == "" {
		t.Error("expected non-empty ID")
	}
	if d.URL != "http://example.com/path/to/dir/" {
		t.Errorf("URL = %q", d.URL)
	}
	if d.Status != models.StatusPending {
		t.Errorf("Status = %q, want pending", d.Status)
	}

	d2 := dirFromURL("https://sub.example.org:8080/data/")
	if d2.URL != "https://sub.example.org:8080/data/" {
		t.Errorf("URL = %q", d2.URL)
	}
}

func TestAgentDirFromURLUniqueIDs(t *testing.T) {
	d1 := dirFromURL("http://a.com/")
	d2 := dirFromURL("http://b.com/")
	if d1.ID == d2.ID {
		t.Errorf("different URLs should have different IDs")
	}
}

func TestAgentDirFromURLSameURL(t *testing.T) {
	d1 := dirFromURL("http://example.com/files/")
	d2 := dirFromURL("http://example.com/files/")
	if d1.ID != d2.ID {
		t.Errorf("same URL should produce same ID")
	}
}
