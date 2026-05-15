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
