package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"text/tabwriter"

	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/spf13/cobra"
)

var (
	listDBPath  string
	listByExt   string
	listByCat   string
	listDirID   string
	listJSON    bool
	listExport  string
	listMinSize int64
	listMaxSize int64
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Dosyaları listele, filtrele, dışa aktar",
	Long: `Kategorize edilmiş dosyaları listeler.
Filtreleme: --cat, --ext, --dir, --min-size, --max-size
Çıktı: --json (stdout), --export (dosyaya .json veya .csv)`,
	Run: func(cmd *cobra.Command, args []string) {
		store, err := storage.New(listDBPath)
		if err != nil {
			log.Fatalf("storage error: %v", err)
		}
		defer store.Close()

		var files []*models.FileEntry

		switch {
		case listByExt != "":
			files, err = store.GetFilesByExt(listByExt)
		case listByCat != "":
			files, err = store.GetFilesByCategory(models.FileCategory(listByCat))
		case listDirID != "":
			files, err = store.GetFilesByDir(listDirID)
		default:
			dirs, listErr := store.ListDirectories()
			if listErr != nil {
				log.Fatalf("list error: %v", listErr)
			}
			for _, d := range dirs {
				df, _ := store.GetFilesByDir(d.ID)
				files = append(files, df...)
			}
		}
		if err != nil {
			log.Fatalf("query error: %v", err)
		}

		files = filterBySize(files, listMinSize, listMaxSize)

		if listExport != "" {
			if err := exportFiles(files, listExport); err != nil {
				log.Fatalf("export error: %v", err)
			}
			fmt.Printf("Exported %d files to %s\n", len(files), listExport)
			return
		}

		if listJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(files)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintf(w, "NAME\tSIZE\tEXT\tCAT\tURL\n")
		fmt.Fprintf(w, "----\t----\t---\t---\t---\n")
		for _, f := range files {
			fmt.Fprintf(w, "%s\t%s\t.%s\t%s\t%s\n",
				f.Name, formatBytes(f.Size), f.Ext, f.Category, f.URL)
		}
		w.Flush()
		fmt.Printf("\n%d files\n", len(files))
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&listDBPath, "db", "./odk.db", "database path")
	listCmd.Flags().StringVar(&listByExt, "ext", "", "filter by extension (.mp4, pdf, etc.)")
	listCmd.Flags().StringVar(&listByCat, "cat", "", "filter by category (video, audio, image, document, archive, code)")
	listCmd.Flags().StringVar(&listDirID, "dir", "", "filter by directory ID")
	listCmd.Flags().BoolVarP(&listJSON, "json", "j", false, "JSON format output")
	listCmd.Flags().StringVarP(&listExport, "export", "o", "", "export to file (.json or .csv)")
	listCmd.Flags().Int64Var(&listMinSize, "min-size", 0, "minimum file size in bytes")
	listCmd.Flags().Int64Var(&listMaxSize, "max-size", 0, "maximum file size in bytes (0 = no limit)")
}

func filterBySize(files []*models.FileEntry, min, max int64) []*models.FileEntry {
	if min == 0 && max == 0 {
		return files
	}
	var result []*models.FileEntry
	for _, f := range files {
		if min > 0 && f.Size < min {
			continue
		}
		if max > 0 && f.Size > max {
			continue
		}
		result = append(result, f)
	}
	return result
}

func exportFiles(files []*models.FileEntry, path string) error {
	ext := filepath.Ext(path)
	switch ext {
	case ".json":
		return exportJSON(files, path)
	case ".csv":
		return exportCSV(files, path)
	default:
		return fmt.Errorf("unsupported format: %s (use .json or .csv)", ext)
	}
}

func exportJSON(files []*models.FileEntry, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(files)
}

func exportCSV(files []*models.FileEntry, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"Name", "URL", "Size", "Extension", "Category", "DirectoryID"})
	for _, fe := range files {
		w.Write([]string{
			fe.Name,
			fe.URL,
			strconv.FormatInt(fe.Size, 10),
			fe.Ext,
			string(fe.Category),
			fe.DirectoryID,
		})
	}
	return nil
}

func exportJSONGeneric(v any, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
