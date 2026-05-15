package parser

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bedri/open-directory-crawler/internal/classify"
	"github.com/bedri/open-directory-crawler/internal/models"
)

type FileLink struct {
	Name         string
	URL          string
	Size         int64
	LastModified time.Time
	IsDir        bool
}

func ParseDirectoryListing(rawURL string, body string) (string, []FileLink, error) {
	title := extractTitle(body)

	links := parseApacheStyle(rawURL, body)
	if links == nil {
		links = parseNginxStyle(rawURL, body)
	}
	if links == nil {
		links = parseGenericLinks(rawURL, body)
	}

	return title, links, nil
}

func extractTitle(body string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return ""
	}
	title := strings.TrimSpace(doc.Find("title").First().Text())
	title = strings.ReplaceAll(title, "Index of /", "")
	title = strings.TrimSpace(title)
	return title
}

func parseApacheStyle(rawURL string, body string) []FileLink {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil
	}

	var links []FileLink
	found := false

	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		if found {
			return
		}
		table.Find("tr").Each(func(_ int, row *goquery.Selection) {
			cells := row.Find("td")
			if cells.Length() < 2 {
				return
			}

			linkEl := cells.First().Find("a")
			href, exists := linkEl.Attr("href")
			if !exists || href == "" {
				return
			}

			baseURL, _ := url.Parse(rawURL)
			absURL := resolveURL(baseURL, href)

			name := strings.TrimSpace(linkEl.Text())
			if name == "../" || name == "Parent Directory" {
				return
			}

			isDir := strings.HasSuffix(href, "/")

			fl := FileLink{
				Name:  strings.TrimSuffix(name, "/"),
				URL:   absURL,
				IsDir: isDir,
			}

			if cells.Length() >= 3 {
				sizeText := strings.TrimSpace(cells.Eq(2).Text())
				fl.Size = parseSize(sizeText)
			}

			links = append(links, fl)
			found = true
		})
	})

	if found {
		return links
	}
	return nil
}

func parseNginxStyle(rawURL string, body string) []FileLink {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil
	}

	var links []FileLink
	found := false

	doc.Find("pre").Each(func(_ int, pre *goquery.Selection) {
		if found {
			return
		}
		pre.Find("a").Each(func(_ int, a *goquery.Selection) {
			href, exists := a.Attr("href")
			if !exists || href == "" {
				return
			}
			if href == "../" {
				return
			}

			baseURL, _ := url.Parse(rawURL)
			absURL := resolveURL(baseURL, href)

			name := strings.TrimSpace(a.Text())
			isDir := strings.HasSuffix(href, "/")

			links = append(links, FileLink{
				Name:  strings.TrimSuffix(name, "/"),
				URL:   absURL,
				IsDir: isDir,
			})
			found = true
		})
	})

	if found {
		return links
	}
	return nil
}

func parseGenericLinks(rawURL string, body string) []FileLink {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil
	}

	var links []FileLink

	doc.Find("a").Each(func(_ int, a *goquery.Selection) {
		href, exists := a.Attr("href")
		if !exists || href == "" {
			return
		}

		href = strings.TrimSpace(href)
		if href == "../" || href == "#" || href == "/" {
			return
		}

		if strings.HasPrefix(href, "?") || strings.HasPrefix(href, "#") {
			return
		}

		if strings.Contains(href, "mailto:") || strings.Contains(href, "javascript:") {
			return
		}

		baseURL, _ := url.Parse(rawURL)
		absURL := resolveURL(baseURL, href)

		name := strings.TrimSpace(a.Text())
		isDir := strings.HasSuffix(href, "/")

		links = append(links, FileLink{
			Name:  strings.TrimSuffix(name, "/"),
			URL:   absURL,
			IsDir: isDir,
		})
	})

	return links
}

func resolveURL(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	u, err := url.Parse(href)
	if err != nil {
		return ""
	}

	resolved := base.ResolveReference(u)
	return resolved.String()
}

func parseSize(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))

	if s == "-" || s == "" {
		return 0
	}

	var multipliers = map[string]int64{
		"K": 1024,
		"M": 1024 * 1024,
		"G": 1024 * 1024 * 1024,
		"T": 1024 * 1024 * 1024 * 1024,
	}

	suffixes := []string{"KB", "MB", "GB", "TB", "K", "M", "G", "T"}

	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			numStr = strings.TrimSuffix(numStr, "B")
			numStr = strings.TrimSpace(numStr)
			var val float64
			if _, err := fmt.Sscanf(numStr, "%f", &val); err == nil {
				mult := multipliers[string(suffix[0])]
				return int64(val * float64(mult))
			}
		}
	}

	var val int64
	if _, err := fmt.Sscanf(s, "%d", &val); err == nil {
		return val
	}
	return 0
}

func FileLinksToEntries(links []FileLink, dirID, parentURL string) []*models.FileEntry {
	var entries []*models.FileEntry
	for _, l := range links {
		if l.IsDir {
			continue
		}
		name := path.Base(l.URL)
		if name == "" || name == "." || name == "/" {
			continue
		}

		entries = append(entries, &models.FileEntry{
			ID:          dirID + ":" + name,
			Name:        name,
			URL:         l.URL,
			Size:        l.Size,
			Ext:         classify.Extension(name),
			Category:    classify.FileEntry(name, l.Size),
			ParentURL:   parentURL,
			DirectoryID: dirID,
		})
	}
	return entries
}


