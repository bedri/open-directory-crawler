package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bedri/open-directory-crawler/internal/analysis"
	"github.com/bedri/open-directory-crawler/internal/crawler"
	"github.com/bedri/open-directory-crawler/internal/discover"
	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/spf13/cobra"
)

var (
	agentDBPath         string
	agentReaderPath     string
	agentType           string
	agentWorkers        int
	agentDepth          int
	agentDelay          int
	agentInterval       int
	agentFile           string
	agentOnce           bool
	agentLog            string
	agentAPIAddr        string
	agentAPIToken       string
	agentDownload       bool
	agentDownloadMax    int64
	agentDownloadLimit  int
	agentGDrive         bool
	agentGDriveURL      string
	agentGDriveFolder   string
	agentGDriveCat      string
	agentGDriveExt      string
	agentGDriveMinSize  int64
	agentGDriveMaxSize  int64
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Open directory crawl ajanı (sürekli çalışır)",
	Long: `Veritabanındaki pending durumdaki dizinleri otomatik crawl eder.
--file ile URL listesi verirsen, onları da veritabanına ekler.
--type ile sadece belirli tipte içeriği olan dizinleri crawl eder.
--once ile tek sefer çalışıp çıkar (service modunda --once kullanmayın).`,
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.OpenFile(agentLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("log error: %v", err)
		}
		logger := log.New(f, "", log.LstdFlags)
		logger.Println("Agent started")

		store, err := storage.New(agentDBPath)
		if err != nil {
			logger.Fatalf("storage error: %v", err)
		}
		defer store.Close()

		if agentAPIAddr != "" {
			go func() {
				handler := setupAPIHandler(store, false, agentAPIToken)
				logger.Printf("API server running on %s", agentAPIAddr)
				if err := http.ListenAndServe(agentAPIAddr, handler); err != nil {
					logger.Printf("api server error: %v", err)
				}
			}()
		}

		if agentFile != "" {
			importURLs(store, agentFile, logger)
		}

		cleanupStuckScanning(store, logger)

		crawlCfg := crawler.Config{
			MaxDepth:    agentDepth,
			Concurrency: agentWorkers,
			Delay:       time.Duration(agentDelay) * time.Millisecond,
			Download: crawler.DownloadConfig{
				Enabled:  agentDownload,
				MaxSize:  agentDownloadMax,
				MaxFiles: agentDownloadLimit,
			},
			GDrive: crawler.GDriveConfig{
				Enabled:    agentGDrive,
				WebhookURL: agentGDriveURL,
				FolderID:   agentGDriveFolder,
				Categories: splitCSV(agentGDriveCat),
				Extensions: splitCSV(agentGDriveExt),
				MinSize:    agentGDriveMinSize,
				MaxSize:    agentGDriveMaxSize,
			},
		}

		crawler.ResetDownloadCount()

		for {
			processed := processPending(store, logger, crawlCfg)
			if processed == 0 {
				if !agentOnce {
					logger.Printf("All done. Running analysis...")
					an := analysis.New(store)
					report, err := an.Run()
					if err != nil {
						logger.Printf("analysis error: %v", err)
					} else {
					if err := store.SaveAnalysis(report); err != nil {
						logger.Printf("save analysis error: %v", err)
					} else {
						if wl := analysis.BuildWordlist(report); len(wl) > 0 {
							if err := store.SaveWordlist(wl); err != nil {
								logger.Printf("save wordlist error: %v", err)
							}
						}
						logger.Printf("Analysis saved (%d keywords, %d tlds)",
							len(report.Keywords), len(report.TLDStats))
					}
					}

					logger.Printf("Syncing reader DB...")
					if err := storage.SyncToReader(agentDBPath, agentReaderPath); err != nil {
						logger.Printf("sync error: %v", err)
					} else {
						logger.Printf("Reader DB synced")
					}
					logger.Printf("Resetting all dirs for re-crawl...")
					resetAllDirs(store, logger)
				}
				break
			}
			logger.Printf("Sleeping %d seconds...", agentInterval)
			time.Sleep(time.Duration(agentInterval) * time.Second)
		}

		logger.Println("Agent stopped")
	},
}

func importURLs(store *storage.Store, path string, logger *log.Logger) {
	f, err := os.Open(path)
	if err != nil {
		logger.Printf("import file error: %v", err)
		return
	}
	defer f.Close()

	header := make([]byte, 2)
	f.Read(header)
	f.Seek(0, 0)

	if header[0] == '[' {
		var dirs []models.ImportedDir
		if err := json.NewDecoder(f).Decode(&dirs); err != nil {
			logger.Printf("JSON parse error: %v", err)
			return
		}
		imported := 0
		skipped := 0
		for _, d := range dirs {
			dir := dirFromURL(d.URL)
			existing, err := store.GetDirectory(dir.ID)
			if err == nil && existing != nil {
				skipped++
				continue
			}
			if err := store.SaveDirectory(dir); err == nil {
				imported++
			}
		}
		logger.Printf("Imported %d URLs from %s (%d skipped)", imported, path, skipped)
		return
	}

	sc := bufio.NewScanner(f)
	imported := 0
	skipped := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		dir := dirFromURL(line)
		existing, err := store.GetDirectory(dir.ID)
		if err == nil && existing != nil {
			skipped++
			continue
		}
		if err := store.SaveDirectory(dir); err == nil {
			imported++
		}
	}
	logger.Printf("Imported %d URLs from %s (%d skipped)", imported, path, skipped)
}

