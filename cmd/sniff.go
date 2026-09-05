package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/opendownload/opendownload/internal/download"
	"github.com/opendownload/opendownload/internal/media"
	"github.com/opendownload/opendownload/internal/sniffer"
	"github.com/spf13/cobra"
)

var (
	sniffPort         int
	sniffAutoDownload bool
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

func handleDetectedMedia(ctx context.Context, detected sniffer.DetectedMedia) {
	typeLabel := strings.ToUpper(detected.Type)
	fmt.Printf("[%s] %s\n", typeLabel, detected.URL)
	if detected.ContentType != "" {
		fmt.Printf("  Content-Type: %s\n", detected.ContentType)
	}
	if detected.Size > 0 {
		fmt.Printf("  Size: %s\n", media.FormatBytes(detected.Size))
	}
	fmt.Println()

	if sniffAutoDownload {
		go func() {
			service := download.NewService(download.Config{DefaultOutputDir: func() (string, error) { return outputDir, nil }, DefaultWorkers: workers})
			snapshot, err := service.Queue(ctx, download.Request{ID: "detected_" + detected.Type + "_" + fmt.Sprint(detected.Size), URL: detected.URL, OutputDir: outputDir, UserAgent: userAgent, Workers: workers})
			if err == nil {
				snapshot, err = service.Wait(ctx, snapshot.ID)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Download failed: %s: %v\n", detected.URL, err)
			} else if snapshot.Status == download.StatusCompleted {
				fmt.Printf("Downloaded: %s\n", snapshot.OutputPath)
			}
		}()
	}
}
