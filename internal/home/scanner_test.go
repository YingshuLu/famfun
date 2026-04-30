package home

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yingshulu/famfun/internal/model"
)

func TestIsSupportedVideo(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"video.mp4", true},
		{"video.MP4", true},
		{"movie.mkv", true},
		{"clip.avi", true},
		{"clip.mov", true},
		{"clip.webm", true},
		{"doc.pdf", false},
		{"image.png", false},
		{"readme.txt", false},
		{"noext", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isSupportedVideo(tt.path)
			if got != tt.want {
				t.Errorf("isSupportedVideo(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestGenerateVideoID(t *testing.T) {
	id1 := generateVideoID("video1.mp4")
	id2 := generateVideoID("video2.mp4")
	id3 := generateVideoID("video1.mp4")

	if id1 == id2 {
		t.Error("different paths should produce different IDs")
	}
	if id1 != id3 {
		t.Error("same path should produce same ID")
	}
	if len(id1) != 16 {
		t.Errorf("ID length = %d, want 16", len(id1))
	}
}

func TestTitleFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"vacation.mp4", "vacation"},
		{"my movie.mkv", "my movie"},
		{"/path/to/video.avi", "video"},
		{"no-extension", "no-extension"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := titleFromFilename(tt.filename)
			if got != tt.want {
				t.Errorf("titleFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"120.5", 120},
		{"3600.0", 3600},
		{"0", 0},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDuration(tt.input)
			if got != tt.want {
				t.Errorf("parseDuration(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseFilesize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"1024", 1024},
		{"0", 0},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseFilesize(tt.input)
			if got != tt.want {
				t.Errorf("parseFilesize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractResolution(t *testing.T) {
	streams := []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	}{
		{CodecType: "audio", Width: 0, Height: 0},
		{CodecType: "video", Width: 1920, Height: 1080},
	}

	got := extractResolution(streams)
	if got != "1920x1080" {
		t.Errorf("got %q, want %q", got, "1920x1080")
	}
}

func TestExtractResolutionNoVideo(t *testing.T) {
	streams := []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	}{
		{CodecType: "audio", Width: 0, Height: 0},
	}

	got := extractResolution(streams)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExtractCreatedAt(t *testing.T) {
	tags := map[string]string{
		"creation_time": "2026-01-01T00:00:00Z",
	}
	got := extractCreatedAt(tags, "nonexistent.mp4")
	if got != "2026-01-01T00:00:00Z" {
		t.Errorf("got %q, want creation_time from tags", got)
	}
}

func TestExtractCreatedAtFallback(t *testing.T) {
	got := extractCreatedAt(map[string]string{}, "nonexistent.mp4")
	if got == "" {
		t.Error("expected non-empty fallback time")
	}
}

func TestMetadataPath(t *testing.T) {
	got := metadataPath("/streams", "vid1")
	want := filepath.Join("/streams", "vid1", "metadata.json")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSaveAndLoadCachedMetadata(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "famfun_test_metadata")
	defer os.RemoveAll(dir)

	video := &model.Video{
		ID:         "vid1",
		Filename:   "test.mp4",
		Title:      "test",
		Duration:   120,
		Filesize:   1024,
		Resolution: "1920x1080",
		CreatedAt:  "2026-01-01T00:00:00Z",
		SourcePath: "/videos/test.mp4",
	}

	saveCachedMetadata(dir, "vid1", video)

	cached, err := loadCachedMetadata(dir, "vid1", "/videos/test.mp4")
	if err != nil {
		t.Fatalf("loadCachedMetadata: %v", err)
	}

	if cached.ID != "vid1" {
		t.Errorf("ID = %q, want %q", cached.ID, "vid1")
	}
	if cached.Duration != 120 {
		t.Errorf("Duration = %d, want 120", cached.Duration)
	}
	if cached.Resolution != "1920x1080" {
		t.Errorf("Resolution = %q, want %q", cached.Resolution, "1920x1080")
	}
	if cached.SourcePath != "/videos/test.mp4" {
		t.Errorf("SourcePath = %q, want %q", cached.SourcePath, "/videos/test.mp4")
	}
}

func TestLoadCachedMetadataNotFound(t *testing.T) {
	_, err := loadCachedMetadata("/nonexistent", "vid1", "/videos/test.mp4")
	if err == nil {
		t.Error("expected error for missing metadata")
	}
}

func TestLoadCachedMetadataInvalidJSON(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "famfun_test_badjson")
	videoDir := filepath.Join(dir, "vid1")
	os.MkdirAll(videoDir, 0o755)
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(videoDir, "metadata.json"), []byte("not json"), 0o644)

	_, err := loadCachedMetadata(dir, "vid1", "/videos/test.mp4")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSaveCachedMetadataCreatesDir(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "famfun_test_savedir")
	defer os.RemoveAll(dir)

	video := &model.Video{ID: "vid1", Title: "test"}
	saveCachedMetadata(dir, "vid1", video)

	path := metadataPath(dir, "vid1")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected metadata file to exist: %v", err)
	}

	data, _ := os.ReadFile(path)
	var meta videoMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if meta.ID != "vid1" {
		t.Errorf("ID = %q, want %q", meta.ID, "vid1")
	}
}

func TestBuildVideoFromProbe(t *testing.T) {
	probe := &ffprobeOutput{}
	probe.Format.Duration = "120.5"
	probe.Format.Size = "2048"
	probe.Streams = []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	}{
		{CodecType: "video", Width: 1280, Height: 720},
	}

	v := buildVideoFromProbe("abc123", "sub/video.mp4", "/videos/sub/video.mp4", probe)

	if v.ID != "abc123" {
		t.Errorf("ID = %q, want %q", v.ID, "abc123")
	}
	if v.Filename != "video.mp4" {
		t.Errorf("Filename = %q", v.Filename)
	}
	if v.Title != "video" {
		t.Errorf("Title = %q", v.Title)
	}
	if v.Duration != 120 {
		t.Errorf("Duration = %d, want 120", v.Duration)
	}
	if v.Filesize != 2048 {
		t.Errorf("Filesize = %d, want 2048", v.Filesize)
	}
	if v.Resolution != "1280x720" {
		t.Errorf("Resolution = %q", v.Resolution)
	}
	if v.SourcePath != "/videos/sub/video.mp4" {
		t.Errorf("SourcePath = %q", v.SourcePath)
	}
}
