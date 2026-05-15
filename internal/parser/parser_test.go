package parser

import (
	"net/url"
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"1024", 1024},
		{"1K", 1024},
		{"2M", 2 * 1024 * 1024},
		{"1G", 1024 * 1024 * 1024},
		{"1T", 1024 * 1024 * 1024 * 1024},
		{"1KB", 1024},
		{"2MB", 2 * 1024 * 1024},
		{"1.5K", 1536},
		{"-", 0},
		{"", 0},
		{"abc", 0},
		{" 1M ", 1024 * 1024},
	}
	for _, tt := range tests {
		got := parseSize(tt.input)
		if got != tt.want {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestResolveURL(t *testing.T) {
	base, _ := url.Parse("http://example.com/files/")
	tests := []struct {
		href string
		want string
	}{
		{"subdir/", "http://example.com/files/subdir/"},
		{"file.txt", "http://example.com/files/file.txt"},
		{"/absolute/file.txt", "http://example.com/absolute/file.txt"},
		{"http://other.com/", "http://other.com/"},
		{"https://other.com/s", "https://other.com/s"},
	}
	for _, tt := range tests {
		got := resolveURL(base, tt.href)
		if got != tt.want {
			t.Errorf("resolveURL(%q) = %q, want %q", tt.href, got, tt.want)
		}
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{"<html><head><title>Index of /files</title></head></html>", "files"},
		{"<html><head><title>Test Site</title></head></html>", "Test Site"},
		{"<html><body>no title</body></html>", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractTitle(tt.body)
		if got != tt.want {
			t.Errorf("extractTitle(%q) = %q, want %q", tt.body[:min(len(tt.body), 50)], got, tt.want)
		}
	}
}

func TestParseApacheStyle(t *testing.T) {
	body := `<html>
<body>
<table>
<tr><td><a href="file1.mp3">file1.mp3</a></td><td>2024-01-01</td><td>1M</td></tr>
<tr><td><a href="subdir/">subdir/</a></td><td>2024-01-01</td><td>-</td></tr>
<tr><td><a href="../">Parent Directory</a></td><td></td><td></td></tr>
</table>
</body>
</html>`
	links := parseApacheStyle("http://example.com/files/", body)
	if links == nil {
		t.Fatal("parseApacheStyle returned nil")
	}
	if len(links) != 2 {
		t.Fatalf("got %d links, want 2", len(links))
	}
	if links[0].Name != "file1.mp3" || links[0].IsDir {
		t.Errorf("link0 = %+v, want file", links[0])
	}
	if links[1].Name != "subdir" || !links[1].IsDir {
		t.Errorf("link1 = %+v, want dir", links[1])
	}
}

func TestParseNginxStyle(t *testing.T) {
	body := `<html>
<body>
<pre><a href="../">../</a>
<a href="file1.mp3">file1.mp3</a>
<a href="subdir/">subdir/</a>
</pre>
</body>
</html>`
	links := parseNginxStyle("http://example.com/files/", body)
	if links == nil {
		t.Fatal("parseNginxStyle returned nil")
	}
	if len(links) != 2 {
		t.Fatalf("got %d links, want 2", len(links))
	}
	if links[0].Name != "file1.mp3" || links[0].IsDir {
		t.Errorf("link0 = %+v", links[0])
	}
	if links[1].Name != "subdir" || !links[1].IsDir {
		t.Errorf("link1 = %+v", links[1])
	}
}

func TestParseGenericLinks(t *testing.T) {
	body := `<html>
<body>
<a href="file1.mp3">file1.mp3</a>
<a href="subdir/">subdir/</a>
<a href="../">../</a>
<a href="#anchor">anchor</a>
<a href="mailto:test@test.com">mail</a>
</body>
</html>`
	links := parseGenericLinks("http://example.com/files/", body)
	if len(links) != 2 {
		t.Fatalf("got %d links, want 2", len(links))
	}
}

func TestParseDirectoryListing(t *testing.T) {
	body := `<html><head><title>Index of /test</title></head>
<body>
<table>
<tr><td><a href="a.mp3">a.mp3</a></td><td>2024-01-01</td><td>1M</td></tr>
<tr><td><a href="b.pdf">b.pdf</a></td><td>2024-01-01</td><td>2M</td></tr>
</table>
</body>
</html>`
	title, links, err := ParseDirectoryListing("http://example.com/test/", body)
	if err != nil {
		t.Fatalf("ParseDirectoryListing: %v", err)
	}
	if title != "test" {
		t.Errorf("title = %q, want %q", title, "test")
	}
	if len(links) != 2 {
		t.Fatalf("got %d links, want 2", len(links))
	}
}

func TestParseDirectoryListingNoTable(t *testing.T) {
	body := `<html><pre><a href="f.txt">f.txt</a></pre></html>`
	_, links, err := ParseDirectoryListing("http://example.com/", body)
	if err != nil {
		t.Fatalf("ParseDirectoryListing: %v", err)
	}
	if len(links) != 1 {
		t.Errorf("got %d links, want 1", len(links))
	}
}

func TestParseDirectoryListingEmpty(t *testing.T) {
	_, links, err := ParseDirectoryListing("http://example.com/", "")
	if err != nil {
		t.Fatalf("ParseDirectoryListing: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("got %d links, want 0", len(links))
	}
}

func TestFileLinksToEntries(t *testing.T) {
	links := []FileLink{
		{Name: "a.mp3", URL: "http://example.com/a.mp3", Size: 1024, IsDir: false},
		{Name: "b.mp4", URL: "http://example.com/b.mp4", Size: 2048, IsDir: false},
		{Name: "subdir", URL: "http://example.com/subdir/", IsDir: true},
	}
	entries := FileLinksToEntries(links, "dir123", "http://example.com/")
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (dirs should be filtered)", len(entries))
	}
	if entries[0].Name != "a.mp3" || entries[1].Name != "b.mp4" {
		t.Errorf("entries = %+v", entries)
	}
}
