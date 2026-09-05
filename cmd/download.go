package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/opendownload/opendownload/internal/download"
	"github.com/opendownload/opendownload/internal/extractors"
	"github.com/opendownload/opendownload/internal/media"
	"github.com/opendownload/opendownload/internal/transport"
	"github.com/spf13/cobra"
)

var (
	formatSelection string
	filename        string
	forceOverwrite  bool
	headers         []string
	cookie          string
	cookieFile      string
	browserCookies  string
)

var downloadCmd = &cobra.Command{
	Use: "download [url]", Short: "Download a video from a URL",
	Long: "Download a video from a direct URL, HLS stream (.m3u8), or DASH stream (.mpd).",
	Args: cobra.ExactArgs(1), Aliases: []string{"dl", "d"}, RunE: runDownload,
}

func init() {
	downloadCmd.Flags().StringVarP(&formatSelection, "format", "f", "best", "format selection (best, worst, or format ID)")
	downloadCmd.Flags().StringVar(&filename, "filename", "", "output filename (auto detected if empty)")
	downloadCmd.Flags().BoolVar(&forceOverwrite, "force", false, "replace an existing file after a successful transfer")
	downloadCmd.Flags().StringSliceVar(&headers, "header", []string{}, "custom headers")
	downloadCmd.Flags().StringVar(&cookie, "cookie", "", "cookie string")
	downloadCmd.Flags().StringVar(&cookieFile, "cookie-file", "", "Netscape formatted cookie file")
	downloadCmd.Flags().StringVar(&browserCookies, "cookies-from-browser", "", "load cookies from browser (chrome or edge)")
	rootCmd.AddCommand(downloadCmd)
}

func runDownload(cmd *cobra.Command, args []string) error {
	streamURL := args[0]
	requestHeaders, requestCookie, selectedFilename := parseHeaders(headers), cookie, filename
	if extractor := extractors.GetExtractor(streamURL); extractor != nil {
		fmt.Printf("Using extractor: %s\n", extractor.Name())
		if result, err := extractor.Extract(cmd.Context(), streamURL); err == nil {
			streamURL = result.StreamURL
			if selectedFilename == "" && result.Title != "" {
				selectedFilename = result.Title
			}
			for key, value := range result.Headers {
				requestHeaders[key] = value
			}
			if result.Cookies != "" {
				requestCookie = result.Cookies
			}
		}
	}
	if browserCookies != "" {
		if host := media.HostFromURL(streamURL); host != "" {
			if loaded, err := transport.LoadCookiesFromBrowser(browserCookies, host); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not load browser cookies: %v\n", err)
			} else {
				requestCookie = loaded
				fmt.Printf("Loaded cookies for %s\n", host)
			}
		}
	}
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		select {
		case <-signals:
			fmt.Fprintln(os.Stderr, "Interrupted. Cancelling download.")
			cancel()
		case <-ctx.Done():
		}
	}()
	service := download.NewService(download.Config{DefaultOutputDir: func() (string, error) { return outputDir, nil }, DefaultWorkers: workers})
	stop := service.Subscribe(printDownloadSnapshot)
	defer stop()
	snapshot, err := service.Queue(ctx, download.Request{ID: "cli", URL: streamURL, OutputDir: outputDir, Filename: selectedFilename, Format: formatSelection, Headers: requestHeaders, Cookie: requestCookie, CookieFile: cookieFile, UserAgent: userAgent, ProxyURL: proxyURL, Workers: workers, Overwrite: forceOverwrite})
	if err != nil {
		return err
	}
	fmt.Printf("Output: %s\n", snapshot.OutputPath)
	completed, err := service.Wait(ctx, snapshot.ID)
	if err != nil {
		return err
	}
	if completed.Status != download.StatusCompleted {
		if completed.Message != "" {
			return fmt.Errorf("download %s: %s", completed.Status, completed.Message)
		}
		return fmt.Errorf("download %s", completed.Status)
	}
	fmt.Printf("Completed: %s\n", completed.OutputPath)
	return nil
}

func parseHeaders(values []string) map[string]string {
	parsed := make(map[string]string)
	for _, value := range values {
		parts := strings.SplitN(value, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
			parsed[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return parsed
}

func printDownloadSnapshot(snapshot download.JobSnapshot) {
	if snapshot.Status != download.StatusDownloading || snapshot.TotalBytes <= 0 {
		return
	}
	percent := float64(snapshot.DownloadedBytes) * 100 / float64(snapshot.TotalBytes)
	fmt.Printf("\r%.0f%%  %s of %s  %s/s", percent, media.FormatBytes(snapshot.DownloadedBytes), media.FormatBytes(snapshot.TotalBytes), media.FormatBytes(int64(snapshot.BytesPerSecond)))
}
