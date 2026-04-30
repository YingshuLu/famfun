package home

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yingshulu/famfun/internal/model"
)

type ThumbnailGenerator interface {
	Generate(video *model.Video, outputDir string) ([]byte, error)
	IsGenerated(video *model.Video, outputDir string) bool
}

type ThumbnailGeneratorImpl struct{}

func NewThumbnailGenerator() *ThumbnailGeneratorImpl {
	return &ThumbnailGeneratorImpl{}
}

func (g *ThumbnailGeneratorImpl) Generate(video *model.Video, outputDir string) ([]byte, error) {
	if err := ensureDir(outputDir); err != nil {
		return nil, err
	}

	outputPath := thumbnailPath(outputDir, video.ID)

	if err := extractThumbnail(video.SourcePath, outputPath); err != nil {
		return nil, err
	}

	return readThumbnail(outputPath)
}

func (g *ThumbnailGeneratorImpl) IsGenerated(video *model.Video, outputDir string) bool {
	return fileExists(thumbnailPath(outputDir, video.ID))
}

func thumbnailPath(outputDir, videoID string) string {
	return filepath.Join(outputDir, videoID+".png")
}

func extractThumbnail(inputPath, outputPath string) error {
	args := buildThumbnailArgs(inputPath, outputPath)
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg thumbnail: %w", err)
	}
	return nil
}

func buildThumbnailArgs(inputPath, outputPath string) []string {
	return []string{
		"-i", inputPath,
		"-ss", "00:00:05",
		"-vframes", "1",
		"-vf", "scale=320:-1",
		outputPath,
	}
}

func readThumbnail(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read thumbnail: %w", err)
	}
	return data, nil
}
