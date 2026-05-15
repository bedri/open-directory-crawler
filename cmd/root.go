package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "odk",
	Short: "Open Directory Crawler - open directory bulur, crawl eder, kategorize eder",
	Long: `odk (Open Directory Crawler) bir open directory crawler'dir.
Web'deki açık dizinleri bulur, recursive olarak crawl eder,
dosyaları formatına göre kategorize eder ve BadgerDB'ye kaydeder.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is odk.yaml)")
}

func initConfig() {
	if cfgFile != "" {
		return
	}
	if _, err := os.Stat("odk.yaml"); err == nil {
		cfgFile = "odk.yaml"
	}
}
