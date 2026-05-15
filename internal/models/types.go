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
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	URL          string       `json:"url"`
	Size         int64        `json:"size"`
	Ext          string       `json:"ext"`
	Category     FileCategory `json:"category"`
	LastModified time.Time    `json:"last_modified"`
	ParentURL    string       `json:"parent_url"`
	DirectoryID  string       `json:"directory_id"`
}

type CrawlJob struct {
	URL    string
	Depth  int
	MaxDepth int
}

type Stats struct {
	TotalDirectories int            `json:"total_directories"`
	TotalFiles       int64          `json:"total_files"`
	TotalSize        int64          `json:"total_size"`
	CategoryCounts   map[FileCategory]int64 `json:"category_counts"`
	ExtCounts        map[string]int         `json:"ext_counts"`
}
