package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
)

func newTestStoreForAPI(t *testing.T) *storage.Store {
	t.Helper()
	dir, err := os.MkdirTemp("", "odk-api-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	s, err := storage.New(dir)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newTestAPIServer(t *testing.T) *apiServer {
	t.Helper()
	store := newTestStoreForAPI(t)
	return &apiServer{store: store}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, 200, map[string]string{"key": "value"})
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["key"] != "value" {
		t.Errorf("body = %+v", body)
	}
}

func TestCORS(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

func TestCORSPreflight(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
}

func TestHandleStatsEmpty(t *testing.T) {
	srv := newTestAPIServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stats", nil)
	srv.handleStats(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var stats models.Stats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats.TotalDirectories != 0 || stats.TotalFiles != 0 {
		t.Errorf("stats = %+v", stats)
	}
}

func TestHandleStatsMethodNotAllowed(t *testing.T) {
	srv := newTestAPIServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/stats", nil)
	srv.handleStats(w, r)
	if w.Code != 405 {
		t.Errorf("code = %d, want 405", w.Code)
	}
}

func TestHandleStatsWithData(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.store.SaveDirectory(&models.Directory{ID: "d1", URL: "http://a.com/", Status: models.StatusDone, FileCount: 3, TotalSize: 1000})
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:f1.mp3", DirectoryID: "d1", Name: "f1.mp3", Size: 500, Category: models.CategoryAudio, Ext: "mp3"})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stats", nil)
	srv.handleStats(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var stats models.Stats
	json.NewDecoder(w.Body).Decode(&stats)
	if stats.TotalDirectories != 1 || stats.TotalFiles != 1 || stats.TotalSize != 500 {
		t.Errorf("stats = %+v", stats)
	}
}

func TestHandleDirsEmpty(t *testing.T) {
	srv := newTestAPIServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/dirs", nil)
	srv.handleDirs(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var dirs []*models.Directory
	if err := json.NewDecoder(w.Body).Decode(&dirs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(dirs) != 0 {
		t.Errorf("got %d dirs, want 0", len(dirs))
	}
}

func TestHandleDirsWithData(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.store.SaveDirectory(&models.Directory{ID: "d1", URL: "http://a.com/", Status: models.StatusPending})
	srv.store.SaveDirectory(&models.Directory{ID: "d2", URL: "http://b.com/", Status: models.StatusDone})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/dirs", nil)
	srv.handleDirs(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var dirs []*models.Directory
	json.NewDecoder(w.Body).Decode(&dirs)
	if len(dirs) != 2 {
		t.Errorf("got %d dirs, want 2", len(dirs))
	}
}

func TestHandleDirsMethodNotAllowed(t *testing.T) {
	srv := newTestAPIServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/dirs", nil)
	srv.handleDirs(w, r)
	if w.Code != 405 {
		t.Errorf("code = %d, want 405", w.Code)
	}
}

func TestHandleDirsByID(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.store.SaveDirectory(&models.Directory{ID: "testid", URL: "http://test.com/", Status: models.StatusDone})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/dirs/testid", nil)
	srv.handleDirs(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var dir models.Directory
	json.NewDecoder(w.Body).Decode(&dir)
	if dir.ID != "testid" || dir.URL != "http://test.com/" {
		t.Errorf("dir = %+v", dir)
	}
}

func TestHandleDirsByIDNotFound(t *testing.T) {
	srv := newTestAPIServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/dirs/nonexistent", nil)
	srv.handleDirs(w, r)
	if w.Code != 404 {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestHandleDirsFiles(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.store.SaveDirectory(&models.Directory{ID: "d1", URL: "http://a.com/"})
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:f1.mp3", DirectoryID: "d1", Name: "f1.mp3", Category: models.CategoryAudio, Ext: "mp3"})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/dirs/d1/files", nil)
	srv.handleDirs(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var files []*models.FileEntry
	json.NewDecoder(w.Body).Decode(&files)
	if len(files) != 1 {
		t.Errorf("got %d files, want 1", len(files))
	}
}

func TestHandleDirsFilesEmpty(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.store.SaveDirectory(&models.Directory{ID: "d1", URL: "http://a.com/"})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/dirs/d1/files", nil)
	srv.handleDirs(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var files []*models.FileEntry
	json.NewDecoder(w.Body).Decode(&files)
	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}

func TestHandleDirsPaginationParams(t *testing.T) {
	srv := newTestAPIServer(t)
	for i := 0; i < 5; i++ {
		srv.store.SaveDirectory(&models.Directory{ID: fmt.Sprintf("d%d", i), URL: fmt.Sprintf("http://dir%d.com/", i)})
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/dirs?limit=2&offset=1", nil)
	srv.handleDirs(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var dirs []*models.Directory
	json.NewDecoder(w.Body).Decode(&dirs)
	if len(dirs) != 2 {
		t.Errorf("got %d dirs, want 2", len(dirs))
	}
}

func TestHandleFilesByCat(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:a.mp3", DirectoryID: "d1", Name: "a.mp3", Category: models.CategoryAudio, Ext: "mp3"})
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:b.mp4", DirectoryID: "d1", Name: "b.mp4", Category: models.CategoryVideo, Ext: "mp4"})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/files?cat=audio", nil)
	srv.handleFiles(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var files []*models.FileEntry
	json.NewDecoder(w.Body).Decode(&files)
	if len(files) != 1 {
		t.Errorf("got %d files, want 1", len(files))
	}
}

func TestHandleFilesByExt(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:a.mp3", DirectoryID: "d1", Name: "a.mp3", Category: models.CategoryAudio, Ext: "mp3"})
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:b.mp3", DirectoryID: "d1", Name: "b.mp3", Category: models.CategoryAudio, Ext: "mp3"})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/files?ext=mp3", nil)
	srv.handleFiles(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var files []*models.FileEntry
	json.NewDecoder(w.Body).Decode(&files)
	if len(files) != 2 {
		t.Errorf("got %d files, want 2", len(files))
	}
}

func TestHandleFilesNoParams(t *testing.T) {
	srv := newTestAPIServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/files", nil)
	srv.handleFiles(w, r)
	if w.Code != 400 {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleFilesMethodNotAllowed(t *testing.T) {
	srv := newTestAPIServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/files", nil)
	srv.handleFiles(w, r)
	if w.Code != 405 {
		t.Errorf("code = %d, want 405", w.Code)
	}
}

func TestHandleSearch(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.store.SaveDirectory(&models.Directory{ID: "d1", URL: "http://a.com/"})
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:test.mp3", DirectoryID: "d1", Name: "test.mp3", Category: models.CategoryAudio, Ext: "mp3"})
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:other.txt", DirectoryID: "d1", Name: "other.txt", Category: models.CategoryDocument, Ext: "txt"})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/search?q=test", nil)
	srv.handleSearch(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var results []struct {
		File  *models.FileEntry `json:"file"`
		DirID string            `json:"directory_id"`
	}
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestHandleSearchWithCategory(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.store.SaveDirectory(&models.Directory{ID: "d1", URL: "http://a.com/"})
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:test.mp3", DirectoryID: "d1", Name: "test.mp3", Category: models.CategoryAudio, Ext: "mp3"})
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:test.txt", DirectoryID: "d1", Name: "test.txt", Category: models.CategoryDocument, Ext: "txt"})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/search?q=test&cat=audio", nil)
	srv.handleSearch(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var results []struct {
		File  *models.FileEntry `json:"file"`
		DirID string            `json:"directory_id"`
	}
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestHandleSearchNoQuery(t *testing.T) {
	srv := newTestAPIServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/search", nil)
	srv.handleSearch(w, r)
	if w.Code != 400 {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleSearchMethodNotAllowed(t *testing.T) {
	srv := newTestAPIServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/search?q=test", nil)
	srv.handleSearch(w, r)
	if w.Code != 405 {
		t.Errorf("code = %d, want 405", w.Code)
	}
}

func TestHandleSearchCaseInsensitive(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.store.SaveDirectory(&models.Directory{ID: "d1", URL: "http://a.com/"})
	srv.store.SaveFileEntry(&models.FileEntry{ID: "d1:TestSong.MP3", DirectoryID: "d1", Name: "TestSong.MP3", Category: models.CategoryAudio, Ext: "mp3"})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/search?q=test", nil)
	srv.handleSearch(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var results []struct {
		File  *models.FileEntry `json:"file"`
		DirID string            `json:"directory_id"`
	}
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 1 {
		t.Errorf("got %d results, want 1 for case-insensitive search", len(results))
	}
}

func TestHandleFilesLimit(t *testing.T) {
	srv := newTestAPIServer(t)
	for i := 0; i < 10; i++ {
		srv.store.SaveFileEntry(&models.FileEntry{
			ID:          fmt.Sprintf("d1:f%d.mp3", i),
			DirectoryID: "d1",
			Name:        fmt.Sprintf("f%d.mp3", i),
			Category:    models.CategoryAudio,
			Ext:         "mp3",
		})
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/files?cat=audio&limit=3", nil)
	srv.handleFiles(w, r)
	if w.Code != 200 {
		t.Errorf("code = %d, want 200", w.Code)
	}
	var files []*models.FileEntry
	json.NewDecoder(w.Body).Decode(&files)
	if len(files) != 3 {
		t.Errorf("got %d files, want 3", len(files))
	}
}

func TestHandleNotFound(t *testing.T) {
	srv := newTestAPIServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/nonexistent", nil)
	srv.handleNotFound(w, r)
	if w.Code != 404 {
		t.Errorf("code = %d, want 404", w.Code)
	}
}
