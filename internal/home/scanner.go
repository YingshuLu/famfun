package home

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yingshulu/famfun/internal/model"
)

var supportedExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".webm": true,
}

type VideoScanner interface {
	Scan(videoDir, streamDir string) ([]*model.Video, error)
}

type VideoScannerImpl struct{}

func NewVideoScanner() *VideoScannerImpl {
	return &VideoScannerImpl{}
}

func (s *VideoScannerImpl) Scan(videoDir, streamDir string) ([]*model.Video, error) {
	files, err := s.findVideoFiles(videoDir)
	if err != nil {
		return nil, err
	}

	var videos []*model.Video
	for _, filePath := range files {
		video, err := s.probeVideo(videoDir, streamDir, filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping %s: %v\n", filePath, err)
			continue
		}
		videos = append(videos, video)
	}

	return videos, nil
}

func (s *VideoScannerImpl) findVideoFiles(videoDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(videoDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if isSupportedVideo(path) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func isSupportedVideo(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return supportedExtensions[ext]
}

func (s *VideoScannerImpl) probeVideo(videoDir, streamDir, filePath string) (*model.Video, error) {
	relPath, err := filepath.Rel(videoDir, filePath)
	if err != nil {
		return nil, fmt.Errorf("relative path: %w", err)
	}

	videoID := generateVideoID(relPath)

	if cached, err := loadCachedMetadata(streamDir, videoID, filePath); err == nil {
		return cached, nil
	}

	probe, err := runFFProbe(filePath)
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	video := buildVideoFromProbe(videoID, relPath, filePath, probe)

	saveCachedMetadata(streamDir, videoID, video)

	return video, nil
}

type videoMetadata struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	Duration    int64  `json:"duration"`
	Filesize    int64  `json:"filesize"`
	Resolution  string `json:"resolution"`
	CreatedAt   string `json:"created_at"`
}

func metadataPath(streamDir, videoID string) string {
	return filepath.Join(streamDir, videoID, "metadata.json")
}

func loadCachedMetadata(streamDir, videoID, sourcePath string) (*model.Video, error) {
	path := metadataPath(streamDir, videoID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta videoMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	visibility := meta.Visibility
	if visibility == "" {
		visibility = "member"
	}
	return &model.Video{
		ID:          meta.ID,
		Filename:    meta.Filename,
		Title:       meta.Title,
		Description: meta.Description,
		Visibility:  visibility,
		Duration:    meta.Duration,
		Filesize:    meta.Filesize,
		Resolution:  meta.Resolution,
		CreatedAt:   meta.CreatedAt,
		SourcePath:  sourcePath,
	}, nil
}

func saveCachedMetadata(streamDir, videoID string, v *model.Video) {
	meta := videoMetadata{
		ID:          v.ID,
		Filename:    v.Filename,
		Title:       v.Title,
		Description: v.Description,
		Visibility:  v.Visibility,
		Duration:    v.Duration,
		Filesize:    v.Filesize,
		Resolution:  v.Resolution,
		CreatedAt:   v.CreatedAt,
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}

	dir := filepath.Dir(metadataPath(streamDir, videoID))
	os.MkdirAll(dir, 0o755)
	os.WriteFile(metadataPath(streamDir, videoID), data, 0o644)
}

func generateVideoID(relPath string) string {
	hash := sha256.Sum256([]byte(relPath))
	return hex.EncodeToString(hash[:8])
}

func titleFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filepath.Base(filename), ext)
}

type ffprobeOutput struct {
	Format struct {
		Duration string            `json:"duration"`
		Size     string            `json:"size"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
}

func runFFProbe(filePath string) (*ffprobeOutput, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe exec: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	return &probe, nil
}

func buildVideoFromProbe(videoID, relPath, sourcePath string, probe *ffprobeOutput) *model.Video {
	return &model.Video{
		ID:         videoID,
		Filename:   filepath.Base(sourcePath),
		Title:      titleFromFilename(sourcePath),
		Duration:   parseDuration(probe.Format.Duration),
		Filesize:   parseFilesize(probe.Format.Size),
		Resolution: extractResolution(probe.Streams),
		CreatedAt:  extractCreatedAt(probe.Format.Tags, sourcePath),
		SourcePath: sourcePath,
	}
}

func parseDuration(s string) int64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(f)
}

func parseFilesize(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func extractResolution(streams []struct {
	CodecType string `json:"codec_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}) string {
	for _, s := range streams {
		if s.CodecType == "video" && s.Width > 0 && s.Height > 0 {
			return fmt.Sprintf("%dx%d", s.Width, s.Height)
		}
	}
	return ""
}

func extractCreatedAt(tags map[string]string, filePath string) string {
	if ct, ok := tags["creation_time"]; ok {
		return ct
	}
	return fileModTime(filePath)
}

func fileModTime(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return info.ModTime().UTC().Format(time.RFC3339)
}
