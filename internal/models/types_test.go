package models

import (
	"testing"
	"time"
)

func TestFileCategoryConstants(t *testing.T) {
	tests := []struct {
		cat  FileCategory
		want string
	}{
		{CategoryVideo, "video"},
		{CategoryAudio, "audio"},
		{CategoryImage, "image"},
		{CategoryDocument, "document"},
		{CategoryArchive, "archive"},
		{CategoryCode, "code"},
		{CategoryTorrent, "torrent"},
		{CategoryExecutable, "executable"},
		{CategoryOther, "other"},
	}
	for _, tt := range tests {
		if string(tt.cat) != tt.want {
			t.Errorf("FileCategory(%s) = %q, want %q", tt.want, string(tt.cat), tt.want)
		}
	}
}

func TestDirStatusConstants(t *testing.T) {
	tests := []struct {
		s    DirStatus
		want string
	}{
		{StatusPending, "pending"},
		{StatusScanning, "scanning"},
		{StatusDone, "done"},
		{StatusError, "error"},
	}
	for _, tt := range tests {
		if string(tt.s) != tt.want {
			t.Errorf("DirStatus(%s) = %q, want %q", tt.want, string(tt.s), tt.want)
		}
	}
}

func TestDirectoryDefaults(t *testing.T) {
	d := &Directory{}
	if d.Status != "" {
		t.Errorf("default Directory.Status = %q, want empty", d.Status)
	}
	if d.ScannedAt != (time.Time{}) {
		t.Error("default Directory.ScannedAt should be zero")
	}
}

func TestDirectoryFields(t *testing.T) {
	now := time.Now()
	d := &Directory{
		ID:        "test123",
		URL:       "http://example.com/files/",
		Title:     "test dir",
		Server:    "Apache",
		FileCount: 42,
		TotalSize: 1024,
		Depth:     1,
		ScannedAt: now,
		Status:    StatusDone,
		Error:     "",
	}
	if d.ID != "test123" || d.URL != "http://example.com/files/" || d.FileCount != 42 {
		t.Errorf("Directory fields not set correctly: %+v", d)
	}
}

func TestFileEntryDefaults(t *testing.T) {
	f := &FileEntry{}
	if f.Category != "" {
		t.Errorf("default FileEntry.Category = %q, want empty", f.Category)
	}
}

func TestStatsDefaults(t *testing.T) {
	s := &Stats{}
	if s.CategoryCounts != nil || s.ExtCounts != nil {
		t.Error("default Stats maps should be nil")
	}
}
