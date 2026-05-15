package storage

import (
	"fmt"
	"os"
	"testing"

	"github.com/bedri/open-directory-crawler/internal/models"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir, err := os.MkdirTemp("", "odk-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSaveGetDirectory(t *testing.T) {
	s := newTestStore(t)
	d := &models.Directory{
		ID:     "test123",
		URL:    "http://example.com/",
		Status: models.StatusPending,
	}
	if err := s.SaveDirectory(d); err != nil {
		t.Fatalf("SaveDirectory: %v", err)
	}
	got, err := s.GetDirectory("test123")
	if err != nil {
		t.Fatalf("GetDirectory: %v", err)
	}
	if got.URL != d.URL || got.Status != d.Status {
		t.Errorf("GetDirectory = %+v, want %+v", got, d)
	}
}

func TestGetDirectoryNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetDirectory("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestOverwriteDirectory(t *testing.T) {
	s := newTestStore(t)
	d1 := &models.Directory{ID: "test1", URL: "http://a.com/", Status: models.StatusPending}
	if err := s.SaveDirectory(d1); err != nil {
		t.Fatal(err)
	}
	d2 := &models.Directory{ID: "test1", URL: "http://a.com/", Status: models.StatusDone, FileCount: 10}
	if err := s.SaveDirectory(d2); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetDirectory("test1")
	if got.Status != models.StatusDone || got.FileCount != 10 {
		t.Errorf("overwrite failed: %+v", got)
	}
}

func TestListDirectories(t *testing.T) {
	s := newTestStore(t)
	urls := []string{"http://a.com/", "http://b.com/", "http://c.com/"}
	for _, u := range urls {
		s.SaveDirectory(&models.Directory{ID: u, URL: u})
	}
	dirs, err := s.ListDirectories()
	if err != nil {
		t.Fatalf("ListDirectories: %v", err)
	}
	if len(dirs) != 3 {
		t.Errorf("got %d dirs, want 3", len(dirs))
	}
}

func TestSaveFileEntry(t *testing.T) {
	s := newTestStore(t)
	f := &models.FileEntry{
		ID:          "dir1:test.mp3",
		Name:        "test.mp3",
		URL:         "http://example.com/test.mp3",
		Size:        1024,
		Ext:         "mp3",
		Category:    models.CategoryAudio,
		DirectoryID: "dir1",
	}
	if err := s.SaveFileEntry(f); err != nil {
		t.Fatalf("SaveFileEntry: %v", err)
	}
}

func TestGetFilesByDir(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("file%d.mp3", i)
		s.SaveFileEntry(&models.FileEntry{
			ID:          "dir1:" + name,
			Name:        name,
			DirectoryID: "dir1",
			Category:    models.CategoryAudio,
			Ext:         "mp3",
		})
	}
	files, err := s.GetFilesByDir("dir1")
	if err != nil {
		t.Fatalf("GetFilesByDir: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("got %d files, want 3", len(files))
	}
}

func TestGetFilesByCategory(t *testing.T) {
	s := newTestStore(t)
	s.SaveFileEntry(&models.FileEntry{ID: "d1:a.mp3", DirectoryID: "d1", Name: "a.mp3", Category: models.CategoryAudio, Ext: "mp3"})
	s.SaveFileEntry(&models.FileEntry{ID: "d1:b.mp4", DirectoryID: "d1", Name: "b.mp4", Category: models.CategoryVideo, Ext: "mp4"})
	s.SaveFileEntry(&models.FileEntry{ID: "d2:c.mp3", DirectoryID: "d2", Name: "c.mp3", Category: models.CategoryAudio, Ext: "mp3"})

	audio, _ := s.GetFilesByCategory(models.CategoryAudio)
	if len(audio) != 2 {
		t.Errorf("got %d audio files, want 2", len(audio))
	}
	video, _ := s.GetFilesByCategory(models.CategoryVideo)
	if len(video) != 1 {
		t.Errorf("got %d video files, want 1", len(video))
	}
}

func TestGetStats(t *testing.T) {
	s := newTestStore(t)
	s.SaveDirectory(&models.Directory{ID: "d1", URL: "http://a.com/"})
	s.SaveDirectory(&models.Directory{ID: "d2", URL: "http://b.com/"})
	s.SaveFileEntry(&models.FileEntry{ID: "d1:a.mp3", DirectoryID: "d1", Name: "a.mp3", Size: 500, Category: models.CategoryAudio, Ext: "mp3"})
	s.SaveFileEntry(&models.FileEntry{ID: "d1:b.mp3", DirectoryID: "d1", Name: "b.mp3", Size: 1500, Category: models.CategoryAudio, Ext: "mp3"})

	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalDirectories != 2 {
		t.Errorf("TotalDirectories = %d, want 2", stats.TotalDirectories)
	}
	if stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", stats.TotalFiles)
	}
	if stats.TotalSize != 2000 {
		t.Errorf("TotalSize = %d, want 2000", stats.TotalSize)
	}
	if stats.CategoryCounts[models.CategoryAudio] != 2 {
		t.Errorf("Audio count = %d, want 2", stats.CategoryCounts[models.CategoryAudio])
	}
}

