package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opendownload/opendownload/server/internal/downloader"
	"github.com/opendownload/opendownload/server/internal/extractors"
	"github.com/opendownload/opendownload/server/internal/parser"
	"github.com/opendownload/opendownload/server/internal/util"
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
	Use:   "download [url]",
	Short: "Download a video from a URL",
	Long:  `Download a video from a direct URL, HLS stream (.m3u8), or DASH stream (.mpd).`,
	Args:  cobra.ExactArgs(1),
	Aliases: []string{"dl", "d"},
	RunE:  runDownload,
}

func init() {
	downloadCmd.Flags().StringVarP(&formatSelection, "format", "f", "best", "format selection (best, worst, or format ID)")
	downloadCmd.Flags().StringVar(&filename, "filename", "", "output filename (auto-detected if empty)")
	downloadCmd.Flags().BoolVar(&forceOverwrite, "force", false, "overwrite existing files")
	downloadCmd.Flags().StringSliceVar(&headers, "header", []string{}, "custom headers (e.g. 'Referer: https://example.com')")
	downloadCmd.Flags().StringVar(&cookie, "cookie", "", "cookie string (e.g. 'key=value; key2=value2')")
	downloadCmd.Flags().StringVar(&cookieFile, "cookie-file", "", "Netscape formatted cookie file")
	downloadCmd.Flags().StringVar(&browserCookies, "cookies-from-browser", "", "load cookies from browser (chrome|edge)")
	rootCmd.AddCommand(downloadCmd)
}

