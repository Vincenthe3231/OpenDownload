package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	outputDir  string
	workers    int
	userAgent  string
	proxyURL   string
	verbose    bool
	configFile string
)

var rootCmd = &cobra.Command{
	Use:   "opendownload",
	Short: "A network traffic interception and video downloading CLI tool",
	Long: `OpenDownload is a CLI tool that intercepts network traffic to detect
and download video streams. It supports direct HTTP downloads, HLS/DASH
streams, and can sniff network traffic through a local proxy to
auto-detect downloadable media.`,
}

func initConfig() error {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
			viper.AddConfigPath(".")
			viper.SetConfigName(".opendownload")
			viper.SetConfigType("yaml")
		}
	}

	viper.SetEnvPrefix("OD")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}

	// Bind flags
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("workers", rootCmd.PersistentFlags().Lookup("workers"))
	viper.BindPFlag("proxy", rootCmd.PersistentFlags().Lookup("proxy"))
	viper.BindPFlag("user-agent", rootCmd.PersistentFlags().Lookup("user-agent"))

	// Sync variables
	outputDir = viper.GetString("output")
	workers = viper.GetInt("workers")
	proxyURL = viper.GetString("proxy")
	userAgent = viper.GetString("user-agent")

	return nil
}

func Execute() error {
	if err := initConfig(); err != nil {
		return err
	}
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputDir, "output", "o", "./download", "output directory for downloads")
	rootCmd.PersistentFlags().IntVarP(&workers, "workers", "w", 8, "number of concurrent download workers")
	rootCmd.PersistentFlags().StringVarP(&userAgent, "user-agent", "U", "", "custom User-Agent string")
	rootCmd.PersistentFlags().StringVarP(&proxyURL, "proxy", "x", "", "proxy URL (e.g. http://127.0.0.1:9000)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path")

	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpTemplate(fmt.Sprintf(`%s
Examples:
  opendownload download https://example.com/video.mp4
  opendownload download https://example.com/stream.m3u8
  opendownload info https://example.com/stream.m3u8
  opendownload sniff --port 9000

`, rootCmd.HelpTemplate()))
}
