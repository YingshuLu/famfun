package home

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yingshulu/famfun/internal/model"
)

type HLSConverter interface {
	Convert(video *model.Video, outputDir string) error
	IsConverted(video *model.Video, outputDir string) bool
}

type HLSConverterImpl struct{}

func NewHLSConverter() *HLSConverterImpl {
	return &HLSConverterImpl{}
}

func (c *HLSConverterImpl) Convert(video *model.Video, outputDir string) error {
	outDir := videoOutputDir(outputDir, video.ID)
	if err := ensureDir(outDir); err != nil {
		return err
	}

	outputPath := hlsOutputPath(outDir)
	args := buildFFmpegArgs(video.SourcePath, outputPath)

	return runFFmpeg(args)
}

func (c *HLSConverterImpl) IsConverted(video *model.Video, outputDir string) bool {
	outDir := videoOutputDir(outputDir, video.ID)
	return fileExists(hlsOutputPath(outDir))
}

func videoOutputDir(outputDir, videoID string) string {
	return filepath.Join(outputDir, videoID)
}

func hlsOutputPath(outDir string) string {
	return filepath.Join(outDir, "index.m3u8")
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

/*
func buildFFmpegArgs(inputPath, outputPath string) []string {
	return []string{
		"-i", inputPath,
		"-codec:", "copy",
		"-start_number", "0",
		"-hls_time", "6",
		"-hls_list_size", "0",
		"-f", "hls",
		outputPath,
	}
}
*/

func runFFmpeg(args []string) error {
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getVideoCodec(inputPath string) string {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func buildFFmpegArgs(inputPath, outputPath string) []string {
	codec := getVideoCodec(inputPath)

	log.Printf("Detected %s video codec: %s", inputPath, codec)
	switch codec {
	case "h264":
		return []string{
			"-i", inputPath,
			"-c:v", "copy",
			"-c:a", "copy",
			"-bsf:v", "h264_mp4toannexb",
			"-hls_time", "6",
			"-hls_list_size", "0",
			"-f", "hls",
			outputPath,
		}

	default:
		return []string{
			"-i", inputPath,
			"-c:v", "libx264",
			"-c:a", "aac",
			"-preset", "ultrafast",
			"-hls_time", "6",
			"-hls_list_size", "0",
			"-f", "hls",
			outputPath,
		}
	}
}
