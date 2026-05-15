package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/bedri/open-directory-crawler/internal/webui"
	"github.com/spf13/cobra"
)

var (
	apiDBPath string
	apiAddr   string
	apiCORS   bool
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "REST API ile sorgulama sunucusu",
	Long: `BadgerDB üzerinden REST API sunar.
JSON formatında sorgulama yapabilirsin.

Endpoints:
  GET  /stats        İstatistikler
  GET  /dirs         Dizin listesi (?limit=N&offset=N)
  GET  /dirs/:id     Dizin detay
  GET  /dirs/:id/files  Dizindeki dosyalar
  GET  /search?q=    Dosya ara (?q=query&cat=audio&json)
  GET  /files?cat=   Kategoriye göre dosyalar (?cat=audio&ext=mp3&limit=N)

Örnek:
  odk api
  odk api --addr :8080 --cors`,
	Run: func(cmd *cobra.Command, args []string) {
		store, err := storage.New(apiDBPath)
		if err != nil {
			log.Fatalf("storage error: %v", err)
		}
		defer store.Close()

		mux := http.NewServeMux()
		srv := &apiServer{store: store}

		mux.HandleFunc("/stats", srv.handleStats)
		mux.HandleFunc("/dirs", srv.handleDirs)
		mux.HandleFunc("/files", srv.handleFiles)
		mux.HandleFunc("/search", srv.handleSearch)
		mux.Handle("/", webui.Handler())

		handler := http.Handler(mux)
		if apiCORS {
			handler = corsMiddleware(handler)
		}

		fmt.Printf("API server listening on %s\n", apiAddr)
		fmt.Printf("Endpoints:\n")
		fmt.Printf("  GET /stats\n")
		fmt.Printf("  GET /dirs\n")
		fmt.Printf("  GET /search?q=<query>\n")
		fmt.Printf("  GET /search?q=<query>&cat=audio\n")
		fmt.Printf("  GET /files?cat=audio\n")
		fmt.Printf("  GET /files?cat=video&limit=50\n")

		srvHTTP := &http.Server{
			Addr:         apiAddr,
			Handler:      handler,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
		}

		if err := srvHTTP.ListenAndServe(); err != nil {
			log.Fatalf("server error: %v", err)
		}
	},
}

type apiServer struct {
	store *storage.Store
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *apiServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, apiError{"method not allowed"})
		return
	}
	st, err := s.store.GetStats()
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}
	writeJSON(w, 200, st)
}

func (s *apiServer) handleDirs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, apiError{"method not allowed"})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/dirs")
	path = strings.TrimSuffix(path, "/")

	if path != "" && strings.HasPrefix(path, "/") {
		parts := strings.Split(path, "/")
		if len(parts) >= 3 && parts[2] == "files" {
			dirID := parts[1]
			files, err := s.store.GetFilesByDir(dirID)
			if err != nil {
				writeJSON(w, 500, apiError{err.Error()})
				return
			}
			if files == nil {
				files = []*models.FileEntry{}
			}
			writeJSON(w, 200, files)
			return
		}
		dirID := strings.TrimPrefix(path, "/")
		dir, err := s.store.GetDirectory(dirID)
		if err != nil {
			writeJSON(w, 404, apiError{"directory not found"})
			return
		}
		writeJSON(w, 200, dir)
		return
	}

	dirs, err := s.store.ListDirectories()
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}

	limit := 100
	offset := 0
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)

	if offset > len(dirs) {
		offset = len(dirs)
	}
	if limit <= 0 {
		limit = 100
	}
	end := offset + limit
	if end > len(dirs) {
		end = len(dirs)
	}

	result := dirs[offset:end]
	if result == nil {
		result = []*models.Directory{}
	}
	writeJSON(w, 200, result)
}

func (s *apiServer) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, apiError{"method not allowed"})
		return
	}

	cat := r.URL.Query().Get("cat")
	ext := r.URL.Query().Get("ext")

	var files []*models.FileEntry
	var err error

	if cat != "" {
		files, err = s.store.GetFilesByCategory(models.FileCategory(cat))
	} else if ext != "" {
		files, err = s.store.GetFilesByExt(ext)
	} else {
		writeJSON(w, 400, apiError{"specify ?cat= or ?ext="})
		return
	}

	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}

	limit := 1000
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}

	if files == nil {
		files = []*models.FileEntry{}
	}
	writeJSON(w, 200, files)
}

func (s *apiServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, apiError{"method not allowed"})
		return
	}

	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, 400, apiError{"?q=<query> required"})
		return
	}

	cat := r.URL.Query().Get("cat")

	dirs, err := s.store.ListDirectories()
	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}

	type result struct {
		File  *models.FileEntry `json:"file"`
		DirID string            `json:"directory_id"`
		DirURL string           `json:"directory_url"`
	}

	var matched []result
	for _, d := range dirs {
		files, err := s.store.GetFilesByDir(d.ID)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !strings.Contains(strings.ToLower(f.Name), query) {
				continue
			}
			if cat != "" && string(f.Category) != cat {
				continue
			}
			matched = append(matched, result{
				File:  f,
				DirID: d.ID,
				DirURL: d.URL,
			})
		}
	}

	limit := 100
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}

	if matched == nil {
		matched = []result{}
	}
	writeJSON(w, 200, matched)
}

func (s *apiServer) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 404, apiError{"not found"})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.Flags().StringVar(&apiDBPath, "db", "./odk.db", "database path")
	apiCmd.Flags().StringVarP(&apiAddr, "addr", "a", ":40444", "listen address")
	apiCmd.Flags().BoolVar(&apiCORS, "cors", false, "enable CORS headers")
}
