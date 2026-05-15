package cmd

import (
	"log"
	"net/http"

	"github.com/bedri/open-directory-crawler/internal/portal"
	"github.com/bedri/open-directory-crawler/internal/storage"
	"github.com/spf13/cobra"
)

var (
	portalDBPath string
	portalAddr   string
)

var portalCmd = &cobra.Command{
	Use:   "portal",
	Short: "Public search engine for open directory files",
	Long: `Public-facing arama motoru. Kullanıcılar dosya arayabilir,
kategoriye göre gezinebilir, müzik/video oynatabilir, PDF/Excel görüntüleyebilir.

Kullanım:
  odk portal
  odk portal --addr :8080 --db ./odk-reader.db`,
	Run: func(cmd *cobra.Command, args []string) {
		store, err := storage.NewReadOnly(portalDBPath)
		if err != nil {
			log.Printf("reader DB (%s) not found, trying main DB", portalDBPath)
			store, err = storage.NewReadOnly("./odk.db")
			if err != nil {
				log.Fatalf("storage error: %v", err)
			}
		}
		defer store.Close()

		handler := portal.New(store)

		log.Printf("Portal running on %s", portalAddr)
		log.Printf("Search engine ready — open http://localhost%s", portalAddr)

		if err := http.ListenAndServe(portalAddr, handler); err != nil {
			log.Fatalf("server error: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(portalCmd)
	portalCmd.Flags().StringVar(&portalDBPath, "db", "./odk-reader.db", "database path (reader)")
	portalCmd.Flags().StringVarP(&portalAddr, "addr", "a", ":40445", "listen address")
}
