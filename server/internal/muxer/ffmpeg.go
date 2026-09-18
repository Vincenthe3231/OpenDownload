package muxer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type FFmpegConfig struct {
	BinaryPath string
	Verbose    bool
}

type FFmpeg struct {
	config FFmpegConfig
}

func NewFFmpeg(config FFmpegConfig) *FFmpeg {
	if config.BinaryPath == "" {
		config.BinaryPath = findFFmpeg()
	}
	return &FFmpeg{config: config}
}

func (f *FFmpeg) Available() bool {
	if f.config.BinaryPath == "" {
		return false
	}
	_, err := exec.LookPath(f.config.BinaryPath)
	return err == nil
}

func (f *FFmpeg) Remux(ctx context.Context, input, output string) error {
	args := []string{
		"-y",
		"-i", input,
		"-c", "copy",
		output,
	}
	return f.run(ctx, args)
}

func (f *FFmpeg) MergeVideoAudio(ctx context.Context, videoPath, audioPath, output string) error {
	args := []string{
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-c", "copy",
		"-shortest",
		output,
	}
	return f.run(ctx, args)
}

func (f *FFmpeg) ConvertToMP4(ctx context.Context, input, output string) error {
	args := []string{
		"-y",
		"-i", input,
		"-c:v", "copy",
		"-c:a", "aac",
		"-movflags", "+faststart",
		output,
	}
	return f.run(ctx, args)
}

func (f *FFmpeg) ExtractAudio(ctx context.Context, input, output, format string) error {
	args := []string{
		"-y",
		"-i", input,
		"-vn",
		"-acodec", format,
		output,
	}
	return f.run(ctx, args)
}

func (f *FFmpeg) ConcatFiles(ctx context.Context, inputs []string, output string) error {
	listFile := filepath.Join(filepath.Dir(output), ".concat_list.txt")
	var lines []string
	for _, in := range inputs {
		absPath, _ := filepath.Abs(in)
		lines = append(lines, fmt.Sprintf("file '%s'", strings.ReplaceAll(absPath, "'", "'\\''")))
	}

	if err := os.WriteFile(listFile, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to create concat list: %w", err)
	}
	defer func() { _ = os.Remove(listFile) }()

	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-c", "copy",
		output,
	}
	return f.run(ctx, args)
}

func (f *FFmpeg) run(ctx context.Context, args []string) error {
	if !f.config.Verbose {
		args = append([]string{"-hide_banner", "-loglevel", "error"}, args...)
	}

	cmd := exec.CommandContext(ctx, f.config.BinaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	return nil
}

func findFFmpeg() string {
	candidates := []string{"ffmpeg"}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, "ffmpeg.exe")
	}

	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return "ffmpeg"
}
