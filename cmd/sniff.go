package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opendownload/opendownload/internal/downloader"
	"github.com/opendownload/opendownload/internal/sniffer"
	"github.com/opendownload/opendownload/internal/util"
	"github.com/spf13/cobra"
)

var (
	sniffPort         int
	sniffAutoDownload bool
	caCertPath        string
	caKeyPath         string
)

var sniffCmd = &cobra.Command{
	Use:   "sniff",
	Short: "Start a local proxy to sniff network traffic for video streams",
	Long: `Start a local HTTP/HTTPS proxy that intercepts network traffic and
detects video streams. Configure your browser to use the proxy, then
browse normally. Detected video streams will be displayed and can be
downloaded.`,
	RunE: runSniff,
}

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage the local capture proxy",
}

var proxyRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the local capture proxy",
	Long:  sniffCmd.Long,
	RunE:  runSniff,
}

func init() {
	addProxyRunFlags(sniffCmd)
	addProxyRunFlags(proxyRunCmd)
	proxyCmd.AddCommand(proxyRunCmd)
	rootCmd.AddCommand(sniffCmd)
	rootCmd.AddCommand(proxyCmd)
}

func addProxyRunFlags(cmd *cobra.Command) {
	cmd.Flags().IntVarP(&sniffPort, "port", "p", 9000, "proxy listen port")
	cmd.Flags().BoolVar(&sniffAutoDownload, "auto", false, "automatically download detected streams")
	cmd.Flags().StringVar(&caCertPath, "ca-cert", "", "path to CA certificate for HTTPS interception")
	cmd.Flags().StringVar(&caKeyPath, "ca-key", "", "path to CA private key for HTTPS interception")
}

func runSniff(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nShutting down proxy...")
		cancel()
	}()

	detector := sniffer.NewMediaDetector()
	mediaCh := detector.MediaChannel()

	proxyConfig := sniffer.ProxyConfig{
		Port:     sniffPort,
		CACert:   caCertPath,
		CAKey:    caKeyPath,
		Verbose:  verbose,
		Detector: detector,
	}

	proxy, err := sniffer.NewProxy(proxyConfig)
	if err != nil {
		return fmt.Errorf("failed to create proxy: %w", err)
	}

	go func() {
		for media := range mediaCh {
			handleDetectedMedia(ctx, media)
		}
	}()

	fmt.Printf("Proxy listening on http://127.0.0.1:%d\n", sniffPort)
	fmt.Printf("Configure your browser to use this proxy.\n")
	fmt.Printf("Press Ctrl+C to stop.\n\n")

	return proxy.Start(ctx)
}

func handleDetectedMedia(ctx context.Context, media sniffer.DetectedMedia) {
	typeLabel := strings.ToUpper(media.Type)
	fmt.Printf("[%s] %s\n", typeLabel, media.URL)
	if media.ContentType != "" {
		fmt.Printf("  Content-Type: %s\n", media.ContentType)
	}
	if media.Size > 0 {
		fmt.Printf("  Size: %s\n", util.FormatBytes(media.Size))
	}
	fmt.Println()

	if sniffAutoDownload {
		go func() {
			client := util.NewHTTPClient(util.HTTPClientConfig{
				UserAgent: userAgent,
				Verbose:   verbose,
			})

			outName := util.FilenameFromURL(media.URL)
			outPath := filepath.Join(outputDir, outName)

			eng := downloader.NewHTTPDownloader(client, downloader.HTTPDownloaderConfig{
				Workers: workers,
				Verbose: verbose,
			})

			if err := eng.Download(ctx, media.URL, outPath); err != nil {
				fmt.Fprintf(os.Stderr, "Download failed: %s: %v\n", media.URL, err)
			} else {
				fmt.Printf("Downloaded: %s\n", outPath)
			}
		}()
	}
}
