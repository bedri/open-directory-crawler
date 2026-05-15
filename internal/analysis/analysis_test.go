package analysis

import (
	"os"
	"testing"

	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dir, err := os.MkdirTemp("", "odk-analysis-test-*")
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

func TestExtractKeywords(t *testing.T) {
	freq := make(map[string]int)
	extractKeywords("hello-world_test.mp3", freq)
	if freq["hello"] != 1 {
		t.Errorf("missing 'hello'")
	}
	if freq["world"] != 1 {
		t.Errorf("missing 'world'")
	}
	if freq["test"] != 1 {
		t.Errorf("missing 'test'")
	}
	if freq["mp3"] != 0 {
		t.Errorf("'mp3' should be in stopwords")
	}
}

func TestExtractKeywordsNumeric(t *testing.T) {
	freq := make(map[string]int)
	extractKeywords("file_12345_v2.mp3", freq)
	if freq["file"] != 1 {
		t.Errorf("missing 'file'")
	}
	if freq["12345"] != 0 {
		t.Errorf("numeric only '12345' should be filtered")
	}
}

func TestExtractKeywordsShort(t *testing.T) {
	freq := make(map[string]int)
	extractKeywords("ab x.pdf", freq)
	if freq["ab"] != 0 {
		t.Errorf("short 'ab' should be filtered")
	}
}

func TestExtractKeywordsStopWords(t *testing.T) {
	freq := make(map[string]int)
	extractKeywords("the_and_for_test.pdf", freq)
	if freq["the"] != 0 {
		t.Errorf("stopword 'the' should be filtered")
	}
	if freq["test"] != 1 {
		t.Errorf("'test' should be present")
	}
}

func TestExtractKeywordsLong(t *testing.T) {
	freq := make(map[string]int)
	long := make([]byte, 35)
	for i := range long {
		long[i] = 'a'
	}
	extractKeywords("x "+string(long)+" y.pdf", freq)
	if freq[string(long)] != 0 {
		t.Errorf("long >30 chars should be filtered")
	}
}

func TestExtractTLD(t *testing.T) {
	tests := []struct {
		url string
		tld string
	}{
		{"http://example.com/file.pdf", "com"},
		{"https://example.org/file", "org"},
		{"ftp://files.net/pub/", "net"},
		{"http://edu.example.edu/pdf/", "edu"},
		{"http://example.com:8080/path", "com"},
		{"http://192.168.1.1/file", "1"},
		{"invalid", "unknown"},
	}
	for _, tc := range tests {
		got := extractTLD(tc.url)
		if got != tc.tld {
			t.Errorf("extractTLD(%q) = %q, want %q", tc.url, got, tc.tld)
		}
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		url    string
		domain string
	}{
		{"http://example.com/file.pdf", "example.com"},
		{"https://sub.example.org:8080/path", "sub.example.org"},
		{"ftp://files.net/", "files.net"},
		{"invalid", "unknown"},
	}
	for _, tc := range tests {
		got := extractDomain(tc.url)
		if got != tc.domain {
			t.Errorf("extractDomain(%q) = %q, want %q", tc.url, got, tc.domain)
		}
	}
}

func TestSizeBucket(t *testing.T) {
	tests := []struct {
		size int64
		b    int
	}{
		{0, 0},
		{1023, 0},
		{1024, 1},
		{10 * 1024, 2},
		{100 * 1024, 3},
		{1024 * 1024, 4},
		{10 * 1024 * 1024, 5},
		{100 * 1024 * 1024, 6},
		{1 << 60, 6},
	}
	for _, tc := range tests {
		got := sizeBucket(tc.size)
		if got != tc.b {
			t.Errorf("sizeBucket(%d) = %d, want %d", tc.size, got, tc.b)
		}
	}
}

func TestIsEduDomain(t *testing.T) {
	tests := []struct {
		domain string
		tld    string
		edu    bool
	}{
		{"example.edu", "edu", true},
		{"example.ac.uk", "uk", true},
		{"example.ac.jp", "jp", true},
		{"example.ac.kr", "kr", true},
		{"example.com", "com", false},
		{"example.org", "org", false},
		{"example.ac", "ac", true},
	}
	for _, tc := range tests {
		got := isEduDomain(tc.domain, tc.tld)
		if got != tc.edu {
			t.Errorf("isEduDomain(%q, %q) = %v, want %v", tc.domain, tc.tld, got, tc.edu)
		}
	}
}

func TestBuildWordlist(t *testing.T) {
	report := &models.AnalysisReport{
		Keywords: []models.KeywordEntry{
			{Word: "hello", Count: 5},
			{Word: "world", Count: 3},
			{Word: "test", Count: 1},
		},
	}
	data := BuildWordlist(report)
	expected := "hello\nworld\ntest\n"
	if string(data) != expected {
		t.Errorf("BuildWordlist = %q, want %q", string(data), expected)
	}
}

func TestAnalyzerRunEmpty(t *testing.T) {
	s := newTestStore(t)
	a := New(s)
	report, err := a.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.TotalDirs != 0 || report.TotalFiles != 0 {
		t.Errorf("expected empty report, got %+v", report)
	}
}

func TestAnalyzerRunWithData(t *testing.T) {
	s := newTestStore(t)

	s.SaveDirectory(&models.Directory{
		ID: "d1", URL: "http://example.edu/", Status: models.StatusDone, Depth: 1,
	})
	s.SaveDirectory(&models.Directory{
		ID: "d2", URL: "http://example.com/", Status: models.StatusDone, Depth: 2,
	})

	s.SaveFileEntry(&models.FileEntry{
		ID: "d1:hello_world.mp3", DirectoryID: "d1", Name: "hello_world.mp3",
		URL: "http://example.edu/hello_world.mp3", Size: 2097152,
		Category: models.CategoryAudio, Ext: "mp3",
	})
	s.SaveFileEntry(&models.FileEntry{
		ID: "d1:doc.pdf", DirectoryID: "d1", Name: "doc.pdf",
		URL: "http://example.edu/doc.pdf", Size: 1048576,
		Category: models.CategoryDocument, Ext: "pdf",
	})
	s.SaveFileEntry(&models.FileEntry{
		ID: "d2:photo.jpg", DirectoryID: "d2", Name: "photo.jpg",
		URL: "http://example.com/photo.jpg", Size: 524288,
		Category: models.CategoryImage, Ext: "jpg",
	})

	a := New(s)
	report, err := a.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.TotalDirs != 2 {
		t.Errorf("TotalDirs = %d, want 2", report.TotalDirs)
	}
	if report.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", report.TotalFiles)
	}
	if report.TotalSize != 3670016 {
		t.Errorf("TotalSize = %d, want 3670016", report.TotalSize)
	}

	if report.TLDStats["edu"] == nil || report.TLDStats["edu"].Files != 2 {
		t.Errorf("edu TLD should have 2 files")
	}
	if report.TLDStats["com"] == nil || report.TLDStats["com"].Files != 1 {
		t.Errorf("com TLD should have 1 file")
	}

	if report.EduBreakdown == nil || report.EduBreakdown.TotalFiles != 2 {
		t.Errorf("edu breakdown should have 2 files")
	}

	if len(report.Keywords) == 0 {
		t.Errorf("expected keywords")
	}
	found := false
	for _, kw := range report.Keywords {
		if kw.Word == "hello" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'hello' in keywords")
	}

	if report.CatExtMatrix["audio:mp3"] != 1 {
		t.Errorf("expected audio:mp3=1, got %d", report.CatExtMatrix["audio:mp3"])
	}
	if report.CatExtMatrix["document:pdf"] != 1 {
		t.Errorf("expected document:pdf=1")
	}

	if report.SizeBuckets["1-10MB"] != 2 {
		t.Errorf("expected 2 files in 1-10MB bucket, got %d", report.SizeBuckets["1-10MB"])
	}
	if report.SizeBuckets["100KB-1MB"] != 1 {
		t.Errorf("expected 1 file in 100KB-1MB bucket")
	}

	if report.ExtCatMatrix["mp3"] == nil || report.ExtCatMatrix["mp3"][models.CategoryAudio] != 1 {
		t.Errorf("expected mp3→audio=1")
	}
}

func TestAnalyzerTLDWithPort(t *testing.T) {
	s := newTestStore(t)
	s.SaveDirectory(&models.Directory{ID: "d1", URL: "http://example.com:8080/"})
	s.SaveFileEntry(&models.FileEntry{
		ID: "d1:f.txt", DirectoryID: "d1", Name: "f.txt",
		URL: "http://example.com:8080/f.txt", Category: models.CategoryDocument,
	})

	a := New(s)
	report, err := a.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.TLDStats["com"] == nil || report.TLDStats["com"].Files != 1 {
		t.Errorf("expected com TLD, got %+v", report.TLDStats)
	}
}
