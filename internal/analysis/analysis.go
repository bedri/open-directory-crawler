package analysis

import (
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
)

type Analyzer struct {
	store *storage.Store
	mu    sync.Mutex

	keywords map[string]int

	tldStats       map[string]*models.TLDInfo
	catExtMatrix   map[string]int
	sizeBuckets    [7]int64
	depthDirs      map[int]int
	depthFiles     map[int]int64
	filesPerDir    []int64
	eduBucket      models.EduBreakdown
	serverTypes    map[string]int
	catSizeBuckets map[models.FileCategory][7]int64
	domainCounts   map[string]int
	extCatMatrix   map[string]map[models.FileCategory]int64
}

func New(store *storage.Store) *Analyzer {
	return &Analyzer{
		store:          store,
		keywords:       make(map[string]int),
		tldStats:       make(map[string]*models.TLDInfo),
		catExtMatrix:   make(map[string]int),
		depthDirs:      make(map[int]int),
		depthFiles:     make(map[int]int64),
		filesPerDir:    make([]int64, 0),
		serverTypes:    make(map[string]int),
		catSizeBuckets: make(map[models.FileCategory][7]int64),
		domainCounts:   make(map[string]int),
		extCatMatrix:   make(map[string]map[models.FileCategory]int64),
	}
}

var wordSplitter = regexp.MustCompile(`[a-zA-Z0-9]+`)
var digitOnly = regexp.MustCompile(`^[0-9]+$`)
var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true,
	"this": true, "that": true, "are": true, "was": true, "has": true,
	"not": true, "but": true, "all": true, "can": true, "its": true,
	"pdf": true, "zip": true, "rar": true, "tar": true, "gz":  true,
	"exe": true, "mp3": true, "mp4": true, "jpg": true, "png": true,
	"gif": true, "txt": true, "doc": true, "html": true, "htm": true,
	"www": true, "com": true, "org": true, "net": true, "edu": true,
	"http": true, "https": true, "index": true,
}

func extractKeywords(name string, freq map[string]int) {
	words := wordSplitter.FindAllString(strings.ToLower(name), -1)
	for _, w := range words {
		if len(w) < 3 || len(w) > 30 {
			continue
		}
		if stopWords[w] {
			continue
		}
		if digitOnly.MatchString(w) {
			continue
		}
		freq[w]++
	}
}

func extractTLD(rawURL string) string {
	rawURL = strings.TrimPrefix(rawURL, "http://")
	rawURL = strings.TrimPrefix(rawURL, "https://")
	rawURL = strings.TrimPrefix(rawURL, "ftp://")
	parts := strings.Split(rawURL, "/")
	if len(parts) == 0 {
		return "unknown"
	}
	host := parts[0]
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	dot := strings.LastIndex(host, ".")
	if dot < 0 {
		return "unknown"
	}
	tld := host[dot+1:]
	if tld == "" {
		return "unknown"
	}
	return strings.ToLower(tld)
}

func extractDomain(rawURL string) string {
	rawURL = strings.TrimPrefix(rawURL, "http://")
	rawURL = strings.TrimPrefix(rawURL, "https://")
	rawURL = strings.TrimPrefix(rawURL, "ftp://")
	parts := strings.Split(rawURL, "/")
	if len(parts) == 0 || !strings.Contains(rawURL, ".") {
		return "unknown"
	}
	host := parts[0]
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	return strings.ToLower(host)
}

func sizeBucket(size int64) int {
	switch {
	case size < 1024:
		return 0
	case size < 10*1024:
		return 1
	case size < 100*1024:
		return 2
	case size < 1024*1024:
		return 3
	case size < 10*1024*1024:
		return 4
	case size < 100*1024*1024:
		return 5
	default:
		return 6
	}
}

var bucketLabels = []string{"<1KB", "1-10KB", "10-100KB", "100KB-1MB", "1-10MB", "10-100MB", ">100MB"}

func isEduDomain(domain, tld string) bool {
	if tld == "edu" || tld == "ac" {
		return true
	}
	eduTLDs := []string{".edu", ".ac.uk", ".ac.kr", ".ac.jp", ".ac.th", ".ac.in", ".ac.ir", ".ac.cn", ".ac.id"}
	for _, s := range eduTLDs {
		if strings.HasSuffix(domain, s) {
			return true
		}
	}
	return false
}

