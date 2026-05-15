package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bedri/open-directory-crawler/internal/models"
)

var apiBase = "http://localhost:40444"

type apiClient struct {
	url string
}

func newAPIClient() *apiClient {
	return &apiClient{url: apiBase}
}

func (c *apiClient) ping() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(c.url + "/stats")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *apiClient) getJSON(path string, v any) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(c.url + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API %s: %s", resp.Status, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *apiClient) Dirs() ([]*models.Directory, error) {
	var dirs []*models.Directory
	return dirs, c.getJSON("/dirs?limit=200000", &dirs)
}

func (c *apiClient) Stats() (*models.Stats, error) {
	var st models.Stats
	return &st, c.getJSON("/stats", &st)
}

func (c *apiClient) Search(query, cat string) ([]*models.FileEntry, error) {
	var files []*models.FileEntry
	p := "/search?q=" + query
	if cat != "" {
		p += "&cat=" + cat
	}
	return files, c.getJSON(p, &files)
}

func (c *apiClient) FilesByCat(cat string) ([]*models.FileEntry, error) {
	var files []*models.FileEntry
	return files, c.getJSON("/files?cat="+cat, &files)
}

func (c *apiClient) FilesByExt(ext string) ([]*models.FileEntry, error) {
	var files []*models.FileEntry
	return files, c.getJSON("/files?ext="+ext, &files)
}

func (c *apiClient) FilesByDir(dirID string) ([]*models.FileEntry, error) {
	var files []*models.FileEntry
	return files, c.getJSON("/dirs/"+dirID+"/files", &files)
}

func (c *apiClient) AllFiles() ([]*models.FileEntry, error) {
	var files []*models.FileEntry
	return files, c.getJSON("/files?limit=200000", &files)
}
