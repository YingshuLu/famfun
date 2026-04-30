package home

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yingshulu/famfun/internal/model"
)

func TestThumbnailPath(t *testing.T) {
	got := thumbnailPath("/thumbs", "vid1")
	want := filepath.Join("/thumbs", "vid1.png")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildThumbnailArgs(t *testing.T) {
	args := buildThumbnailArgs("/input/video.mp4", "/output/thumb.png")

	if len(args) != 9 {
		t.Fatalf("len(args) = %d, want 9", len(args))
	}
	if args[1] != "/input/video.mp4" {
		t.Errorf("input = %q", args[1])
	}
	if args[len(args)-1] != "/output/thumb.png" {
		t.Errorf("output = %q", args[len(args)-1])
	}
	if args[3] != "00:00:05" {
		t.Errorf("seek = %q, want 00:00:05", args[3])
	}
}

func TestReadThumbnail(t *testing.T) {
	f, err := os.CreateTemp("", "famfun_thumb_*.png")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	content := []byte("fake-png-data")
	f.Write(content)
	f.Close()

	data, err := readThumbnail(f.Name())
	if err != nil {
		t.Fatalf("readThumbnail: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("got %q, want %q", data, content)
	}
}

func TestReadThumbnailNotFound(t *testing.T) {
	_, err := readThumbnail("/nonexistent/thumb.png")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestIsGeneratedFalse(t *testing.T) {
	gen := NewThumbnailGenerator()
	video := &model.Video{ID: "nonexistent"}
	if gen.IsGenerated(video, "/tmp/nonexistent_dir") {
		t.Error("expected not generated")
	}
}

func TestIsGeneratedTrue(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "famfun_test_thumb")
	os.MkdirAll(dir, 0o755)
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "vid1.png"), []byte("png"), 0o644)

	gen := NewThumbnailGenerator()
	video := &model.Video{ID: "vid1"}
	if !gen.IsGenerated(video, dir) {
		t.Error("expected generated")
	}
}