func TestDeleteDirectory(t *testing.T) {
	s := newTestStore(t)
	s.SaveDirectory(&models.Directory{ID: "delme", URL: "http://del.com/"})
	s.SaveFileEntry(&models.FileEntry{ID: "delme:f.txt", DirectoryID: "delme", Name: "f.txt"})
	if err := s.DeleteDirectory("delme"); err != nil {
		t.Fatalf("DeleteDirectory: %v", err)
	}
	_, err := s.GetDirectory("delme")
	if err == nil {
		t.Error("expected error after delete")
	}
	files, _ := s.GetFilesByDir("delme")
	if len(files) != 0 {
		t.Error("files should be deleted too")
	}
}

func TestRunGC(t *testing.T) {
	s := newTestStore(t)
	err := s.RunGC()
	if err != nil && err.Error() != "Value log GC attempt didn't result in any cleanup" {
		t.Fatalf("RunGC unexpected error: %v", err)
	}
}

func TestEmptyStore(t *testing.T) {
	s := newTestStore(t)
	dirs, _ := s.ListDirectories()
	if len(dirs) != 0 {
		t.Error("expected empty dirs")
	}
	files, _ := s.GetFilesByDir("nonexistent")
	if len(files) != 0 {
		t.Error("expected empty files")
	}
}

func TestGetFilesByExt(t *testing.T) {
	s := newTestStore(t)
	s.SaveFileEntry(&models.FileEntry{ID: "d1:a.mp3", DirectoryID: "d1", Name: "a.mp3", Category: models.CategoryAudio, Ext: "mp3"})
	s.SaveFileEntry(&models.FileEntry{ID: "d1:b.mp4", DirectoryID: "d1", Name: "b.mp4", Category: models.CategoryVideo, Ext: "mp4"})
	s.SaveFileEntry(&models.FileEntry{ID: "d2:c.mp3", DirectoryID: "d2", Name: "c.mp3", Category: models.CategoryAudio, Ext: "mp3"})

	mp3s, err := s.GetFilesByExt("mp3")
	if err != nil {
		t.Fatalf("GetFilesByExt: %v", err)
	}
	if len(mp3s) != 2 {
		t.Errorf("got %d mp3 files, want 2", len(mp3s))
	}

	mp4s, _ := s.GetFilesByExt("mp4")
	if len(mp4s) != 1 {
		t.Errorf("got %d mp4 files, want 1", len(mp4s))
	}

	none, _ := s.GetFilesByExt("nonexistent")
	if len(none) != 0 {
		t.Errorf("got %d files for nonexistent ext, want 0", len(none))
	}
}

func TestGetFilesByCategoryEmpty(t *testing.T) {
	s := newTestStore(t)
	files, err := s.GetFilesByCategory(models.CategoryAudio)
	if err != nil {
		t.Fatalf("GetFilesByCategory: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}

func TestGetFilesByExtEmpty(t *testing.T) {
	s := newTestStore(t)
	files, err := s.GetFilesByExt("mp3")
	if err != nil {
		t.Fatalf("GetFilesByExt: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}
