package cmd

import (
	"fmt"
	"log"
	"time"

	"github.com/bedri/open-directory-crawler/internal/crawler"
	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/spf13/cobra"
)

var (
	crawlDepth       int
	crawlConcurrency int
	crawlDelay       int
	crawlDBPath      string
	crawlMaxFilesize int64
)

var crawlCmd = &cobra.Command{
	Use:   "crawl [url]",
	Short: "Bir open directory'yi crawl et",
	Long:  `Verilen URL'deki open directory'yi recursive olarak crawl eder, dosyaları kategorize eder ve veritabanına kaydeder.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		store, err := storage.New(crawlDBPath)
		if err != nil {
			log.Fatalf("storage error: %v", err)
		}
		defer store.Close()

		cfg := crawler.Config{
			MaxDepth:    crawlDepth,
			Concurrency: crawlConcurrency,
			Delay:       time.Duration(crawlDelay) * time.Millisecond,
			MaxFileSize: crawlMaxFilesize,
		}

		c := crawler.New(store, cfg)
		fmt.Printf("Crawling: %s\n", args[0])
		fmt.Printf("Max depth: %d, Concurrency: %d\n", crawlDepth, crawlConcurrency)

		start := time.Now()
		if err := c.Crawl(args[0]); err != nil {
			log.Fatalf("crawl error: %v", err)
		}

		fmt.Printf("Done! Took %v\n", time.Since(start))
	},
}

func init() {
	rootCmd.AddCommand(crawlCmd)
	crawlCmd.Flags().IntVarP(&crawlDepth, "depth", "d", 3, "max crawl depth")
	crawlCmd.Flags().IntVarP(&crawlConcurrency, "concurrency", "c", 5, "number of concurrent workers")
	crawlCmd.Flags().IntVarP(&crawlDelay, "delay", "w", 200, "delay between requests (ms)")
	crawlCmd.Flags().Int64Var(&crawlMaxFilesize, "max-size", 0, "skip files larger than this (bytes)")
	crawlCmd.Flags().StringVar(&crawlDBPath, "db", "./odk.db", "database path")
}
