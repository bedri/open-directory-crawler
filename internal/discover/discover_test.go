package discover

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bedri/open-directory-crawler/internal/models"
)

func newTestFinder(t *testing.T, timeout time.Duration) *Finder {
	t.Helper()
	f := New()
	f.client = &http.Client{Timeout: timeout}
	return f
}

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

func TestSearxngResponseJSON(t *testing.T) {
	raw := `{"results":[{"url":"http://example.com/","title":"Test","content":"desc"}]}`
	var resp searxngResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(resp.Results))
	}
	if resp.Results[0].URL != "http://example.com/" {
		t.Errorf("URL = %q", resp.Results[0].URL)
	}
	if resp.Results[0].Title != "Test" {
		t.Errorf("Title = %q", resp.Results[0].Title)
	}
	if resp.Results[0].Content != "desc" {
		t.Errorf("Content = %q", resp.Results[0].Content)
	}
}

func TestSearxngResponseJSONEmpty(t *testing.T) {
	raw := `{"results":[]}`
	var resp searxngResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("got %d results, want 0", len(resp.Results))
	}
}

func TestGoogleAPIResponseJSON(t *testing.T) {
	raw := `{"items":[{"link":"http://a.com/","title":"A"},{"link":"http://b.com/","title":"B"}]}`
	var resp googleAPIResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(resp.Items))
	}
	if resp.Items[0].Link != "http://a.com/" {
		t.Errorf("Link[0] = %q", resp.Items[0].Link)
	}
}

func TestBingAPIResponseJSON(t *testing.T) {
	raw := `{"webPages":{"value":[{"url":"http://x.com/","name":"X"},{"url":"http://y.com/","name":"Y"}]}}`
	var resp bingAPIResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.WebPages.Value) != 2 {
		t.Fatalf("got %d values, want 2", len(resp.WebPages.Value))
	}
	if resp.WebPages.Value[0].URL != "http://x.com/" {
		t.Errorf("URL[0] = %q", resp.WebPages.Value[0].URL)
	}
}

func TestShodanResponseJSON(t *testing.T) {
	raw := `{"matches":[{"ip_str":"1.2.3.4","port":80},{"ip_str":"5.6.7.8","port":8080}]}`
	var resp shodanResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(resp.Matches))
	}
	if resp.Matches[0].IPStr != "1.2.3.4" {
		t.Errorf("IPStr[0] = %q", resp.Matches[0].IPStr)
	}
}

func TestShodanResponseJSONEmpty(t *testing.T) {
	raw := `{"matches":[]}`
	var resp shodanResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Matches) != 0 {
		t.Errorf("got %d matches, want 0", len(resp.Matches))
	}
}

func TestDiscoverByTypeError(t *testing.T) {
	f := New()
	_, err := f.DiscoverByType("unsupported")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Errorf("error = %q, want 'unsupported type'", err.Error())
	}
}

func TestIsOpenDirectoryIndicators(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><h1>Index of /files</h1><pre>Parent Directory</pre></body></html>`))
	}))
	defer ts.Close()

	f := newTestFinder(t, 5*time.Second)
	if !f.IsOpenDirectory(ts.URL) {
		t.Error("expected true for directory listing page")
	}
}

func TestIsOpenDirectoryNormalPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><h1>Welcome to my site</h1><p>Some content here</p></body></html>`))
	}))
	defer ts.Close()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	if f.IsOpenDirectory(ts.URL) {
		t.Error("expected false for normal page")
	}
}

func TestIsOpenDirectoryServerError(t *testing.T) {
	f := &Finder{
		client: &http.Client{Timeout: time.Millisecond},
	}
	if f.IsOpenDirectory("http://localhost:19999/") {
		t.Error("expected false for connection error")
	}
}

func TestIsOpenDirectoryInvalidURL(t *testing.T) {
	f := &Finder{}
	if f.IsOpenDirectory("://invalid") {
		t.Error("expected false for invalid URL")
	}
}