func runDownload(cmd *cobra.Command, args []string) error {
	url := args[0]
	finalCookie := cookie
	finalHeaders := parseHeaders(headers)
	var detectedTitle string

	// Attempt automatic extraction
	if e := extractors.GetExtractor(url); e != nil {
		fmt.Printf("Using extractor: %s\n", e.Name())
		res, err := e.Extract(cmd.Context(), url)
		if err == nil {
			url = res.StreamURL
			detectedTitle = res.Title
			for k, v := range res.Headers {
				finalHeaders[k] = v
			}
			if res.Cookies != "" {
				finalCookie = res.Cookies
			}
		}
	}

	if browserCookies != "" {
		host := util.GetHostFromURL(url)
		if host != "" {
			c, err := util.LoadCookiesFromBrowser(browserCookies, host)
			if err == nil {
				finalCookie = c
				fmt.Printf("Loaded cookies for %s\n", host)
			} else {
				fmt.Printf("Warning: failed to load cookies: %v\n", err)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nInterrupted, cleaning up...")
		cancel()
	}()

	client := util.NewHTTPClient(util.HTTPClientConfig{
		UserAgent:  userAgent,
		ProxyURL:   proxyURL,
		Verbose:    verbose,
		Headers:    finalHeaders,
		Cookie:     finalCookie,
		CookieFile: cookieFile,
	})

	streamType := detectStreamType(url)
	
	// Helper to resolve dynamic filename
	resolveFilename := func(ext string) string {
		if filename != "" {
			return filename
		}
		if detectedTitle != "" {
			return detectedTitle + ext
		}
		return util.FilenameFromURL(url)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	switch streamType {
	case "hls":
		filename = resolveFilename(".ts")
		return downloadHLS(ctx, client, url)
	case "dash":
		filename = resolveFilename(".mp4")
		return downloadDASH(ctx, client, url)
	default:
		filename = resolveFilename(filepath.Ext(util.FilenameFromURL(url)))
		return downloadDirect(ctx, client, url)
	}
}

func parseHeaders(h []string) map[string]string {
	m := make(map[string]string)
	for _, s := range h {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}

func detectStreamType(url string) string {
	lower := strings.ToLower(url)
	if strings.Contains(lower, ".m3u8") {
		return "hls"
	}
	if strings.Contains(lower, ".mpd") {
		return "dash"
	}
	return "direct"
}

func downloadDirect(ctx context.Context, client *util.HTTPClient, url string) error {
	outPath := filepath.Join(outputDir, filename)

	if !forceOverwrite {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("file already exists: %s (use --force to overwrite)", outPath)
		}
	}

	eng := downloader.NewHTTPDownloader(client, downloader.HTTPDownloaderConfig{
		Workers: workers,
		Verbose: verbose,
	})

	fmt.Printf("Downloading: %s\n", url)
	fmt.Printf("Output:      %s\n", outPath)

	return eng.Download(ctx, url, outPath)
}

func downloadHLS(ctx context.Context, client *util.HTTPClient, url string) error {
	fmt.Printf("Fetching HLS playlist: %s\n", url)

	body, err := client.Get(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to fetch playlist: %w", err)
	}

	playlist, err := parser.ParseHLS(body, url)
	if err != nil {
		return fmt.Errorf("failed to parse playlist: %w", err)
	}

	if playlist.IsMaster {
		variant := selectHLSVariant(playlist)
		if variant == nil {
			return fmt.Errorf("no suitable variant found")
		}
		fmt.Printf("Selected variant: %dx%d @ %d kbps\n",
			variant.Resolution.Width, variant.Resolution.Height, variant.Bandwidth/1000)
		return downloadHLS(ctx, client, variant.URI)
	}

	outPath := filepath.Join(outputDir, filename)

	if !forceOverwrite {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("file already exists: %s (use --force to overwrite)", outPath)
		}
	}

	eng := downloader.NewHLSDownloader(client, downloader.HLSDownloaderConfig{
		Workers: workers,
		Verbose: verbose,
	})

	fmt.Printf("Segments:    %d\n", len(playlist.Segments))
	fmt.Printf("Output:      %s\n", outPath)

	return eng.Download(ctx, playlist, outPath)
}

func downloadDASH(ctx context.Context, client *util.HTTPClient, url string) error {
	fmt.Printf("Fetching DASH manifest: %s\n", url)

	body, err := client.Get(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}

	manifest, err := parser.ParseDASH(body, url)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	rep := selectDASHRepresentation(manifest)
	if rep == nil {
		return fmt.Errorf("no suitable representation found")
	}

	fmt.Printf("Selected: %dx%d @ %d kbps\n",
		rep.Width, rep.Height, rep.Bandwidth/1000)

	outPath := filepath.Join(outputDir, filename)

	if !forceOverwrite {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("file already exists: %s (use --force to overwrite)", outPath)
		}
	}

	eng := downloader.NewDASHDownloader(client, downloader.DASHDownloaderConfig{
		Workers: workers,
		Verbose: verbose,
	})

	fmt.Printf("Segments:    %d\n", len(rep.Segments))
	fmt.Printf("Output:      %s\n", outPath)

	return eng.Download(ctx, rep, outPath)
}

func selectHLSVariant(playlist *parser.HLSPlaylist) *parser.HLSVariant {
	if len(playlist.Variants) == 0 {
		return nil
	}

	switch formatSelection {
	case "worst":
		best := &playlist.Variants[0]
		for i := range playlist.Variants {
			if playlist.Variants[i].Bandwidth < best.Bandwidth {
				best = &playlist.Variants[i]
			}
		}
		return best
	default:
		best := &playlist.Variants[0]
		for i := range playlist.Variants {
			if playlist.Variants[i].Bandwidth > best.Bandwidth {
				best = &playlist.Variants[i]
			}
		}
		return best
	}
}

func selectDASHRepresentation(manifest *parser.DASHManifest) *parser.DASHRepresentation {
	var allReps []parser.DASHRepresentation
	for _, period := range manifest.Periods {
		for _, as := range period.AdaptationSets {
			if as.MimeType == "" || strings.HasPrefix(as.MimeType, "video/") {
				allReps = append(allReps, as.Representations...)
			}
		}
	}

	if len(allReps) == 0 {
		return nil
	}

	switch formatSelection {
	case "worst":
		best := &allReps[0]
		for i := range allReps {
			if allReps[i].Bandwidth < best.Bandwidth {
				best = &allReps[i]
			}
		}
		return best
	default:
		best := &allReps[0]
		for i := range allReps {
			if allReps[i].Bandwidth > best.Bandwidth {
				best = &allReps[i]
			}
		}
		return best
	}
}
