package cloud

import (
	"testing"

	"github.com/yingshulu/famfun/internal/model"
)

func TestVideoToJSON(t *testing.T) {
	v := &model.Video{
		ID:           "vid-1",
		Filename:     "test.mp4",
		Title:        "Test Video",
		Duration:     120,
		Filesize:     1024,
		Resolution:   "1920x1080",
		CreatedAt:    "2026-01-01T00:00:00Z",
		HomeServerID: "home-1",
	}

	j := videoToJSON(v)

	if j.ID != "vid-1" {
		t.Errorf("ID = %q, want %q", j.ID, "vid-1")
	}
	if j.ThumbnailURL != "/api/thumbnail/home-1/vid-1" {
		t.Errorf("ThumbnailURL = %q, want %q", j.ThumbnailURL, "/api/thumbnail/home-1/vid-1")
	}
}

func TestVideosToJSON(t *testing.T) {
	videos := []*model.Video{
		{ID: "v1", HomeServerID: "h1"},
		{ID: "v2", HomeServerID: "h2"},
	}

	result := videosToJSON(videos)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0].ThumbnailURL != "/api/thumbnail/h1/v1" {
		t.Errorf("[0] ThumbnailURL = %q", result[0].ThumbnailURL)
	}
	if result[1].ThumbnailURL != "/api/thumbnail/h2/v2" {
		t.Errorf("[1] ThumbnailURL = %q", result[1].ThumbnailURL)
	}
}

func TestVideosToJSONEmpty(t *testing.T) {
	result := videosToJSON([]*model.Video{})
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}
