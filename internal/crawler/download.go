package crawler

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func downloadAndProcess(url, ua string, timeout time.Duration, maxSize int64) map[string]string {
	m := make(map[string]string)

	client := &http.Client{Timeout: timeout}

	headReq, _ := http.NewRequest("HEAD", url, nil)
	headReq.Header.Set("User-Agent", ua)
	headResp, err := client.Do(headReq)
	if err != nil {
		return m
	}
	headResp.Body.Close()

	contentLength := headResp.ContentLength
	if contentLength < 0 {
		contentLength = 0
	}
	if maxSize > 0 && contentLength > maxSize {
		m["skip_reason"] = fmt.Sprintf("too large: %d bytes", contentLength)
		return m
	}

	ct := headResp.Header.Get("Content-Type")
	if ct != "" {
		m["content_type"] = ct
	}

	tmpFile, err := os.CreateTemp("", "odk-dl-*")
	if err != nil {
		log.Printf("temp file error: %v", err)
		return m
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	getReq, _ := http.NewRequest("GET", url, nil)
	getReq.Header.Set("User-Agent", ua)
	resp, err := client.Do(getReq)
	if err != nil {
		tmpFile.Close()
		return m
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, 10<<20)

	written, err := io.Copy(tmpFile, limited)
	if err != nil {
		tmpFile.Close()
		return m
	}
	tmpFile.Close()

	if resp.ContentLength > 0 && written < resp.ContentLength {
		m["partial"] = "true"
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return m
	}

	hash := sha256.Sum256(data)
	m["sha256"] = fmt.Sprintf("%x", hash)

	ext := strings.ToLower(filepath.Ext(url))

	mime := ct
	if mime == "" {
		mime = detectMIME(data, ext)
	}
	m["mime"] = mime

	switch {
	case strings.HasPrefix(mime, "text/") || isCodeExt(ext):
		extractText(data, ext, m)
	case strings.HasPrefix(mime, "image/"):
		extractImageInfo(data, m)
	case strings.HasPrefix(mime, "application/pdf"):
		extractPDFText(data, ext, m)
	}

	return m
}

func detectMIME(data []byte, ext string) string {
	if len(data) == 0 {
		return "application/octet-stream"
	}

	switch ext {
	case ".pdf":
		if bytes.HasPrefix(data, []byte("%PDF")) {
			return "application/pdf"
		}
	case ".html", ".htm":
		return "text/html"
	case ".xml":
		return "application/xml"
	}

	ct := http.DetectContentType(data[:min(len(data), 512)])
	return ct
}

func extractText(data []byte, ext string, m map[string]string) {
	content := string(data)
	content = strings.TrimSpace(content)
	if len(content) > 10000 {
		content = content[:10000]
	}
	if len(content) > 0 {
		m["text_preview"] = content
	}
}

func extractImageInfo(data []byte, m map[string]string) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return
	}
	m["width"] = fmt.Sprintf("%d", cfg.Width)
	m["height"] = fmt.Sprintf("%d", cfg.Height)
	if cfg.ColorModel != nil {
		m["color_model"] = fmt.Sprintf("%T", cfg.ColorModel)
	}
}

func extractPDFText(data []byte, ext string, m map[string]string) {
	if bytes.HasPrefix(data, []byte("%PDF")) {
		raw := string(data)
		raw = strings.TrimSpace(raw)
		if pe := extractPDFRough(raw); pe != "" {
			if len(pe) > 10000 {
				pe = pe[:10000]
			}
			m["text_preview"] = pe
		}
	}
}

func extractPDFRough(raw string) string {
	var buf strings.Builder
	inText := false
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "(") && strings.Contains(line, ")") {
			s := strings.TrimPrefix(line, "(")
			if idx := strings.LastIndex(s, ")"); idx >= 0 {
				s = s[:idx]
				s = strings.TrimSpace(s)
				if len(s) > 1 && !isAllGarbage(s) {
					buf.WriteString(s)
					buf.WriteString(" ")
				}
			}
		}
		if strings.HasPrefix(line, "BT") {
			inText = true
			continue
		}
		if strings.HasPrefix(line, "ET") {
			inText = false
			continue
		}

		if inText {
			if idx := strings.Index(line, "("); idx >= 0 {
				s := line[idx+1:]
				if endIdx := strings.Index(s, ")"); endIdx >= 0 {
					s = s[:endIdx]
					s = strings.TrimSpace(s)
					if len(s) > 1 && !isAllGarbage(s) {
						buf.WriteString(s)
						buf.WriteString(" ")
					}
				}
			}
		}
	}
	result := strings.TrimSpace(buf.String())
	if len(result) < 5 {
		return ""
	}
	return result
}

func isAllGarbage(s string) bool {
	garbage := 0
	for _, r := range s {
		if r < 32 || r > 126 {
			garbage++
		}
	}
	return float64(garbage)/float64(len(s)) > 0.3
}

func isCodeExt(ext string) bool {
	switch ext {
	case ".go", ".rs", ".py", ".js", ".ts", ".c", ".cpp", ".h", ".hpp",
		".java", ".rb", ".php", ".pl", ".sh", ".bash", ".zsh",
		".yaml", ".yml", ".json", ".xml", ".toml", ".ini", ".cfg",
		".md", ".txt", ".log", ".sql", ".css", ".scss", ".less",
		".html", ".htm", ".svelte", ".vue", ".jsx", ".tsx":
		return true
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
