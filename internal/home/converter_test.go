package home

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yingshulu/famfun/internal/model"
)

func TestVideoOutputDir(t *testing.T) {
	got := videoOutputDir("/output", "vid123")
	want := filepath.Join("/output", "vid123")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHlsOutputPath(t *testing.T) {
	got := hlsOutputPath("/output/vid123")
	want := filepath.Join("/output/vid123", "index.m3u8")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildFFmpegArgs(t *testing.T) {
	args := buildFFmpegArgs("/input/video.mp4", "/output/index.m3u8")

	if len(args) != 13 {
		t.Fatalf("len(args) = %d, want 13", len(args))
	}
	if args[0] != "-i" || args[1] != "/input/video.mp4" {
		t.Errorf("input args: %v %v", args[0], args[1])
	}
	if args[len(args)-1] != "/output/index.m3u8" {
		t.Errorf("output = %q", args[len(args)-1])
	}
}

func TestEnsureDir(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "famfun_test_ensuredir")
	defer os.RemoveAll(dir)

	if err := ensureDir(dir); err != nil {
		t.Fatalf("ensureDir: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestFileExists(t *testing.T) {
	f, err := os.CreateTemp("", "famfun_test")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	if !fileExists(f.Name()) {
		t.Error("expected file to exist")
	}
	if fileExists("/nonexistent/path/file.txt") {
		t.Error("expected file to not exist")
	}
}

func TestIsConvertedFalse(t *testing.T) {
	conv := NewHLSConverter()
	video := &model.Video{ID: "nonexistent-video"}
	if conv.IsConverted(video, "/tmp/nonexistent_dir") {
		t.Error("expected not converted")
	}
}

func TestIsConvertedTrue(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "famfun_test_converted")
	videoDir := filepath.Join(dir, "vid1")
	os.MkdirAll(videoDir, 0o755)
	defer os.RemoveAll(dir)

	m3u8 := filepath.Join(videoDir, "index.m3u8")
	os.WriteFile(m3u8, []byte("#EXTM3U\n"), 0o644)

	conv := NewHLSConverter()
	video := &model.Video{ID: "vid1"}
	if !conv.IsConverted(video, dir) {
		t.Error("expected converted")
	}
}