func processPending(store *storage.Store, logger *log.Logger, cfg crawler.Config) int {
	dirs, err := store.ListDirectories()
	if err != nil {
		logger.Printf("list error: %v", err)
		return 0
	}

	finder := discover.New()
	processed := 0

	for _, d := range dirs {
		if d.Status != models.StatusPending && d.Status != models.StatusError {
			continue
		}

		if agentType != "" {
			targetCat := models.FileCategory(agentType)
			if !finder.CheckDensity(d.URL, targetCat, 0.15) {
				d.Status = models.StatusDone
				d.Error = fmt.Sprintf("skipped: low %s density", agentType)
				store.SaveDirectory(d)
				continue
			}
		}

		logger.Printf("Crawling: %s", crawler.RedactURL(d.URL))
		d.Status = models.StatusScanning
		store.SaveDirectory(d)

		c := crawler.New(store, cfg)
		if err := c.Crawl(d.URL); err != nil {
			logger.Printf("Crawl error %s: %v", d.URL, err)
			d.Status = models.StatusError
			d.Error = err.Error()
			store.SaveDirectory(d)
		} else {
			logger.Printf("Crawl done: %s", d.URL)
		}
		processed++
	}

	return processed
}

func resetAllDirs(store *storage.Store, logger *log.Logger) {
	dirs, err := store.ListDirectories()
	if err != nil {
		logger.Printf("reset error: %v", err)
		return
	}
	for _, d := range dirs {
		d.Status = models.StatusPending
		store.SaveDirectory(d)
	}
	logger.Printf("Reset %d dirs to pending", len(dirs))
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.Flags().StringVar(&agentDBPath, "db", "./odk.db", "writer database path")
	agentCmd.Flags().StringVar(&agentReaderPath, "reader-db", "./odk-reader.db", "reader database path")
	agentCmd.Flags().StringVarP(&agentType, "type", "t", "", "sadece bu tipte içeriği olan dizinleri crawl et")
	agentCmd.Flags().IntVarP(&agentWorkers, "workers", "w", 3, "concurrent crawl workers")
	agentCmd.Flags().IntVarP(&agentDepth, "depth", "d", 2, "max crawl depth")
	agentCmd.Flags().IntVarP(&agentDelay, "delay", "D", 500, "delay between requests (ms)")
	agentCmd.Flags().IntVarP(&agentInterval, "interval", "I", 60, "seconds between rounds")
	agentCmd.Flags().StringVarP(&agentFile, "file", "f", "", "URL listesi dosyası")
	agentCmd.Flags().BoolVar(&agentOnce, "once", false, "tek sefer çalış ve çık")
	agentCmd.Flags().StringVar(&agentLog, "log", "./odk-agent.log", "log file path")
	agentCmd.Flags().StringVar(&agentAPIAddr, "api", "", "REST API bind address (örn: :40444)")
	agentCmd.Flags().StringVar(&agentAPIToken, "api-token", "", "Bearer token for API authentication")
	agentCmd.Flags().BoolVar(&agentDownload, "download", false, "download small files for metadata extraction")
	agentCmd.Flags().Int64Var(&agentDownloadMax, "download-max", 10<<20, "max download size in bytes (default 10MB)")
	agentCmd.Flags().IntVar(&agentDownloadLimit, "download-limit", 100, "max files to download per crawl cycle")
	agentCmd.Flags().BoolVar(&agentGDrive, "gdrive", false, "save files to Google Drive via Apps Script")
	agentCmd.Flags().StringVar(&agentGDriveURL, "gdrive-url", "", "Google Apps Script webhook URL")
	agentCmd.Flags().StringVar(&agentGDriveFolder, "gdrive-folder", "root", "Google Drive folder ID (default: root)")
	agentCmd.Flags().StringVar(&agentGDriveCat, "gdrive-cat", "", "kategori filtresi (virgülle ayır: video,audio,document)")
	agentCmd.Flags().StringVar(&agentGDriveExt, "gdrive-ext", "", "uzantı filtresi (virgülle ayır: mp4,pdf,zip)")
	agentCmd.Flags().Int64Var(&agentGDriveMinSize, "gdrive-min-size", 0, "minimum file size (default: 0 = no limit)")
	agentCmd.Flags().Int64Var(&agentGDriveMaxSize, "gdrive-max-size", 0, "maximum file size (default: 0 = no limit)")
}

func cleanupStuckScanning(store *storage.Store, logger *log.Logger) {
	dirs, err := store.ListDirectories()
	if err != nil {
		logger.Printf("cleanup error: %v", err)
		return
	}
	fixed := 0
	for _, d := range dirs {
		if d.Status == models.StatusScanning {
			d.Status = models.StatusPending
			d.Error = "reset from stuck scanning (agent restart)"
			store.SaveDirectory(d)
			fixed++
		}
	}
	if fixed > 0 {
		logger.Printf("Reset %d stuck StatusScanning dirs to StatusPending", fixed)
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}
