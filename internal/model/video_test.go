package model

import (
	"testing"

	pb "github.com/yingshulu/famfun/pkg/proto"
)

func TestVideoToProto(t *testing.T) {
	v := &Video{
		ID:           "vid-1",
		Filename:     "test.mp4",
		Title:        "Test",
		Duration:     120,
		Filesize:     1024,
		Resolution:   "1920x1080",
		CreatedAt:    "2026-01-01T00:00:00Z",
		HomeServerID: "home-1",
	}

	p := VideoToProto(v)
	if p.Id != v.ID {
		t.Errorf("Id = %q, want %q", p.Id, v.ID)
	}
	if p.Filename != v.Filename {
		t.Errorf("Filename = %q, want %q", p.Filename, v.Filename)
	}
	if p.Duration != v.Duration {
		t.Errorf("Duration = %d, want %d", p.Duration, v.Duration)
	}
	if p.HomeServerId != v.HomeServerID {
		t.Errorf("HomeServerId = %q, want %q", p.HomeServerId, v.HomeServerID)
	}
}

func TestProtoToVideo(t *testing.T) {
	p := &pb.VideoInfo{
		Id:           "vid-2",
		Filename:     "movie.mp4",
		Title:        "Movie",
		Duration:     3600,
		Filesize:     2048,
		Resolution:   "1280x720",
		CreatedAt:    "2026-02-01T00:00:00Z",
		HomeServerId: "home-2",
	}

	v := ProtoToVideo(p)
	if v.ID != p.Id {
		t.Errorf("ID = %q, want %q", v.ID, p.Id)
	}
	if v.HomeServerID != p.HomeServerId {
		t.Errorf("HomeServerID = %q, want %q", v.HomeServerID, p.HomeServerId)
	}
}

func TestVideosToProtoRoundTrip(t *testing.T) {
	videos := []*Video{
		{ID: "v1", Title: "First"},
		{ID: "v2", Title: "Second"},
		{ID: "v3", Title: "Third"},
	}

	protos := VideosToProto(videos)
	if len(protos) != 3 {
		t.Fatalf("len = %d, want 3", len(protos))
	}

	result := ProtoToVideos(protos)
	if len(result) != 3 {
		t.Fatalf("len = %d, want 3", len(result))
	}

	for i, v := range result {
		if v.ID != videos[i].ID {
			t.Errorf("[%d] ID = %q, want %q", i, v.ID, videos[i].ID)
		}
		if v.Title != videos[i].Title {
			t.Errorf("[%d] Title = %q, want %q", i, v.Title, videos[i].Title)
		}
	}
}

func TestEmptySliceConversion(t *testing.T) {
	protos := VideosToProto([]*Video{})
	if len(protos) != 0 {
		t.Errorf("expected empty slice, got %d", len(protos))
	}

	videos := ProtoToVideos([]*pb.VideoInfo{})
	if len(videos) != 0 {
		t.Errorf("expected empty slice, got %d", len(videos))
	}
}
