package crawler

import (
	"net/http"
	"strings"
	"time"
)

func probeMetadata(rawURL, name string, timeout time.Duration) map[string]string {
	m := make(map[string]string)

	ct := probeContentType(rawURL, timeout)
	if ct != "" {
		m["content_type"] = ct
	}

	if d := parseDimensionFromName(name); d != "" {
		m["dimension"] = d
	}

	return m
}

func probeContentType(rawURL string, timeout time.Duration) string {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("HEAD", rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	return resp.Header.Get("Content-Type")
}

func parseDimensionFromName(name string) string {
	idx := strings.LastIndexByte(name, '_')
	if idx < 0 {
		return ""
	}
	part := name[idx+1:]
	dimIdx := strings.IndexByte(part, 'x')
	if dimIdx < 0 {
		return ""
	}
	w := part[:dimIdx]
	h := part[dimIdx+1:]
	dot := strings.IndexByte(h, '.')
	if dot > 0 {
		h = h[:dot]
	}
	if isDigits(w) && isDigits(h) {
		return w + "x" + h
	}
	return ""
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