func TestSearchSearXNG(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.String(), "format=json") {
			t.Error("request missing format=json")
		}
		json.NewEncoder(w).Encode(searxngResponse{
			Results: []struct {
				URL     string `json:"url"`
				Title   string `json:"title"`
				Content string `json:"content"`
			}{
				{URL: "http://example.com/files/", Title: "Index of files"},
			},
		})
	}))
	defer ts.Close()

	f := &Finder{
		client:     &http.Client{Timeout: 5 * time.Second},
		searxngURL: ts.URL,
	}
	results, err := f.SearchSearXNG()
	if err != nil {
		t.Fatalf("SearchSearXNG: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].Source != "searxng" {
		t.Errorf("Source = %q, want %q", results[0].Source, "searxng")
	}
	if results[0].URL != "http://example.com/files/" {
		t.Errorf("URL = %q", results[0].URL)
	}
}

func TestSearchSearXNGServerError(t *testing.T) {
	f := &Finder{
		client:     &http.Client{Timeout: time.Millisecond},
		searxngURL: "http://localhost:19999/",
	}
	results, err := f.SearchSearXNG()
	if err != nil {
		t.Fatalf("SearchSearXNG should not fail: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on error, got %d", len(results))
	}
}

func TestSearchSearXNGInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	f := &Finder{
		client:     &http.Client{Timeout: 5 * time.Second},
		searxngURL: ts.URL,
	}
	results, err := f.SearchSearXNG()
	if err != nil {
		t.Fatalf("SearchSearXNG should not fail: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for invalid JSON, got %d", len(results))
	}
}

func TestSearchShodanEmptyKey(t *testing.T) {
	f := &Finder{shodanKey: ""}
	results, err := f.SearchShodan()
	if err != nil {
		t.Fatalf("SearchShodan: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty key, got %d", len(results))
	}
}



func TestScrapeAggregators(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><a href="http://example.com/files/">Example</a></html>`))
	}))
	defer ts.Close()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	results, err := f.ScrapeAggregators()
	if err != nil {
		t.Fatalf("ScrapeAggregators: %v", err)
	}
	for _, r := range results {
		if strings.HasPrefix(r.URL, "http") {
			return
		}
	}
}

func TestSearchCommonPaths(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if strings.Contains(r.URL.Path, "movies") || strings.Contains(r.URL.Path, "music") {
			w.Write([]byte(`<html><h1>Index of</h1><pre>Parent Directory</pre></html>`))
			return
		}
		w.Write([]byte(`<html>normal page</html>`))
	}))
	defer ts.Close()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	found := f.SearchCommonPaths(ts.URL)
	if len(found) == 0 {
		t.Error("expected at least one open directory path")
	}
}

func TestQuickProfile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Index of /files</title></head>
<body><table>
<tr><td><a href="a.mp3">a.mp3</a></td><td>2024-01-01</td><td>1M</td></tr>
<tr><td><a href="b.mp4">b.mp4</a></td><td>2024-01-01</td><td>2M</td></tr>
<tr><td><a href="c.pdf">c.pdf</a></td><td>2024-01-01</td><td>500K</td></tr>
</table></body></html>`))
	}))
	defer ts.Close()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	counts, err := f.QuickProfile(ts.URL)
	if err != nil {
		t.Fatalf("QuickProfile: %v", err)
	}
	if counts[models.CategoryAudio] != 1 {
		t.Errorf("Audio count = %d, want 1", counts[models.CategoryAudio])
	}
	if counts[models.CategoryVideo] != 1 {
		t.Errorf("Video count = %d, want 1", counts[models.CategoryVideo])
	}
	if counts[models.CategoryDocument] != 1 {
		t.Errorf("Document count = %d, want 1", counts[models.CategoryDocument])
	}
}

func TestQuickProfileServerError(t *testing.T) {
	f := &Finder{
		client: &http.Client{Timeout: time.Millisecond},
	}
	_, err := f.QuickProfile("http://localhost:19999/")
	if err == nil {
		t.Error("expected error for connection failure")
	}
}

func TestQuickProfileInvalidURL(t *testing.T) {
	f := &Finder{}
	_, err := f.QuickProfile("://invalid")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestQuickProfileEmptyBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(``))
	}))
	defer ts.Close()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	counts, err := f.QuickProfile(ts.URL)
	if err != nil {
		t.Fatalf("QuickProfile: %v", err)
	}
	if len(counts) != 0 {
		t.Errorf("expected empty counts, got %d", len(counts))
	}
}

