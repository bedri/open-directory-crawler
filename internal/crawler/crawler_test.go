package crawler

import (
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	c := New(nil, Config{})
	if c.cfg.MaxDepth != 3 {
		t.Errorf("default MaxDepth = %d, want 3", c.cfg.MaxDepth)
	}
	if c.cfg.Concurrency != 5 {
		t.Errorf("default Concurrency = %d, want 5", c.cfg.Concurrency)
	}
	if c.cfg.Delay != 200*time.Millisecond {
		t.Errorf("default Delay = %v, want 200ms", c.cfg.Delay)
	}
	if c.cfg.UserAgent != "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" {
		t.Errorf("default UserAgent = %q", c.cfg.UserAgent)
	}
	if c.cfg.Timeout != 30*time.Second {
		t.Errorf("default Timeout = %v, want 30s", c.cfg.Timeout)
	}
}

func TestNewCustomConfig(t *testing.T) {
	cfg := Config{
		MaxDepth:    5,
		Concurrency: 2,
		Delay:       100 * time.Millisecond,
		UserAgent:   "custom-ua",
		Timeout:     10 * time.Second,
	}
	c := New(nil, cfg)
	if c.cfg.MaxDepth != 5 || c.cfg.Concurrency != 2 || c.cfg.Delay != 100*time.Millisecond {
		t.Errorf("custom config not applied: %+v", c.cfg)
	}
}

func TestShouldFollowDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"subdir", true},
		{"photos", true},
		{"", false},
		{"..", false},
		{".svn", false},
		{".git", false},
		{".SVN", false},
		{".GIT", false},
	}
	for _, tt := range tests {
		got := shouldFollowDir(tt.name)
		if got != tt.want {
			t.Errorf("shouldFollowDir(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestUrlToID(t *testing.T) {
	tests := []struct {
		rawURL string
	}{
		{"http://example.com/files/"},
		{"http://example.com/files"},
		{"https://mirror.example.com/pub/linux/"},
		{"ftp://ftp.example.org/pub/"},
		{"http://example.com/path/to/dir/?query=param"},
	}
	seen := make(map[string]bool)
	for _, tt := range tests {
		id := urlToID(tt.rawURL)
		if id == "" {
			t.Errorf("urlToID(%q) returned empty", tt.rawURL)
		}
		if len(id) != 16 {
			t.Errorf("urlToID(%q) = %q, len=%d, want 16", tt.rawURL, id, len(id))
		}
		if seen[id] {
			t.Errorf("duplicate id for %q: %s", tt.rawURL, id)
		}
		seen[id] = true
	}
}

func TestUrlToIDConsistency(t *testing.T) {
	id1 := urlToID("http://example.com/files/")
	id2 := urlToID("http://example.com/files/")
	if id1 != id2 {
		t.Errorf("urlToID not consistent: %s vs %s", id1, id2)
	}
}


