package cloud

import (
	"testing"

	pb "github.com/yingshulu/famfun/pkg/proto"
)

func TestBuildCacheKey(t *testing.T) {
	got := buildCacheKey("home1", "vid1", "segment-0.ts")
	want := "home1/vid1/segment-0.ts"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetSegmentCacheHit(t *testing.T) {
	cache := NewLRUCache(1024)
	mgr := NewHomeManager()
	proxy := NewStreamProxy(mgr, cache)

	cache.Put("h1/v1/segment-0.ts", []byte("cached-data"))

	data, err := proxy.GetSegment("h1", "v1", "segment-0.ts")
	if err != nil {
		t.Fatalf("GetSegment: %v", err)
	}
	if string(data) != "cached-data" {
		t.Errorf("got %q, want %q", data, "cached-data")
	}
}

func TestGetSegmentHomeNotConnected(t *testing.T) {
	cache := NewLRUCache(1024)
	mgr := NewHomeManager()
	proxy := NewStreamProxy(mgr, cache)

	_, err := proxy.GetSegment("nonexistent", "v1", "segment-0.ts")
	if err == nil {
		t.Fatal("expected error for disconnected home")
	}
}

func TestGetPlaylistHomeNotConnected(t *testing.T) {
	cache := NewLRUCache(1024)
	mgr := NewHomeManager()
	proxy := NewStreamProxy(mgr, cache)

	_, err := proxy.GetPlaylist("nonexistent", "v1")
	if err == nil {
		t.Fatal("expected error for disconnected home")
	}
}

func TestExtractPlaylistContent(t *testing.T) {
	resp := createPlaylistResponseEnvelope(true, "", []byte("#EXTM3U\nsegment-0.ts\n"))
	data, err := extractPlaylistContent(resp)
	if err != nil {
		t.Fatalf("extractPlaylistContent: %v", err)
	}
	if string(data) != "#EXTM3U\nsegment-0.ts\n" {
		t.Errorf("got %q", data)
	}
}

func TestExtractPlaylistContentError(t *testing.T) {
	resp := createPlaylistResponseEnvelope(false, "not found", nil)
	_, err := extractPlaylistContent(resp)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractSegmentContent(t *testing.T) {
	resp := createSegmentResponseEnvelope(true, "", []byte("ts-data"))
	data, err := extractSegmentContent(resp)
	if err != nil {
		t.Fatalf("extractSegmentContent: %v", err)
	}
	if string(data) != "ts-data" {
		t.Errorf("got %q", data)
	}
}

func TestExtractSegmentContentError(t *testing.T) {
	resp := createSegmentResponseEnvelope(false, "not found", nil)
	_, err := extractSegmentContent(resp)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetThumbnailCacheHit(t *testing.T) {
	cache := NewLRUCache(1024)
	mgr := NewHomeManager()
	proxy := NewStreamProxy(mgr, cache)

	cache.Put("h1/v1/thumbnail", []byte("cached-thumb"))

	data, err := proxy.GetThumbnail("h1", "v1")
	if err != nil {
		t.Fatalf("GetThumbnail: %v", err)
	}
	if string(data) != "cached-thumb" {
		t.Errorf("got %q, want %q", data, "cached-thumb")
	}
}

func TestGetThumbnailHomeNotConnected(t *testing.T) {
	cache := NewLRUCache(1024)
	mgr := NewHomeManager()
	proxy := NewStreamProxy(mgr, cache)

	_, err := proxy.GetThumbnail("nonexistent", "v1")
	if err == nil {
		t.Fatal("expected error for disconnected home")
	}
}

func TestUploadVideoHomeNotConnected(t *testing.T) {
	cache := NewLRUCache(1024)
	mgr := NewHomeManager()
	proxy := NewStreamProxy(mgr, cache)

	_, err := proxy.UploadVideo("nonexistent", "test.mp4", "Test", "", 1024, "abc123", nil)
	if err == nil {
		t.Fatal("expected error for disconnected home")
	}
}

// helpers

func createPlaylistResponseEnvelope(success bool, errMsg string, content []byte) *pb.Envelope {
	return &pb.Envelope{
		Payload: &pb.Envelope_GetPlaylistResponse{
			GetPlaylistResponse: &pb.GetPlaylistResponse{
				Success: success,
				Error:   errMsg,
				Content: content,
			},
		},
	}
}

func createSegmentResponseEnvelope(success bool, errMsg string, content []byte) *pb.Envelope {
	return &pb.Envelope{
		Payload: &pb.Envelope_GetSegmentResponse{
			GetSegmentResponse: &pb.GetSegmentResponse{
				Success: success,
				Error:   errMsg,
				Content: content,
			},
		},
	}
}
