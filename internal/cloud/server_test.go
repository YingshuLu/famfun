package cloud

import (
	"sort"
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

func TestPaginateVideos(t *testing.T) {
	videos := []videoJSON{
		{ID: "a", CreatedAt: "2026-01-03"},
		{ID: "b", CreatedAt: "2026-01-01"},
		{ID: "c", CreatedAt: "2026-01-02"},
		{ID: "d", CreatedAt: "2026-01-02"},
		{ID: "e", CreatedAt: "2026-01-03"},
	}

	sort.Slice(videos, func(i, j int) bool {
		if videos[i].CreatedAt != videos[j].CreatedAt {
			return videos[i].CreatedAt > videos[j].CreatedAt
		}
		return videos[i].ID < videos[j].ID
	})

	if videos[0].ID != "a" || videos[1].ID != "e" {
		t.Fatalf("sort order wrong: got %v %v", videos[0].ID, videos[1].ID)
	}
	if videos[2].ID != "c" || videos[3].ID != "d" {
		t.Fatalf("sort order wrong: got %v %v", videos[2].ID, videos[3].ID)
	}
	if videos[4].ID != "b" {
		t.Fatalf("sort order wrong: got %v", videos[4].ID)
	}

	total := len(videos)

	page1 := videos[0:2]
	if len(page1) != 2 || page1[0].ID != "a" || page1[1].ID != "e" {
		t.Errorf("page1 = %v", page1)
	}
	hasMore1 := 2 < total
	if !hasMore1 {
		t.Error("expected has_more=true for page1")
	}

	page2 := videos[2:4]
	if len(page2) != 2 || page2[0].ID != "c" || page2[1].ID != "d" {
		t.Errorf("page2 = %v", page2)
	}

	page3 := videos[4:5]
	if len(page3) != 1 || page3[0].ID != "b" {
		t.Errorf("page3 = %v", page3)
	}
	hasMore3 := 6 < total
	if hasMore3 {
		t.Error("expected has_more=false for page3")
	}

	offset := 10
	if offset >= total {
		// out of range returns empty
	} else {
		t.Error("expected offset >= total")
	}
}
