package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/bedri/open-directory-crawler/internal/analysis"
	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/spf13/cobra"
)

var (
	analyzeDBPath   string
	analyzeExport   string
	analyzeWordlist string
	analyzeForce    bool
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Veritabanı analizi: keyword, TLD, istatistikler",
	Long: `Tüm dosyaları tarar ve kapsamlı istatistikler üretir.

Varsayılan: kaydedilmiş analiz varsa gösterir, yoksa çalıştırır.
--force: analizi yeniden çalıştırır (writer DB gerekir).
--export file.json: JSON raporu kaydeder.
--wordlist file.txt: keyword listesini satır satır kaydeder.`,
	Run: func(cmd *cobra.Command, args []string) {
		var report *models.AnalysisReport

		if !analyzeForce {
			store, err := storage.NewReadOnly(analyzeDBPath)
			if err == nil {
				var r models.AnalysisReport
				err = store.LoadAnalysis(&r)
				store.Close()
				if err == nil && r.TotalFiles > 0 {
					report = &r
				}
			}
		}

	if report == nil {
		if !analyzeForce {
			log.Printf("No cached analysis found, running analysis...")
		}
		store, err := storage.New(analyzeDBPath)
		if err != nil {
			log.Fatalf("storage error: %v", err)
		}
		an := analysis.New(store)
		r, err := an.Run()
		store.Close()
		if err != nil {
			log.Fatalf("analysis error: %v", err)
		}
		report = r
	}

		if analyzeExport != "" {
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				log.Fatalf("json error: %v", err)
			}
			if err := os.WriteFile(analyzeExport, data, 0644); err != nil {
				log.Fatalf("export error: %v", err)
			}
			fmt.Printf("Report: %s (%d bytes)\n", analyzeExport, len(data))
		}

		if analyzeWordlist != "" {
			data := analysis.BuildWordlist(report)
			if err := os.WriteFile(analyzeWordlist, data, 0644); err != nil {
				log.Fatalf("wordlist error: %v", err)
			}
			fmt.Printf("Wordlist: %s (%d keywords, %d bytes)\n",
				analyzeWordlist, len(report.Keywords), len(data))
		}

		if analyzeExport == "" && analyzeWordlist == "" {
			printAnalysisReport(report)
		}
	},
}

func printAnalysisReport(r *models.AnalysisReport) {
	fmt.Printf("\n=== ODK ANALYSIS ===\n")
	fmt.Printf("Generated: %s  (took %s)\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05"), r.Duration)

	fmt.Printf("── GLOBAL ──\n")
	fmt.Printf("  Directories: %d\n", r.TotalDirs)
	fmt.Printf("  Files:       %d\n", r.TotalFiles)
	fmt.Printf("  Total Size:  %s\n", formatBytes(r.TotalSize))
	fmt.Printf("  Avg files/dir: %.1f\n\n", r.AvgFilesPerDir)

	fmt.Printf("── SIZE BUCKETS ──\n")
	labels := []string{"<1KB", "1-10KB", "10-100KB", "100KB-1MB", "1-10MB", "10-100MB", ">100MB"}
	for _, l := range labels {
		if c, ok := r.SizeBuckets[l]; ok {
			pct := float64(c) * 100 / float64(r.TotalFiles)
			fmt.Printf("  %-12s %8d  (%5.1f%%)\n", l, c, pct)
		}
	}
	fmt.Println()

	fmt.Printf("── CATEGORY × EXTENSION (top 15) ──\n")
	type cePair struct {
		key   string
		count int
	}
	var ceList []cePair
	for k, v := range r.CatExtMatrix {
		ceList = append(ceList, cePair{k, v})
	}
	sort.Slice(ceList, func(i, j int) bool { return ceList[j].count < ceList[i].count })
	n := 15
	if n > len(ceList) {
		n = len(ceList)
	}
	for _, p := range ceList[:n] {
		pct := float64(p.count) * 100 / float64(r.TotalFiles)
		fmt.Printf("  %-25s %8d  (%5.1f%%)\n", p.key, p.count, pct)
	}
	fmt.Println()

	fmt.Printf("── TLD DISTRIBUTION ──\n")
	type tldRow struct {
		tld  string
		info *models.TLDInfo
	}
	var tlds []tldRow
	for t, info := range r.TLDStats {
		tlds = append(tlds, tldRow{t, info})
	}
	sort.Slice(tlds, func(i, j int) bool { return tlds[j].info.Files < tlds[i].info.Files })
	for _, e := range tlds {
		pct := float64(e.info.Files) * 100 / float64(r.TotalFiles)
		fmt.Printf("  .%-4s  %5d dirs  %8d files  (%5.1f%%)  %s\n",
			e.tld, e.info.Directories, e.info.Files, pct, formatBytes(e.info.TotalSize))
	}
	fmt.Println()

	if r.EduBreakdown != nil && r.EduBreakdown.TotalFiles > 0 {
		fmt.Printf("── EDU/AC BREAKDOWN ──\n")
		fmt.Printf("  Total: %d files (%.1f%% of all)\n",
			r.EduBreakdown.TotalFiles,
			float64(r.EduBreakdown.TotalFiles)*100/float64(r.TotalFiles))
		for cat, count := range r.EduBreakdown.Categories {
			pct := float64(count) * 100 / float64(r.EduBreakdown.TotalFiles)
			fmt.Printf("  %-12s %8d  (%5.1f%%)\n", cat, count, pct)
		}
		fmt.Println()
	}

	fmt.Printf("── TOP DOMAINS (top 10) ──\n")
	nd := 10
	if nd > len(r.TopDomains) {
		nd = len(r.TopDomains)
	}
	for _, d := range r.TopDomains[:nd] {
		fmt.Printf("  %-45s %d files\n", d.Domain, d.Count)
	}
	fmt.Println()

	fmt.Printf("── TOP KEYWORDS (top 30) ──\n")
	nk := 30
	if nk > len(r.Keywords) {
		nk = len(r.Keywords)
	}
	for _, kw := range r.Keywords[:nk] {
		fmt.Printf("  %-30s %d\n", kw.Word, kw.Count)
	}
	fmt.Printf("\nTotal unique keywords: %d\n\n", len(r.Keywords))
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().StringVar(&analyzeDBPath, "db", "./odk-reader.db", "database path")
	analyzeCmd.Flags().StringVarP(&analyzeExport, "export", "o", "", "export report to JSON file")
	analyzeCmd.Flags().StringVar(&analyzeWordlist, "wordlist", "", "export wordlist (one word per line)")
	analyzeCmd.Flags().BoolVarP(&analyzeForce, "force", "f", false, "force re-run analysis (needs writer DB)")
}
