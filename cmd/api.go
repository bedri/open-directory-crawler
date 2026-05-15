package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

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
		store, err := storage.NewReadOnly(apiDBPath)
		if err != nil {
			log.Fatalf("storage error: %v", err)
		}
		defer store.Close()

		handler := setupAPIHandler(store, apiCORS)

		log.Printf("API server running on %s", apiAddr)
		printAPIEpilog()

		if err := http.ListenAndServe(apiAddr, handler); err != nil {
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

func setupAPIHandler(store *storage.Store, cors bool) http.Handler {
	mux := http.NewServeMux()
	srv := &apiServer{store: store}

	mux.HandleFunc("/stats", srv.handleStats)
	mux.HandleFunc("/stats/analysis", srv.handleAnalysis)
	mux.HandleFunc("/stats/keywords", srv.handleKeywords)
	mux.HandleFunc("/stats/tlds", srv.handleTLDs)
	mux.HandleFunc("/stats/edu", srv.handleEduBreakdown)
	mux.HandleFunc("/stats/domains", srv.handleDomains)
	mux.HandleFunc("/wordlist", srv.handleWordlist)
	mux.HandleFunc("/dirs", srv.handleDirs)
	mux.HandleFunc("/files", srv.handleFiles)
	mux.HandleFunc("/search", srv.handleSearch)
	mux.Handle("/", webui.Handler())

	if cors {
		return corsMiddleware(mux)
	}
	return mux
}

func printAPIEpilog() {
	log.Printf("Endpoints:")
	log.Printf("  GET /stats              — basic stats")
	log.Printf("  GET /stats/analysis     — full analysis report")
	log.Printf("  GET /stats/keywords     — top keywords?limit=100")
	log.Printf("  GET /stats/tlds         — TLD distribution")
	log.Printf("  GET /stats/edu          — edu/ac breakdown")
	log.Printf("  GET /stats/domains      — top domains?limit=50")
	log.Printf("  GET /wordlist           — keyword wordlist (text/plain)")
	log.Printf("  GET /dirs               — list dirs")
	log.Printf("  GET /search?q=<query>   — search files")
	log.Printf("  GET /files?cat=audio    — filter by category")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *apiServer) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, apiError{"method not allowed"})
		return
	}
	var report models.AnalysisReport
	if err := s.store.LoadAnalysis(&report); err != nil {
		writeJSON(w, 404, apiError{"analysis not available yet"})
		return
	}
	writeJSON(w, 200, &report)
}

func (s *apiServer) handleKeywords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, apiError{"method not allowed"})
		return
	}
	var report models.AnalysisReport
	if err := s.store.LoadAnalysis(&report); err != nil {
		writeJSON(w, 404, apiError{"analysis not available"})
		return
	}
	limit := 100
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	if limit <= 0 {
		limit = 100
	}
	if limit > len(report.Keywords) {
		limit = len(report.Keywords)
	}
	writeJSON(w, 200, report.Keywords[:limit])
}

func (s *apiServer) handleTLDs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, apiError{"method not allowed"})
		return
	}
	var report models.AnalysisReport
	if err := s.store.LoadAnalysis(&report); err != nil {
		writeJSON(w, 404, apiError{"analysis not available"})
		return
	}
	writeJSON(w, 200, report.TLDStats)
}

func (s *apiServer) handleEduBreakdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, apiError{"method not allowed"})
		return
	}
	var report models.AnalysisReport
	if err := s.store.LoadAnalysis(&report); err != nil {
		writeJSON(w, 404, apiError{"analysis not available"})
		return
	}
	if report.EduBreakdown == nil {
		writeJSON(w, 200, map[string]string{"message": "no edu data"})
		return
	}
	writeJSON(w, 200, report.EduBreakdown)
}

func (s *apiServer) handleDomains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, apiError{"method not allowed"})
		return
	}
	var report models.AnalysisReport
	if err := s.store.LoadAnalysis(&report); err != nil {
		writeJSON(w, 404, apiError{"analysis not available"})
		return
	}
	limit := 50
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	if limit <= 0 {
		limit = 50
	}
	if limit > len(report.TopDomains) {
		limit = len(report.TopDomains)
	}
	writeJSON(w, 200, report.TopDomains[:limit])
}

func (s *apiServer) handleWordlist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, apiError{"method not allowed"})
		return
	}
	data, err := s.store.LoadWordlist()
	if err != nil {
		writeJSON(w, 404, apiError{"wordlist not available"})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write(data)
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
		files, err = s.store.GetAllFiles()
	}

	if err != nil {
		writeJSON(w, 500, apiError{err.Error()})
		return
	}

	limit := 100000
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
	apiCmd.Flags().StringVar(&apiDBPath, "db", "./odk-reader.db", "database path (reader, sync edilen)")
	apiCmd.Flags().StringVarP(&apiAddr, "addr", "a", ":40444", "listen address")
	apiCmd.Flags().BoolVar(&apiCORS, "cors", false, "enable CORS headers")
}
