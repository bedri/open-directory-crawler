package crawler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bedri/open-directory-crawler/internal/models"
)

type GDriveConfig struct {
	Enabled     bool
	WebhookURL  string
	FolderID    string
	Categories  []string
	Extensions  []string
	MinSize     int64
	MaxSize     int64
	Concurrency int
}

type gdriveTask struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	FolderID string `json:"folder,omitempty"`
}

type gdriveResult struct {
	DriveID string `json:"driveId"`
	Size    int64  `json:"size"`
	MIME    string `json:"mime"`
	Name    string `json:"name"`
	Error   string `json:"error,omitempty"`
}

func gdriveShouldSave(f *models.FileEntry, cfg GDriveConfig) bool {
	if len(cfg.Categories) > 0 {
		if !contains(cfg.Categories, string(f.Category)) {
			return false
		}
	}
	if len(cfg.Extensions) > 0 {
		if !contains(cfg.Extensions, strings.TrimPrefix(f.Ext, ".")) {
			return false
		}
	}
	if cfg.MinSize > 0 && f.Size < cfg.MinSize {
		return false
	}
	if cfg.MaxSize > 0 && f.Size > cfg.MaxSize {
		return false
	}
	return true
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func sendToGDrive(webhookURL, fileURL, fileName, folderID string, timeout time.Duration) (*gdriveResult, error) {
	task := gdriveTask{URL: fileURL, Name: fileName, FolderID: folderID}
	body, _ := json.Marshal(task)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gdrive post: %w", err)
	}
	defer resp.Body.Close()

	var result gdriveResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gdrive decode: %w", err)
	}
	if result.Error != "" {
		return &result, fmt.Errorf("gdrive error: %s", result.Error)
	}
	return &result, nil
}
