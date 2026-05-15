package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/spf13/cobra"
)

var (
	importKeywordsDB   string
	importKeywordsFile string
)

var (
	importDBPath string
	importFile   string
	importOutput string
	importDirect bool
)

var importKeywordsCmd = &cobra.Command{
	Use:   "keywords",
	Short: "MySQL dump'tan keywordbot tablosunu wordlist'e import et",
	Long: `MySQL dump dosyasından keywordbot tablosundaki keyword'leri çıkarır
ve ODK wordlist'ine kaydeder (autocomplete + analysis için).

Kullanım:
  odk import keywords --file hoopss_all_sitelinks.sql --db ./odk.db`,
	Run: func(cmd *cobra.Command, args []string) {
		if importKeywordsFile == "" {
			fmt.Println("Hata: --file gerekli")
			os.Exit(1)
		}
		if importKeywordsDB == "" {
			fmt.Println("Hata: --db gerekli")
			os.Exit(1)
		}

		fmt.Printf("Keyword dosyası açılıyor: %s\n", importKeywordsFile)
		f, err := os.Open(importKeywordsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Dosya açma hatası: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		seen := make(map[string]bool)
		var total int
		sc := bufio.NewScanner(f)
		buf := make([]byte, 2*1024*1024)
		sc.Buffer(buf, len(buf))

		for sc.Scan() {
			line := sc.Text()
			if !strings.Contains(line, "INSERT INTO `keywordbot`") {
				continue
			}
			tuples := extractTuples(line)
			for _, tuple := range tuples {
				kw := extractKeywordFromTuple(tuple)
				if kw != "" && !seen[kw] {
					seen[kw] = true
					total++
				}
			}
		}
		if err := sc.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Okuma hatası: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Toplam %d benzersiz keyword bulundu.\n", total)

		store, err := storage.New(importKeywordsDB)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Veritabanı açma hatası: %v\n", err)
			os.Exit(1)
		}
		defer store.Close()

		var sb strings.Builder
		for kw := range seen {
			sb.WriteString(kw)
			sb.WriteByte('\n')
		}

		if err := store.SaveWordlist([]byte(sb.String())); err != nil {
			fmt.Fprintf(os.Stderr, "Wordlist kaydetme hatası: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wordlist kaydedildi (%d keyword).\n", total)
	},
}

func extractKeywordFromTuple(tuple string) string {
	inStr := false
	escape := false
	fieldIdx := 0

	for i := 0; i < len(tuple); i++ {
		ch := tuple[i]

		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inStr {
			escape = true
			continue
		}
		if ch == '\'' {
			if !inStr {
				inStr = true
				start := i + 1
				for j := start; j < len(tuple); j++ {
					if tuple[j] == '\'' {
						if j+1 < len(tuple) && tuple[j+1] == '\'' {
							j++
							continue
						}
						val := strings.TrimSpace(tuple[start:j])
						if fieldIdx == 1 && val != "" {
							return val
						}
						fieldIdx++
						break
					}
				}
				inStr = false
			}
			continue
		}
		if !inStr && ch == ',' {
			fieldIdx++
		}
	}
	return ""
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "External veri kaynaklarını import et",
}

var importSQLCmd = &cobra.Command{
	Use:   "sql",
	Short: "MySQL dump dosyasından URL'leri import et",
	Long: `MySQL dump dosyasını stream okuyup INSERT'lerden URL'leri çıkarır.
Tablo adına göre kategorize eder, benzersiz dizinleri JSON olarak kaydeder.

Kullanım:
  odk import sql --file hoopss.sql --output dirs.json
  odk import sql --file hoopss.sql --db ./odk.db (direkt veritabanına kaydet)`,
	Run: func(cmd *cobra.Command, args []string) {
		if importFile == "" {
			fmt.Println("Hata: --file gerekli")
			os.Exit(1)
		}
		if importOutput == "" && importDBPath == "" {
			fmt.Println("Hata: --output veya --db gerekli")
			os.Exit(1)
		}

		fmt.Printf("SQL dosyası açılıyor: %s\n", importFile)
		f, err := os.Open(importFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Dosya açma hatası: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		tableCat := map[string]string{
			"android":                    "archive",
			"android_google_site_links":  "archive",
			"archive":                    "archive",
			"archive_ftp":                "archive",
			"archive_google_site_links":  "archive",
			"archive_mamont_site_links":  "archive",
			"document":                   "document",
			"document_ftp":               "document",
			"document_google_site_links": "document",
			"document_mamont_site_links": "document",
			"image":                      "image",
			"music_unique":               "audio",
			"music_ftp":                  "audio",
			"music_google_site_links":    "audio",
			"music_mamont_site_links":    "audio",
			"soundclick":                 "audio",
			"video":                      "video",
			"video_ftp":                  "video",
			"video_google_site_links":    "video",
			"video_mamont_site_links":    "video",
			"torrent":                    "torrent",
			"torrent_google_site_links":  "torrent",
			"youtube":                    "video",
		}

		sc := bufio.NewScanner(f)
		buf := make([]byte, 2*1024*1024)
		sc.Buffer(buf, len(buf))

		uniqueDirs := make(map[string]*models.ImportedDir)
		lineNum := 0
		insertCount := 0

		for sc.Scan() {
			line := sc.Text()
			lineNum++

			if !strings.HasPrefix(line, "INSERT INTO") {
				continue
			}
			insertCount++

			tableName := extractTableName(line)
			if tableName == "" {
				continue
			}
			cat, ok := tableCat[tableName]
			if !ok {
				cat = "other"
			}

			tuples := extractTuples(line)
			for _, tuple := range tuples {
				urlStr := extractURLFromTuple(tuple)
				if urlStr == "" {
					continue
				}
				dirURL := parentDirURL(urlStr)
				if dirURL == "" {
					continue
				}
				if _, exists := uniqueDirs[dirURL]; !exists {
					uniqueDirs[dirURL] = &models.ImportedDir{
						URL:      dirURL,
						Category: cat,
						Source:   "hoopss",
					}
				}
			}

			if lineNum%100000 == 0 {
				fmt.Printf("  %d satır okundu, %d INSERT işlendi, %d benzersiz dizin\n", lineNum, insertCount, len(uniqueDirs))
			}
		}

		if err := sc.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Okuma hatası: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nToplam: %d satır, %d INSERT, %d benzersiz dizin\n", lineNum, insertCount, len(uniqueDirs))

		result := make([]models.ImportedDir, 0, len(uniqueDirs))
		for _, d := range uniqueDirs {
			result = append(result, *d)
		}

		if importOutput != "" {
			out, err := os.Create(importOutput)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Çıktı dosyası oluşturma hatası: %v\n", err)
				os.Exit(1)
			}
			defer out.Close()

			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				fmt.Fprintf(os.Stderr, "JSON yazma hatası: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("JSON kaydedildi: %s (%d dizin)\n", importOutput, len(result))
		}

		if importDBPath != "" {
			fmt.Printf("Veritabanına kaydediliyor: %s\n", importDBPath)
			store, err := storage.New(importDBPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Veritabanı açma hatası: %v\n", err)
				os.Exit(1)
			}
			defer store.Close()

			saved := 0
			skipped := 0
			for _, d := range result {
				dir := dirFromURL(d.URL)
				existing, err := store.GetDirectory(dir.ID)
				if err == nil && existing != nil {
					skipped++
					continue
				}
				if err := store.SaveDirectory(dir); err == nil {
					saved++
				}
			}
			fmt.Printf("Veritabanına kaydedildi: %d (atlanan: %d)\n", saved, skipped)
		}
	},
}

func extractTableName(line string) string {
	idx := strings.Index(line, "`")
	if idx < 0 {
		return ""
	}
	end := strings.Index(line[idx+1:], "`")
	if end < 0 {
		return ""
	}
	return line[idx+1 : idx+1+end]
}

func extractTuples(line string) []string {
	valsIdx := strings.Index(line, "VALUES")
	if valsIdx < 0 {
		valsIdx = strings.Index(line, "values")
	}
	if valsIdx < 0 {
		return nil
	}

	rest := line[valsIdx+6:]
	rest = strings.TrimSpace(rest)

	var tuples []string
	depth := 0
	start := 0
	inStr := false
	escape := false

	for i := 0; i < len(rest); i++ {
		ch := rest[i]

		if escape {
			escape = false
			continue
		}

		if ch == '\\' && inStr {
			escape = true
			continue
		}

		if ch == '\'' {
			inStr = !inStr
			continue
		}

		if inStr {
			continue
		}

		if ch == '(' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				tuples = append(tuples, rest[start+1:i])
			}
		}
	}

	return tuples
}

