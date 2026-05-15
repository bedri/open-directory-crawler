package cmd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/bedri/open-directory-crawler/internal/discover"
	"github.com/bedri/open-directory-crawler/internal/models"
	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/spf13/cobra"
)

var (
	discoverDBPath string
	discoverAll    bool
	discoverCheck  bool
	discoverOutput string
	discoverType   string
	discoverImport string
)

var discoverCmd = &cobra.Command{
	Use:   "discover [url]",
	Short: "Open directory keşfet (Google, Bing, aggregator siteler)",
	Long: `Açık dizinleri çoklu kaynaktan keşfeder.
--all: Google dork + Bing + aggregator siteleri tara.
--check: URL'nin open directory olup olmadığını kontrol eder.
--output: Sonuçları JSON dosyaya kaydeder.

Örnek:
  odk discover --all
  odk discover --all --type audio
  odk discover --all --type video --output videolar.json
  odk discover --import liste.txt
  odk discover --check http://ornek.com/
  odk discover --all --output bulunanlar.json
  odk discover http://ornek.com/`,
	Run: func(cmd *cobra.Command, args []string) {
		finder := discover.New()
		var allResults []discover.Result

		if discoverImport != "" {
			fmt.Printf("URL listesi import ediliyor: %s\n", discoverImport)
			data, err := os.ReadFile(discoverImport)
			if err != nil {
				log.Fatalf("import error: %v", err)
			}
			store, err := storage.New(discoverDBPath)
			if err != nil {
				log.Fatalf("storage error: %v", err)
			}
			defer store.Close()

			trimmed := strings.TrimSpace(string(data))
			saved := 0

			if strings.HasPrefix(trimmed, "[") {
				var dirs []models.ImportedDir
				if err := json.Unmarshal([]byte(trimmed), &dirs); err != nil {
					log.Fatalf("JSON parse error: %v", err)
				}
				for _, d := range dirs {
					dir := dirFromURL(d.URL)
					if err := store.SaveDirectory(dir); err == nil {
						saved++
					}
				}
			} else {
				for _, line := range strings.Split(trimmed, "\n") {
					line = strings.TrimSpace(line)
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					dir := dirFromURL(line)
					if err := store.SaveDirectory(dir); err == nil {
						saved++
					}
				}
			}

			fmt.Printf("%d URL veritabanına kaydedildi.\n", saved)
			fmt.Println("Crawl etmek için: odk crawl <url>")
			return
		}

		if discoverAll {
			if discoverType != "" {
				fmt.Printf("Open directory taranıyor (type: %s)...\n", discoverType)
				fmt.Println("   Kaynaklar: Google dork + Bing (type-specific)")
			} else {
				fmt.Println("Open directory taranıyor...")
				fmt.Println("   Kaynaklar: Google dork, Bing, aggregator siteler")
			}

			var err error
			if discoverType != "" {
				allResults, err = finder.DiscoverByType(discoverType)
			} else {
				allResults, err = finder.DiscoverAll()
			}
			if err != nil {
				log.Fatalf("discover error: %v", err)
			}

			if len(allResults) == 0 {
				fmt.Println("Hiç open directory bulunamadı.")
				return
			}

			fmt.Printf("Bulunan %d open directory:\n\n", len(allResults))
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintf(w, "URL\tKAYNAK\n")
			fmt.Fprintf(w, "---\t------\n")
			for _, r := range allResults {
				fmt.Fprintf(w, "%s\t%s\n", r.URL, r.Source)
			}
			w.Flush()

			store, err := storage.New(discoverDBPath)
			if err != nil {
				log.Fatalf("storage error: %v", err)
			}
			defer store.Close()

			saved := 0
			for _, r := range allResults {
				dir := dirFromURL(r.URL)
				if err := store.SaveDirectory(dir); err == nil {
					saved++
				}
			}
			fmt.Printf("\n%d sonuç veritabanına kaydedildi.\n", saved)

		} else if discoverCheck {
			if len(args) == 0 {
				log.Fatal("url required with --check flag")
			}
			if finder.IsOpenDirectory(args[0]) {
				fmt.Println("[OPEN] Bu bir open directory!")
				paths := finder.SearchCommonPaths(args[0])
				for _, p := range paths {
					fmt.Printf("  Alt dizin: %s\n", p)
				}
			} else {
				fmt.Println("[CLOSED] Open directory değil veya erişilemiyor")
			}

		} else if len(args) > 0 {
			if finder.IsOpenDirectory(args[0]) {
				fmt.Printf("[OPEN] %s\n", args[0])
			} else {
				fmt.Printf("[CLOSED] %s\n", args[0])
			}
		} else {
			cmd.Help()
		}

		if discoverOutput != "" && len(allResults) > 0 {
			if err := finder.SaveResults(allResults, discoverOutput); err != nil {
				log.Fatalf("save error: %v", err)
			}
			fmt.Printf("Sonuçlar kaydedildi: %s\n", discoverOutput)
		}
	},
}

func urlToID(rawURL string) string {
	u, _ := url.Parse(rawURL)
	var sum [32]byte
	if u == nil {
		sum = sha256.Sum256([]byte(rawURL))
		return fmt.Sprintf("%x", sum[:8])
	}
	key := u.Host + u.Path
	sum = sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", sum[:8])
}

func dirFromURL(rawURL string) *models.Directory {
	return &models.Directory{
		ID:     urlToID(rawURL),
		URL:    rawURL,
		Status: models.StatusPending,
	}
}

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.Flags().StringVar(&discoverDBPath, "db", "./odk.db", "database path")
	discoverCmd.Flags().BoolVarP(&discoverAll, "all", "a", false, "tüm kaynaklardan open directory tara")
	discoverCmd.Flags().BoolVarP(&discoverCheck, "check", "c", false, "URL'nin open directory olup olmadığını kontrol et")
	discoverCmd.Flags().StringVarP(&discoverOutput, "output", "o", "", "sonuçları JSON dosyaya kaydet")
	discoverCmd.Flags().StringVarP(&discoverType, "type", "t", "", "içerik tipine göre filtrele (audio, video, image, document, archive, code)")
	discoverCmd.Flags().StringVarP(&discoverImport, "import", "i", "", "URL listesi içeren dosyayı import et")
}
