package cmd

import (
	"fmt"
	"log"

	"github.com/bedri/open-directory-crawler/internal/soundclick"
	"github.com/spf13/cobra"
)

var (
	soundclickKeyword string
	soundclickLimit   int
	soundclickDetail  bool
)

var soundclickCmd = &cobra.Command{
	Use:   "soundclick",
	Short: "SoundClick'te müzik ara ve MP3 URL'lerini bul",
	Long: `SoundClick müzik platformunda arama yapar ve MP3 dosya URL'lerini listeler.

Kullanım:
  odk soundclick --keyword "trance"
  odk soundclick --keyword "progressive house" --limit 20`,
	Run: func(cmd *cobra.Command, args []string) {
		if soundclickKeyword == "" {
			fmt.Println("Hata: --keyword gerekli")
			cmd.Help()
			return
		}

		client := soundclick.New()
		tracks, err := client.Search(soundclickKeyword, soundclickLimit)
		if err != nil {
			log.Fatalf("SoundClick arama hatası: %v", err)
		}

		if len(tracks) == 0 {
			fmt.Println("Sonuç bulunamadı.")
			return
		}

		fmt.Printf("\nSoundClick'te \"%s\" için %d sonuç:\n\n", soundclickKeyword, len(tracks))
		for i, t := range tracks {
			audioURL := t.AudioURL
			if audioURL == "" && soundclickDetail {
				info, err := client.GetTrackInfo(t.SongID)
				if err == nil {
					audioURL = info.AudioURL
					if t.Title == "" {
						t = *info
					}
				}
				if audioURL == "" {
					u, err := client.GetAudioURL(t.SongID)
					if err == nil {
						audioURL = u
					}
				}
			}
			artistTitle := t.Artist
			if artistTitle != "" {
				artistTitle += " - "
			}
			artistTitle += t.Title
			if artistTitle == " - " {
				artistTitle = fmt.Sprintf("Song #%d", t.SongID)
			}

			fmt.Printf("%3d. %s\n", i+1, artistTitle)
			if t.Genre != "" {
				fmt.Printf("     Genre: %s\n", t.Genre)
			}
			if audioURL != "" {
				fmt.Printf("     URL: %s\n", audioURL)
			} else {
				fmt.Printf("     Page: %s\n", t.PageURL)
			}
			fmt.Println()
		}
	},
}

func init() {
	rootCmd.AddCommand(soundclickCmd)
	soundclickCmd.Flags().StringVarP(&soundclickKeyword, "keyword", "k", "", "Aranacak kelime")
	soundclickCmd.Flags().IntVarP(&soundclickLimit, "limit", "l", 20, "Maksimum sonuç sayısı")
	soundclickCmd.Flags().BoolVarP(&soundclickDetail, "detail", "d", false, "Detaylı bilgi (sayfa scraping ile audio URL bul)")
}