func extractURLFromTuple(tuple string) string {
	inStr := false
	escape := false
	fieldIdx := 0
	start := -1

	for i := 0; i < len(tuple); i++ {
		ch := tuple[i]

		if escape {
			escape = false
			continue
		}

		if ch == '\\' && inStr {
			escape = true
			continue
		}

		if ch == '\'' {
			if !inStr {
				if fieldIdx == 1 {
					start = i + 1
				}
				inStr = true
			} else {
				inStr = false
				if fieldIdx == 1 && start >= 0 {
					return tuple[start:i]
				}
				fieldIdx++
			}
			continue
		}

		if !inStr && ch == ',' {
			fieldIdx++
		}
	}

	return ""
}

func parentDirURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" {
		return ""
	}
	dir := path.Dir(u.Path)
	if dir == "." {
		dir = "/"
	}
	u.Path = dir
	if !strings.HasSuffix(u.String(), "/") {
		u.Path = dir + "/"
	}
	return u.String()
}

func init() {
	importSQLCmd.Flags().StringVarP(&importFile, "file", "f", "", "SQL dump dosyası")
	importSQLCmd.Flags().StringVarP(&importOutput, "output", "o", "", "Çıktı JSON dosyası")
	importSQLCmd.Flags().StringVar(&importDBPath, "db", "", "Direkt veritabanına kaydet (opsiyonel)")
	importCmd.AddCommand(importSQLCmd)
	importKeywordsCmd.Flags().StringVarP(&importKeywordsFile, "file", "f", "", "SQL dump dosyası")
	importKeywordsCmd.Flags().StringVarP(&importKeywordsDB, "db", "d", "", "ODK BadgerDB yolu")
	importCmd.AddCommand(importKeywordsCmd)
	rootCmd.AddCommand(importCmd)
}
