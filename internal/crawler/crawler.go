package crawler

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/bedri/open-directory-crawler/internal/classify"
	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/parser"
	"github.com/bedri/open-directory-crawler/internal/storage"
)

type Config struct {
	MaxDepth    int
	Concurrency int
	Delay       time.Duration
	UserAgent   string
	MaxFileSize int64
	Timeout     time.Duration
}

type Crawler struct {
	cfg   Config
	store *storage.Store
	mu    sync.Mutex
	jobs  chan models.CrawlJob
}

func New(store *storage.Store, cfg Config) *Crawler {
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 3
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 5
	}
	if cfg.Delay == 0 {
		cfg.Delay = 200 * time.Millisecond
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Crawler{
		cfg:   cfg,
		store: store,
		jobs:  make(chan models.CrawlJob, 100),
	}
}

func (c *Crawler) Crawl(startURL string) error {
	parsed, err := url.Parse(startURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	normalized := parsed.String()
	dirID := urlToID(normalized)

	dir := &models.Directory{
		ID:     dirID,
		URL:    normalized,
		Status: models.StatusScanning,
		Depth:  0,
	}
	if err := c.store.SaveDirectory(dir); err != nil {
		return fmt.Errorf("save dir: %w", err)
	}

	var wg sync.WaitGroup
	quit := make(chan struct{})

	for i := 0; i < c.cfg.Concurrency; i++ {
		go c.worker(&wg, quit)
	}

	wg.Add(1)
	c.jobs <- models.CrawlJob{URL: normalized, Depth: 0, MaxDepth: c.cfg.MaxDepth}

	wg.Wait()
	close(quit)
	return nil
}

func (c *Crawler) worker(wg *sync.WaitGroup, quit chan struct{}) {
	for {
		select {
		case job, ok := <-c.jobs:
			if !ok {
				return
			}
			c.crawlPage(wg, job)
		case <-quit:
			return
		}
	}
}

func (c *Crawler) crawlPage(wg *sync.WaitGroup, job models.CrawlJob) {
	defer wg.Done()
	time.Sleep(c.cfg.Delay)

	body, err := fetchURL(job.URL, c.cfg.UserAgent, c.cfg.Timeout)
	if err != nil {
		log.Printf("fetch error %s: %v", job.URL, err)
		return
	}

	title, links, err := parser.ParseDirectoryListing(job.URL, body)
	if err != nil {
		log.Printf("parse error %s: %v", job.URL, err)
		return
	}

	dirID := urlToID(job.URL)

	c.mu.Lock()
	if d, err := c.store.GetDirectory(dirID); err == nil && title != "" {
		d.Title = title
		c.store.SaveDirectory(d)
	}
	c.mu.Unlock()

	var fileEntries []*models.FileEntry
	var subDirs []string

	for _, link := range links {
		if link.IsDir {
			if shouldFollowDir(link.Name) && job.Depth < job.MaxDepth {
				subDirs = append(subDirs, link.URL)
			}
			continue
		}

			name := strings.TrimSpace(link.Name)
		if !parser.IsValidName(name) {
			if p := path.Base(link.URL); parser.IsValidName(p) {
				name = p
			} else {
				continue
			}
		}

		entry := &models.FileEntry{
			ID:          dirID + ":" + name,
			Name:        name,
			URL:         link.URL,
			Size:        link.Size,
			Ext:         classify.Extension(name),
			Category:    classify.FileEntry(name, link.Size),
			ParentURL:   job.URL,
			DirectoryID: dirID,
		}

		if c.cfg.MaxFileSize > 0 && entry.Size > c.cfg.MaxFileSize {
			continue
		}

		fileEntries = append(fileEntries, entry)
	}

	c.mu.Lock()
	for _, f := range fileEntries {
		if err := c.store.SaveFileEntry(f); err != nil {
			log.Printf("save file error: %v", err)
		}
	}
	if d, err := c.store.GetDirectory(dirID); err == nil {
		d.Title = title
		d.FileCount += len(fileEntries)
		d.TotalSize += func() int64 {
			var total int64
			for _, f := range fileEntries {
				total += f.Size
			}
			return total
		}()
		d.Status = models.StatusDone
		d.ScannedAt = time.Now()
		c.store.SaveDirectory(d)
	}
	c.mu.Unlock()

	for _, sub := range subDirs {
		subID := urlToID(sub)
		subDir := &models.Directory{
			ID:     subID,
			URL:    sub,
			Status: models.StatusPending,
			Depth:  job.Depth + 1,
		}
		c.mu.Lock()
		c.store.SaveDirectory(subDir)
		c.mu.Unlock()

		wg.Add(1)
		c.jobs <- models.CrawlJob{
			URL:      sub,
			Depth:    job.Depth + 1,
			MaxDepth: job.MaxDepth,
		}
	}
}

func shouldFollowDir(name string) bool {
	name = strings.ToLower(name)
	return name != "" && name != ".." && name != ".svn" && name != ".git"
}

func fetchURL(rawURL, ua string, timeout time.Duration) (string, error) {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") && !strings.HasPrefix(ct, "text/plain") {
		return "", fmt.Errorf("unexpected content-type: %s", ct)
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func urlToID(rawURL string) string {
	u, _ := url.Parse(rawURL)
	var sum [32]byte
	if u == nil {
		sum = sha256.Sum256([]byte(rawURL))
		return fmt.Sprintf("%x", sum[:8])
	}
	key := u.Host + u.Path
	sum = sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", sum[:8])
}
