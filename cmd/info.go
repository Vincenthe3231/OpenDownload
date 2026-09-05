package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/opendownload/opendownload/internal/media"
	"github.com/opendownload/opendownload/internal/parser"
	"github.com/opendownload/opendownload/internal/transport"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info [url]",
	Short: "Inspect a URL and list available formats",
	Long:  `Inspect a video URL and display available formats, qualities, and metadata without downloading.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func runInfo(cmd *cobra.Command, args []string) error {
	url := args[0]
	ctx := context.Background()

	client := transport.NewHTTPClient(transport.HTTPClientConfig{
		UserAgent: userAgent,
		ProxyURL:  proxyURL,
		Verbose:   verbose,
	})

	switch media.ClassifyURL(url) {
	case media.SourceHLS:
		return infoHLS(ctx, client, url)
	case media.SourceDASH:
		return infoDASH(ctx, client, url)
	default:
		return infoDirect(ctx, client, url)
	}
}

func infoDirect(ctx context.Context, client *transport.HTTPClient, url string) error {
	size, resumable, contentType, err := client.Head(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to inspect URL: %w", err)
	}

	fmt.Printf("URL:          %s\n", url)
	fmt.Printf("Type:         Direct download\n")
	fmt.Printf("Content-Type: %s\n", contentType)
	if size > 0 {
		fmt.Printf("Size:         %s\n", media.FormatBytes(size))
	} else {
		fmt.Printf("Size:         Unknown\n")
	}
	fmt.Printf("Resumable:    %v\n", resumable)
	fmt.Printf("Filename:     %s\n", media.FilenameFromURL(url))

	return nil
}

func infoHLS(ctx context.Context, client *transport.HTTPClient, url string) error {
	body, err := client.Get(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to fetch playlist: %w", err)
	}

	playlist, err := parser.ParseHLS(body, url)
	if err != nil {
		return fmt.Errorf("failed to parse playlist: %w", err)
	}

	fmt.Printf("URL:     %s\n", url)
	fmt.Printf("Type:    HLS stream\n")

	if playlist.IsMaster {
		fmt.Printf("Variants: %d\n\n", len(playlist.Variants))
		fmt.Printf("  %-4s  %-12s  %-8s  %-10s  %s\n", "ID", "Resolution", "FPS", "Bandwidth", "Codecs")
		fmt.Printf("  %-4s  %-12s  %-8s  %-10s  %s\n", "──", "──────────", "───", "─────────", "──────")
		for i, v := range playlist.Variants {
			res := fmt.Sprintf("%dx%d", v.Resolution.Width, v.Resolution.Height)
			bw := media.FormatBitrate(v.Bandwidth)
			fps := ""
			if v.FrameRate > 0 {
				fps = fmt.Sprintf("%.0f", v.FrameRate)
			}
			fmt.Printf("  %-4d  %-12s  %-8s  %-10s  %s\n", i, res, fps, bw, v.Codecs)
		}

		if len(playlist.AudioGroups) > 0 {
			fmt.Printf("\nAudio tracks:\n")
			for group, tracks := range playlist.AudioGroups {
				fmt.Printf("  Group: %s\n", group)
				for _, t := range tracks {
					def := ""
					if t.Default {
						def = " (default)"
					}
					fmt.Printf("    - %s [%s]%s\n", t.Name, t.Language, def)
				}
			}
		}
	} else {
		fmt.Printf("Segments: %d\n", len(playlist.Segments))
		if playlist.TargetDuration > 0 {
			fmt.Printf("Target Duration: %.0fs\n", playlist.TargetDuration)
		}
		if playlist.Encryption != nil {
			fmt.Printf("Encryption: %s\n", playlist.Encryption.Method)
		}
	}

	return nil
}

func infoDASH(ctx context.Context, client *transport.HTTPClient, url string) error {
	body, err := client.Get(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}

	manifest, err := parser.ParseDASH(body, url)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	fmt.Printf("URL:     %s\n", url)
	fmt.Printf("Type:    DASH stream\n")
	fmt.Printf("Periods: %d\n\n", len(manifest.Periods))

	for pi, period := range manifest.Periods {
		fmt.Printf("Period %d:\n", pi)
		for _, as := range period.AdaptationSets {
			fmt.Printf("  Adaptation Set: %s (lang=%s)\n", as.MimeType, as.Lang)
			fmt.Printf("    %-4s  %-12s  %-10s  %s\n", "ID", "Resolution", "Bandwidth", "Codecs")
			fmt.Printf("    %-4s  %-12s  %-10s  %s\n", "──", "──────────", "─────────", "──────")
			for _, r := range as.Representations {
				res := fmt.Sprintf("%dx%d", r.Width, r.Height)
				bw := media.FormatBitrate(r.Bandwidth)
				codecs := r.Codecs
				if codecs == "" {
					codecs = as.Codecs
				}
				isVideo := strings.HasPrefix(as.MimeType, "video/")
				if !isVideo {
					res = "-"
				}
				fmt.Printf("    %-4s  %-12s  %-10s  %s\n", r.ID, res, bw, codecs)
			}
		}
	}

	return nil
}
