package cmd

import (
	"fmt"
	"log"

	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/spf13/cobra"
)

var statsDBPath string

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Veritabanı istatistiklerini göster",
	Run: func(cmd *cobra.Command, args []string) {
		store, err := storage.NewReadOnly(statsDBPath)
		if err != nil {
			log.Fatalf("storage error: %v", err)
		}
		defer store.Close()

		st, err := store.GetStats()
		if err != nil {
			log.Fatalf("stats error: %v", err)
		}

		fmt.Printf("=== ODK Statistics ===\n\n")
		fmt.Printf("Total Directories: %d\n", st.TotalDirectories)
		fmt.Printf("Total Files:       %d\n", st.TotalFiles)
		fmt.Printf("Total Size:        %s\n\n", formatBytes(st.TotalSize))

		fmt.Println("Categories:")
		for cat, count := range st.CategoryCounts {
			fmt.Printf("  %-12s %d files\n", cat, count)
		}

		fmt.Println("\nTop Extensions:")
		limit := 20
		i := 0
		for ext, count := range st.ExtCounts {
			if i >= limit {
				break
			}
			fmt.Printf("  .%-10s %d\n", ext, count)
			i++
		}
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().StringVar(&statsDBPath, "db", "./odk.db", "database path")
}
