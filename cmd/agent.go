package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bedri/open-directory-crawler/internal/crawler"
	"github.com/bedri/open-directory-crawler/internal/discover"
	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/spf13/cobra"
)

var (
	agentDBPath   string
	agentType     string
	agentWorkers  int
	agentDepth    int
	agentDelay    int
	agentInterval int
	agentFile     string
	agentOnce     bool
	agentLog      string
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

		if agentFile != "" {
			importURLs(store, agentFile, logger)
		}

		crawlCfg := crawler.Config{
			MaxDepth:    agentDepth,
			Concurrency: agentWorkers,
			Delay:       time.Duration(agentDelay) * time.Millisecond,
		}

		for {
			processed := processPending(store, logger, crawlCfg)
			if agentOnce || processed == 0 {
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

		logger.Printf("Crawling: %s", d.URL)
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

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.Flags().StringVar(&agentDBPath, "db", "./odk.db", "database path")
	agentCmd.Flags().StringVarP(&agentType, "type", "t", "", "sadece bu tipte içeriği olan dizinleri crawl et")
	agentCmd.Flags().IntVarP(&agentWorkers, "workers", "w", 3, "concurrent crawl workers")
	agentCmd.Flags().IntVarP(&agentDepth, "depth", "d", 2, "max crawl depth")
	agentCmd.Flags().IntVarP(&agentDelay, "delay", "D", 500, "delay between requests (ms)")
	agentCmd.Flags().IntVarP(&agentInterval, "interval", "I", 60, "seconds between rounds")
	agentCmd.Flags().StringVarP(&agentFile, "file", "f", "", "URL listesi dosyası")
	agentCmd.Flags().BoolVar(&agentOnce, "once", false, "tek sefer çalış ve çık")
	agentCmd.Flags().StringVar(&agentLog, "log", "./odk-agent.log", "log file path")
}
