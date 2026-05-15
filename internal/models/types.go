package models

import "time"

type FileCategory string

const (
	CategoryVideo    FileCategory = "video"
	CategoryAudio    FileCategory = "audio"
	CategoryImage    FileCategory = "image"
	CategoryDocument FileCategory = "document"
	CategoryArchive  FileCategory = "archive"
	CategoryCode     FileCategory = "code"
	CategoryTorrent   FileCategory = "torrent"
	CategoryExecutable FileCategory = "executable"
	CategoryOther    FileCategory = "other"
)

type DirStatus string

const (
	StatusPending  DirStatus = "pending"
	StatusScanning DirStatus = "scanning"
	StatusDone     DirStatus = "done"
	StatusError    DirStatus = "error"
)

type Directory struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Server    string    `json:"server"`
	FileCount int       `json:"file_count"`
	TotalSize int64     `json:"total_size"`
	Depth     int       `json:"depth"`
	ScannedAt time.Time `json:"scanned_at"`
	Status    DirStatus `json:"status"`
	Error     string    `json:"error,omitempty"`
}

type FileEntry struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	URL          string            `json:"url"`
	Size         int64             `json:"size"`
	Ext          string            `json:"ext"`
	Category     FileCategory      `json:"category"`
	LastModified time.Time         `json:"last_modified"`
	ParentURL    string            `json:"parent_url"`
	DirectoryID  string            `json:"directory_id"`
	Active       bool              `json:"active"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type CrawlJob struct {
	URL    string
	Depth  int
	MaxDepth int
}

type Stats struct {
	TotalDirectories int                    `json:"total_directories"`
	TotalFiles       int64                  `json:"total_files"`
	TotalSize        int64                  `json:"total_size"`
	CategoryCounts   map[FileCategory]int64 `json:"category_counts"`
	ExtCounts        map[string]int         `json:"ext_counts"`
}

type AnalysisReport struct {
	GeneratedAt    time.Time                  `json:"generated_at"`
	Duration       string                     `json:"duration,omitempty"`
	Keywords       []KeywordEntry             `json:"keywords,omitempty"`
	TLDStats       map[string]*TLDInfo        `json:"tld_stats"`
	CatExtMatrix   map[string]int             `json:"cat_ext_matrix"`
	SizeBuckets    map[string]int64           `json:"size_buckets"`
	DepthDirs      map[int]int                `json:"depth_dirs"`
	AvgFilesPerDir float64                    `json:"avg_files_per_dir"`
	EduBreakdown   *EduBreakdown              `json:"edu_breakdown"`
	ServerTypes    map[string]int             `json:"server_types"`
	CatSizeBuckets map[string][7]int64        `json:"cat_size_buckets"`
	TopDomains     []DomainEntry              `json:"top_domains"`
	ExtCatMatrix   map[string]map[FileCategory]int64 `json:"ext_cat_matrix"`
	TotalDirs      int                        `json:"total_dirs"`
	TotalFiles     int64                      `json:"total_files"`
	TotalSize      int64                      `json:"total_size"`
}

type KeywordEntry struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

type DomainEntry struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

type TLDInfo struct {
	Directories int                    `json:"directories"`
	Files       int64                  `json:"files"`
	TotalSize   int64                  `json:"total_size"`
	Categories  map[FileCategory]int64 `json:"categories"`
}

type EduBreakdown struct {
	TotalFiles int64                    `json:"total_files"`
	Categories map[FileCategory]int64   `json:"categories"`
}
