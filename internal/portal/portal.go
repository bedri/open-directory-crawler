package portal

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
)

//go:embed index.html
var content embed.FS

type Server struct {
	store *storage.Store
}

func New(store *storage.Store) http.Handler {
	mux := http.NewServeMux()
	srv := &Server{store: store}

	mux.HandleFunc("/api/search", srv.handleSearch)
	mux.HandleFunc("/api/browse", srv.handleBrowse)
	mux.HandleFunc("/api/categories", srv.handleCategories)
	mux.HandleFunc("/api/suggest", srv.handleSuggest)
	mux.HandleFunc("/", srv.handleIndex)

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	writeJSON(w, map[string]string{"error": msg})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, _ := content.ReadFile("index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetStats()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, st.CategoryCounts)
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	cat := r.URL.Query().Get("cat")
	ext := r.URL.Query().Get("ext")
	offset := 0
	limit := 50
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var files []*models.FileEntry
	var err error

	if ext != "" {
		files, err = s.store.GetFilesByExt(ext)
	} else if cat != "" {
		files, err = s.store.GetFilesByCategory(models.FileCategory(cat))
	} else {
		files, err = s.store.GetAllFiles()
	}

	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	total := len(files)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	result := files[offset:end]
	if result == nil {
		result = []*models.FileEntry{}
	}

	w.Header().Set("X-Total-Count", fmt.Sprintf("%d", total))
	writeJSON(w, map[string]any{
		"files": result,
		"total": total,
		"limit": limit,
		"offset": offset,
	})
}

type searchResult struct {
	File  *models.FileEntry `json:"file"`
	DirURL string           `json:"dir_url"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query == "" {
		writeError(w, 400, "?q= required")
		return
	}

	cat := r.URL.Query().Get("cat")
	limit := 100
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	if limit < 1 || limit > 500 {
		limit = 100
	}

	dirs, err := s.store.ListDirectories()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	var matched []searchResult
	for _, d := range dirs {
		files, err := s.store.GetFilesByDir(d.ID)
		if err != nil {
			continue
		}
		for _, f := range files {
			if len(matched) >= limit {
				break
			}
			if !strings.Contains(strings.ToLower(f.Name), query) {
				continue
			}
			if cat != "" && string(f.Category) != cat {
				continue
			}
			matched = append(matched, searchResult{
				File:   f,
				DirURL: d.URL,
			})
		}
		if len(matched) >= limit {
			break
		}
	}

	if matched == nil {
		matched = []searchResult{}
	}

	writeJSON(w, map[string]any{
		"results": matched,
		"total":   len(matched),
	})
}

func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q == "" {
		writeJSON(w, []string{})
		return
	}

	data, err := s.store.LoadWordlist()
	if err != nil {
		writeJSON(w, []string{})
		return
	}

	words := strings.Split(string(data), "\n")
	var suggestions []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(w), q) {
			suggestions = append(suggestions, w)
			if len(suggestions) >= 20 {
				break
			}
		}
	}

	if suggestions == nil {
		suggestions = []string{}
	}
	writeJSON(w, suggestions)
}