func TestCheckDensity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><table>
<tr><td><a href="a.mp3">a.mp3</a></td><td>2024-01-01</td><td>1M</td></tr>
<tr><td><a href="b.mp3">b.mp3</a></td><td>2024-01-01</td><td>2M</td></tr>
<tr><td><a href="c.mp3">c.mp3</a></td><td>2024-01-01</td><td>3M</td></tr>
<tr><td><a href="d.pdf">d.pdf</a></td><td>2024-01-01</td><td>500K</td></tr>
</table></body></html>`))
	}))
	defer ts.Close()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	if !f.CheckDensity(ts.URL, models.CategoryAudio, 0.5) {
		t.Error("expected true for 75% audio density")
	}
	if f.CheckDensity(ts.URL, models.CategoryVideo, 0.5) {
		t.Error("expected false for 0% video density")
	}
}

func TestCheckDensityLowThreshold(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><table>
<tr><td><a href="a.mp3">a.mp3</a></td><td>2024-01-01</td><td>1M</td></tr>
<tr><td><a href="b.pdf">b.pdf</a></td><td>2024-01-01</td><td>2M</td></tr>
</table></body></html>`))
	}))
	defer ts.Close()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	if !f.CheckDensity(ts.URL, models.CategoryAudio, 0.1) {
		t.Error("expected true for 50% audio at 0.1 threshold")
	}
}

func TestCheckDensityServerError(t *testing.T) {
	f := &Finder{
		client: &http.Client{Timeout: time.Millisecond},
	}
	if f.CheckDensity("http://localhost:19999/", models.CategoryAudio, 0.5) {
		t.Error("expected false for connection error")
	}
}

func TestParseGoogleResultsURLEncoded(t *testing.T) {
	body := `<a href="/url?q=http://example.com/path+with+spaces/&sa=U"></a>`
	results := parseGoogleResults(body, "test")
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].URL != "http://example.com/path with spaces/" {
		t.Errorf("URL = %q", results[0].URL)
	}
}

func TestParseGoogleResultsMalformed(t *testing.T) {
	body := `<a href="/url?q=not%valid&sa=U"></a>`
	results := parseGoogleResults(body, "test")
	if len(results) != 0 {
		t.Errorf("expected 0 results for malformed URL, got %d", len(results))
	}
}

func TestParseGoogleResultsNoHref(t *testing.T) {
	body := `<a href="/other?param=value"></a>`
	results := parseGoogleResults(body, "test")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestNewSearxngDefault(t *testing.T) {
	before := os.Getenv("ODK_SEARXNG_URL")
	os.Unsetenv("ODK_SEARXNG_URL")
	defer os.Setenv("ODK_SEARXNG_URL", before)

	f := New()
	if f.searxngURL != "https://searx.be" {
		t.Errorf("default searxngURL = %q, want https://searx.be", f.searxngURL)
	}
}

func TestNewCustomSearxng(t *testing.T) {
	before := os.Getenv("ODK_SEARXNG_URL")
	os.Setenv("ODK_SEARXNG_URL", "https://custom.searx/")
	defer os.Setenv("ODK_SEARXNG_URL", before)

	f := New()
	if f.searxngURL != "https://custom.searx/" {
		t.Errorf("custom searxngURL = %q, want https://custom.searx/", f.searxngURL)
	}
}



func TestSearchDuckDuckGoTimeout(t *testing.T) {
	f := &Finder{
		client: &http.Client{Timeout: time.Millisecond},
	}
	results, err := f.SearchDuckDuckGo()
	if err != nil {
		t.Fatalf("SearchDuckDuckGo: %v", err)
	}
	_ = results
}

func TestScrapeAggregatorsEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>no urls here</html>`))
	}))
	defer ts.Close()

	oldAgg := aggregators
	aggregators = []aggregator{{name: "test", url: ts.URL}}
	defer func() { aggregators = oldAgg }()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	results, err := f.ScrapeAggregators()
	if err != nil {
		t.Fatalf("ScrapeAggregators: %v", err)
	}
	_ = results
}

func TestResultUnique(t *testing.T) {
	results := []Result{
		{URL: "http://example.com/", Source: "a"},
		{URL: "http://example.com/", Source: "b"},
		{URL: "http://example.com/files/", Source: "c"},
	}
	dedup := func(rs []Result) []Result {
		seen := map[string]bool{}
		var out []Result
		for _, r := range rs {
			u := strings.TrimRight(r.URL, "/")
			if !seen[u] {
				seen[u] = true
				out = append(out, r)
			}
		}
		return out
	}
	deduped := dedup(results)
	if len(deduped) != 2 {
		t.Errorf("expected 2 unique, got %d", len(deduped))
	}
}

func TestSearchCommonPathsInvalidBase(t *testing.T) {
	f := &Finder{}
	found := f.SearchCommonPaths("://invalid")
	if len(found) != 0 {
		t.Errorf("expected 0 paths for invalid URL, got %d", len(found))
	}
}

func TestSearchCommonPathsAllNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>normal page</html>`))
	}))
	defer ts.Close()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	found := f.SearchCommonPaths(ts.URL)
	if len(found) != 0 {
		t.Errorf("expected 0 paths found, got %d", len(found))
	}
}