func (a *Analyzer) Run() (*models.AnalysisReport, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	start := time.Now()
	dirs, err := a.store.ListDirectories()
	if err != nil {
		return nil, err
	}

	files, err := a.store.GetAllFiles()
	if err != nil {
		return nil, err
	}

	for _, d := range dirs {
		tld := extractTLD(d.URL)
		ti, ok := a.tldStats[tld]
		if !ok {
			ti = &models.TLDInfo{
				Categories: make(map[models.FileCategory]int64),
			}
			a.tldStats[tld] = ti
		}
		ti.Directories++

		a.depthDirs[d.Depth]++
		a.serverTypes[d.Server]++
	}

	for _, f := range files {
		extractKeywords(f.Name, a.keywords)

		tld := extractTLD(f.URL)
		domain := extractDomain(f.URL)

		ti := a.tldStats[tld]
		if ti == nil {
			ti = &models.TLDInfo{Categories: make(map[models.FileCategory]int64)}
			a.tldStats[tld] = ti
		}
		ti.Files++
		ti.TotalSize += f.Size
		ti.Categories[f.Category]++

		a.domainCounts[domain]++

		if isEduDomain(domain, tld) {
			a.eduBucket.TotalFiles++
			if a.eduBucket.Categories == nil {
				a.eduBucket.Categories = make(map[models.FileCategory]int64)
			}
			a.eduBucket.Categories[f.Category]++
		}

		if _, ok := a.extCatMatrix[f.Ext]; !ok {
			a.extCatMatrix[f.Ext] = make(map[models.FileCategory]int64)
		}
		a.extCatMatrix[f.Ext][f.Category]++

		ceKey := string(f.Category) + ":" + f.Ext
		a.catExtMatrix[ceKey]++

		b := sizeBucket(f.Size)
		a.sizeBuckets[b]++

		csb := a.catSizeBuckets[f.Category]
		csb[b]++
		a.catSizeBuckets[f.Category] = csb
	}

	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}

	avgFPD := 0.0
	if len(dirs) > 0 {
		avgFPD = float64(len(files)) / float64(len(dirs))
	}

	sizeMap := make(map[string]int64)
	for i, label := range bucketLabels {
		sizeMap[label] = a.sizeBuckets[i]
	}

	catSizeMap := make(map[string][7]int64)
	for cat, buckets := range a.catSizeBuckets {
		catSizeMap[string(cat)] = buckets
	}

	report := &models.AnalysisReport{
		GeneratedAt:    start,
		Duration:       time.Since(start).String(),
		TLDStats:       a.tldStats,
		CatExtMatrix:   a.catExtMatrix,
		SizeBuckets:    sizeMap,
		DepthDirs:      a.depthDirs,
		AvgFilesPerDir: avgFPD,
		EduBreakdown:   &a.eduBucket,
		ServerTypes:    a.serverTypes,
		CatSizeBuckets: catSizeMap,
		ExtCatMatrix:   a.extCatMatrix,
		TotalDirs:      len(dirs),
		TotalFiles:     int64(len(files)),
		TotalSize:      totalSize,
	}

	var topKeywords []models.KeywordEntry
	for w, c := range a.keywords {
		topKeywords = append(topKeywords, models.KeywordEntry{Word: w, Count: c})
	}
	report.Keywords = topKeywords

	var topDomains []models.DomainEntry
	for d, c := range a.domainCounts {
		topDomains = append(topDomains, models.DomainEntry{Domain: d, Count: c})
	}
	report.TopDomains = topDomains

	log.Printf("analysis done: %d dirs, %d files, %d unique keywords in %s",
		report.TotalDirs, report.TotalFiles, len(report.Keywords), report.Duration)

	return report, nil
}

func BuildWordlist(report *models.AnalysisReport) []byte {
	var buf strings.Builder
	for _, k := range report.Keywords {
		buf.WriteString(k.Word)
		buf.WriteByte('\n')
	}
	return []byte(buf.String())
}
