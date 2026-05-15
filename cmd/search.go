package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/spf13/cobra"
)

var (
	searchDBPath string
	searchRegex  bool
	searchJSON   bool
	searchExport string
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Dosyalarda isim araması yap",
	Long: `Dosya adlarında arama yapar. Varsayılan: case-insensitive substring.
--regex ile regex desteği, --json ile JSON çıktı, --export ile dosyaya kaydet.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		store, err := storage.NewReadOnly(searchDBPath)
		if err != nil {
			log.Fatalf("storage error: %v", err)
		}
		defer store.Close()

		query := args[0]
		var re *regexp.Regexp
		if searchRegex {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			type regexResult struct {
				re  *regexp.Regexp
				err error
			}
			ch := make(chan regexResult, 1)
			go func() {
				r, e := regexp.Compile(query)
				ch <- regexResult{r, e}
			}()
			select {
			case res := <-ch:
				re, err = res.re, res.err
				if err != nil {
					log.Fatalf("invalid regex: %v", err)
				}
			case <-ctx.Done():
				log.Fatalf("regex compilation timed out (possible ReDoS): %s", query)
			}
		} else {
			query = strings.ToLower(query)
		}

		dirs, err := store.ListDirectories()
		if err != nil {
			log.Fatalf("list error: %v", err)
		}

		var matched []fileResult
		for _, d := range dirs {
			files, err := store.GetFilesByDir(d.ID)
			if err != nil {
				continue
			}
			for _, f := range files {
				match := false
				if searchRegex {
					match = re.MatchString(f.Name)
				} else {
					match = strings.Contains(strings.ToLower(f.Name), query)
				}
				if match {
					matched = append(matched, fileResult{FileEntry: f, DirURL: d.URL})
				}
			}
		}

		if searchExport != "" {
			if err := exportJSONGeneric(matched, searchExport); err != nil {
				log.Fatalf("export error: %v", err)
			}
			fmt.Printf("Exported %d results to %s\n", len(matched), searchExport)
			return
		}

		if searchJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(matched)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintf(w, "NAME\tSIZE\tEXT\tCAT\tDIR\n")
		fmt.Fprintf(w, "----\t----\t---\t---\t---\n")
		for _, m := range matched {
			fmt.Fprintf(w, "%s\t%s\t.%s\t%s\t%s\n",
				m.FileEntry.Name, formatBytes(m.FileEntry.Size), m.FileEntry.Ext, m.FileEntry.Category, m.DirURL)
		}
		w.Flush()
		fmt.Printf("\n%d files found\n", len(matched))
	},
}

type fileResult struct {
	*models.FileEntry
	DirURL string `json:"dir_url"`
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().StringVar(&searchDBPath, "db", "./odk.db", "database path")
	searchCmd.Flags().BoolVarP(&searchRegex, "regex", "e", false, "use regex matching")
	searchCmd.Flags().BoolVarP(&searchJSON, "json", "j", false, "JSON format output")
	searchCmd.Flags().StringVarP(&searchExport, "export", "o", "", "export to .json file")
}