func TestQuickProfileWithDirs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><table>
<tr><td><a href="subdir/">subdir/</a></td><td>2024-01-01</td><td>-</td></tr>
<tr><td><a href="file.txt">file.txt</a></td><td>2024-01-01</td><td>1K</td></tr>
</table></body></html>`))
	}))
	defer ts.Close()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	counts, err := f.QuickProfile(ts.URL)
	if err != nil {
		t.Fatalf("QuickProfile: %v", err)
	}
	if len(counts) != 1 {
		t.Errorf("expected 1 category (skip dirs), got %d", len(counts))
	}
}

func TestCheckDensityEmptyDir(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><table></table></body></html>`))
	}))
	defer ts.Close()

	f := &Finder{
		client: &http.Client{Timeout: 5 * time.Second},
	}
	if f.CheckDensity(ts.URL, models.CategoryAudio, 0.5) {
		t.Error("expected false for empty directory")
	}
}

func TestParseGoogleResultsFullRoundTrip(t *testing.T) {
	body := `<html><body>
<a href="/url?q=http://site1.com/files/&sa=U&ved=1"></a>
<a href="/url?q=http://site2.com/data/&sa=U&ved=2"></a>
<a href="/url?q=http://site3.com/&sa=U&ved=3"></a>
</body></html>`
	results := parseGoogleResults(body, "test")
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	expected := []string{"http://site1.com/files/", "http://site2.com/data/", "http://site3.com/"}
	for i, exp := range expected {
		if results[i].URL != exp {
			t.Errorf("results[%d].URL = %q, want %q", i, results[i].URL, exp)
		}
	}
}

func TestSaveResultsError(t *testing.T) {
	f := New()
	err := f.SaveResults(nil, "/nonexistent/path/out.json")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestSearxngResponseJSONMissingFields(t *testing.T) {
	raw := `{"results":[{"url":"http://example.com/"}]}`
	var resp searxngResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(resp.Results))
	}
	if resp.Results[0].URL != "http://example.com/" {
		t.Errorf("URL = %q", resp.Results[0].URL)
	}
}

func TestGoogleAPIResponseJSONEmpty(t *testing.T) {
	raw := `{}`
	var resp googleAPIResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Errorf("got %d items, want 0", len(resp.Items))
	}
}

func TestBingAPIResponseJSONEmpty(t *testing.T) {
	raw := `{}`
	var resp bingAPIResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.WebPages.Value) != 0 {
		t.Errorf("got %d values, want 0", len(resp.WebPages.Value))
	}
}

func TestAggregatorsList(t *testing.T) {
	if len(aggregators) == 0 {
		t.Error("aggregators should not be empty")
	}
	for i, a := range aggregators {
		if a.name == "" {
			t.Errorf("aggregators[%d].name is empty", i)
		}
		if a.url == "" {
			t.Errorf("aggregators[%d].url is empty", i)
		}
	}
}

func TestTypeDorksNotEmpty(t *testing.T) {
	for typ, dorks := range TypeDorks {
		if len(dorks) == 0 {
			t.Errorf("TypeDorks[%q] is empty", typ)
		}
	}
}
