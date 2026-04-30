package home

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yingshulu/famfun/internal/model"
	"github.com/yingshulu/famfun/internal/protocol"
	pb "github.com/yingshulu/famfun/pkg/proto"
)

type mockScanner struct {
	videos []*model.Video
	err    error
}

func (m *mockScanner) Scan(videoDir, streamDir string) ([]*model.Video, error) {
	return m.videos, m.err
}

type mockConverter struct {
	mu        sync.Mutex
	converted map[string]bool
}

func (m *mockConverter) Convert(video *model.Video, outputDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.converted[video.ID] = true
	return nil
}

func (m *mockConverter) IsConverted(video *model.Video, outputDir string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.converted[video.ID]
}

type mockThumbGen struct {
	mu        sync.Mutex
	generated map[string]bool
}

func (m *mockThumbGen) Generate(video *model.Video, outputDir string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generated[video.ID] = true
	return []byte("fake-png"), nil
}

func (m *mockThumbGen) IsGenerated(video *model.Video, outputDir string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generated[video.ID]
}

type mockConnector struct {
	mu           sync.Mutex
	envelopes    []*pb.Envelope
	handler      MessageHandler
	disconnected chan struct{}
}

func (m *mockConnector) Connect(ctx context.Context, cloudAddr string) error {
	return nil
}

func (m *mockConnector) SendEnvelope(env *pb.Envelope) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.envelopes = append(m.envelopes, env)
	return nil
}

func (m *mockConnector) SetMessageHandler(handler MessageHandler) {
	m.handler = handler
}

func (m *mockConnector) Disconnected() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.disconnected == nil {
		m.disconnected = make(chan struct{})
	}
	return m.disconnected
}

func (m *mockConnector) Close() error {
	return nil
}

func (m *mockConnector) getEnvelopes() []*pb.Envelope {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*pb.Envelope, len(m.envelopes))
	copy(result, m.envelopes)
	return result
}

func TestHomeServerRegister(t *testing.T) {
	conn := &mockConnector{}
	srv := NewHomeServer("home-1", "Test Home", "/videos", "/streams", "/thumbs",
		&mockScanner{}, &mockConverter{converted: make(map[string]bool)},
		&mockThumbGen{generated: make(map[string]bool)}, conn)

	srv.register()

	envs := conn.getEnvelopes()
	if len(envs) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(envs))
	}

	req := envs[0].GetRegisterRequest()
	if req == nil {
		t.Fatal("expected RegisterRequest")
	}
	if req.HomeServerId != "home-1" {
		t.Errorf("HomeServerId = %q, want %q", req.HomeServerId, "home-1")
	}
}

func TestProcessAndPublishSendsToCloud(t *testing.T) {
	conn := &mockConnector{}
	converter := &mockConverter{converted: make(map[string]bool)}
	thumbGen := &mockThumbGen{generated: make(map[string]bool)}

	srv := NewHomeServer("home-1", "Test Home", "/videos", "/streams", "/thumbs",
		&mockScanner{}, converter, thumbGen, conn)

	v := &model.Video{ID: "v1", Filename: "test.mp4", HomeServerID: "home-1"}
	srv.processAndPublish(v)

	envs := conn.getEnvelopes()
	if len(envs) != 1 {
		t.Fatalf("expected 1 envelope, got %d", len(envs))
	}

	update := envs[0].GetVideoListUpdate()
	if update == nil {
		t.Fatal("expected VideoListUpdate")
	}
	if len(update.VideoIds) != 1 {
		t.Errorf("video_ids = %d, want 1", len(update.VideoIds))
	}
}

func TestProcessAndPublishIncremental(t *testing.T) {
	conn := &mockConnector{}
	converter := &mockConverter{converted: make(map[string]bool)}
	thumbGen := &mockThumbGen{generated: make(map[string]bool)}

	srv := NewHomeServer("home-1", "Test Home", "/videos", "/streams", "/thumbs",
		&mockScanner{}, converter, thumbGen, conn)

	srv.processAndPublish(&model.Video{ID: "v1", Filename: "a.mp4", HomeServerID: "home-1"})
	srv.processAndPublish(&model.Video{ID: "v2", Filename: "b.mp4", HomeServerID: "home-1"})

	envs := conn.getEnvelopes()
	if len(envs) != 2 {
		t.Fatalf("expected 2 envelopes, got %d", len(envs))
	}

	first := envs[0].GetVideoListUpdate()
	if len(first.VideoIds) != 1 {
		t.Errorf("first update: video_ids = %d, want 1", len(first.VideoIds))
	}

	second := envs[1].GetVideoListUpdate()
	if len(second.VideoIds) != 2 {
		t.Errorf("second update: video_ids = %d, want 2", len(second.VideoIds))
	}
}

func TestScanAndConvertBackgroundNonBlocking(t *testing.T) {
	scanner := &mockScanner{
		videos: []*model.Video{
			{ID: "v1", Filename: "a.mp4"},
			{ID: "v2", Filename: "b.mp4"},
		},
	}
	conn := &mockConnector{}
	converter := &mockConverter{converted: make(map[string]bool)}
	thumbGen := &mockThumbGen{generated: make(map[string]bool)}

	srv := NewHomeServer("h1", "Test", "/videos", "/streams", "/thumbs",
		scanner, converter, thumbGen, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go srv.scanAndConvertBackground(ctx)

	time.Sleep(100 * time.Millisecond)

	videos := srv.getVideos()
	if len(videos) != 2 {
		t.Errorf("expected 2 videos, got %d", len(videos))
	}

	envs := conn.getEnvelopes()
	if len(envs) != 2 {
		t.Errorf("expected 2 cloud notifications, got %d", len(envs))
	}
}

func TestScanAndConvertSkipsAlreadyConverted(t *testing.T) {
	converter := &mockConverter{converted: map[string]bool{"v1": true}}
	thumbGen := &mockThumbGen{generated: make(map[string]bool)}
	scanner := &mockScanner{
		videos: []*model.Video{
			{ID: "v1", Filename: "a.mp4"},
		},
	}
	conn := &mockConnector{}

	srv := NewHomeServer("h1", "Test", "/videos", "/streams", "/thumbs",
		scanner, converter, thumbGen, conn)

	ctx := context.Background()
	srv.scanAndConvertBackground(ctx)

	converter.mu.Lock()
	wasConverted := converter.converted["v1"]
	converter.mu.Unlock()

	if !wasConverted {
		t.Error("v1 should still be marked as converted")
	}
}

func TestScanAndConvertCallsConverterForNew(t *testing.T) {
	converter := &mockConverter{converted: make(map[string]bool)}
	thumbGen := &mockThumbGen{generated: make(map[string]bool)}
	scanner := &mockScanner{
		videos: []*model.Video{
			{ID: "v1", Filename: "a.mp4"},
			{ID: "v2", Filename: "b.mp4"},
		},
	}
	conn := &mockConnector{}

	srv := NewHomeServer("h1", "Test", "/videos", "/streams", "/thumbs",
		scanner, converter, thumbGen, conn)

	ctx := context.Background()
	srv.scanAndConvertBackground(ctx)

	converter.mu.Lock()
	defer converter.mu.Unlock()

	if !converter.converted["v1"] {
		t.Error("expected v1 to be converted")
	}
	if !converter.converted["v2"] {
		t.Error("expected v2 to be converted")
	}
}

func TestAddVideo(t *testing.T) {
	srv := &HomeServer{videos: make(map[string]*model.Video)}
	srv.addVideo(&model.Video{ID: "v1"})
	srv.addVideo(&model.Video{ID: "v2"})

	videos := srv.getVideos()
	if len(videos) != 2 {
		t.Errorf("expected 2 videos, got %d", len(videos))
	}
}

func TestAddVideoDeduplicate(t *testing.T) {
	srv := &HomeServer{videos: make(map[string]*model.Video)}
	srv.addVideo(&model.Video{ID: "v1", Title: "Old"})
	srv.addVideo(&model.Video{ID: "v1", Title: "New"})

	videos := srv.getVideos()
	if len(videos) != 1 {
		t.Errorf("expected 1 video, got %d", len(videos))
	}
	if videos[0].Title != "New" {
		t.Errorf("expected title 'New', got %q", videos[0].Title)
	}
}

func TestGetVideosCopiesSlice(t *testing.T) {
	srv := &HomeServer{videos: make(map[string]*model.Video)}
	srv.addVideo(&model.Video{ID: "v1"})

	videos := srv.getVideos()
	videos = append(videos, &model.Video{ID: "v2"})

	if len(srv.getVideos()) != 1 {
		t.Error("getVideos should return a copy, not a reference")
	}
}

func TestHomeServerHandleGetPlaylist(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "famfun_test_playlist")
	videoDir := filepath.Join(dir, "vid1")
	os.MkdirAll(videoDir, 0o755)
	defer os.RemoveAll(dir)

	m3u8Content := "#EXTM3U\n#EXTINF:6.0,\nsegment-0.ts\n"
	os.WriteFile(filepath.Join(videoDir, "index.m3u8"), []byte(m3u8Content), 0o644)

	srv := &HomeServer{streamDir: dir, videos: make(map[string]*model.Video)}

	resp := srv.HandleGetPlaylist(&pb.GetPlaylistRequest{
		VideoId:   "vid1",
		RequestId: "req-1",
	})

	if !resp.Success {
		t.Fatalf("expected success, got error: %s", resp.Error)
	}
	if string(resp.Content) != m3u8Content {
		t.Errorf("content = %q, want %q", resp.Content, m3u8Content)
	}
}

func TestHomeServerHandleGetPlaylistNotFound(t *testing.T) {
	srv := &HomeServer{streamDir: "/nonexistent", videos: make(map[string]*model.Video)}

	resp := srv.HandleGetPlaylist(&pb.GetPlaylistRequest{
		VideoId:   "missing",
		RequestId: "req-2",
	})

	if resp.Success {
		t.Error("expected failure")
	}
}

func TestHomeServerHandleGetSegment(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "famfun_test_segment")
	videoDir := filepath.Join(dir, "vid1")
	os.MkdirAll(videoDir, 0o755)
	defer os.RemoveAll(dir)

	segmentData := []byte("fake-ts-data")
	os.WriteFile(filepath.Join(videoDir, "segment-0.ts"), segmentData, 0o644)

	srv := &HomeServer{streamDir: dir, videos: make(map[string]*model.Video)}

	resp := srv.HandleGetSegment(&pb.GetSegmentRequest{
		VideoId:     "vid1",
		SegmentName: "segment-0.ts",
		RequestId:   "req-3",
	})

	if !resp.Success {
		t.Fatalf("expected success, got error: %s", resp.Error)
	}
	if string(resp.Content) != string(segmentData) {
		t.Errorf("content mismatch")
	}
}

func TestHomeServerHandleGetSegmentNotFound(t *testing.T) {
	srv := &HomeServer{streamDir: "/nonexistent", videos: make(map[string]*model.Video)}

	resp := srv.HandleGetSegment(&pb.GetSegmentRequest{
		VideoId:     "missing",
		SegmentName: "segment-0.ts",
		RequestId:   "req-4",
	})

	if resp.Success {
		t.Error("expected failure")
	}
}

func TestHomeServerHandleGetVideoInfo(t *testing.T) {
	srv := &HomeServer{videos: make(map[string]*model.Video)}
	srv.addVideo(&model.Video{ID: "v1", Filename: "a.mp4", Title: "Video A", HomeServerID: "h1"})

	resp := srv.HandleGetVideoInfo(&pb.GetVideoInfoRequest{VideoId: "v1"})
	if !resp.Success {
		t.Fatalf("expected success, got error: %s", resp.Error)
	}
	if resp.Video.Id != "v1" {
		t.Errorf("video id = %q, want %q", resp.Video.Id, "v1")
	}
}

func TestHomeServerHandleGetVideoInfoNotFound(t *testing.T) {
	srv := &HomeServer{videos: make(map[string]*model.Video)}

	resp := srv.HandleGetVideoInfo(&pb.GetVideoInfoRequest{VideoId: "missing"})
	if resp.Success {
		t.Error("expected failure")
	}
}

func TestHomeServerHandleGetThumbnail(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "famfun_test_thumb")
	os.MkdirAll(dir, 0o755)
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "v1.png"), []byte("fake-png"), 0o644)

	srv := &HomeServer{thumbDir: dir, videos: make(map[string]*model.Video)}

	resp := srv.HandleGetThumbnail(&pb.GetThumbnailRequest{VideoId: "v1"})
	if !resp.Success {
		t.Fatalf("expected success, got error: %s", resp.Error)
	}
	if string(resp.Content) != "fake-png" {
		t.Errorf("content = %q, want %q", resp.Content, "fake-png")
	}
}

func TestHomeServerHandleGetThumbnailNotFound(t *testing.T) {
	srv := &HomeServer{thumbDir: "/nonexistent", videos: make(map[string]*model.Video)}

	resp := srv.HandleGetThumbnail(&pb.GetThumbnailRequest{VideoId: "missing"})
	if resp.Success {
		t.Error("expected failure")
	}
}

func TestHomeServerHandleUpdateVideoInfo(t *testing.T) {
	dir := t.TempDir()
	videoDir := filepath.Join(dir, "v1")
	os.MkdirAll(videoDir, 0o755)

	srv := &HomeServer{streamDir: dir, videos: make(map[string]*model.Video)}
	srv.addVideo(&model.Video{ID: "v1", Filename: "a.mp4", Title: "Old Title", HomeServerID: "h1"})

	resp := srv.HandleUpdateVideoInfo(&pb.UpdateVideoInfoRequest{VideoId: "v1", Title: "New Title"})
	if !resp.Success {
		t.Fatalf("expected success, got error: %s", resp.Error)
	}
	if resp.Video.Title != "New Title" {
		t.Errorf("title = %q, want %q", resp.Video.Title, "New Title")
	}

	videos := srv.getVideos()
	if videos[0].Title != "New Title" {
		t.Errorf("in-memory title = %q, want %q", videos[0].Title, "New Title")
	}

	data, err := os.ReadFile(filepath.Join(videoDir, "metadata.json"))
	if err != nil {
		t.Fatalf("read metadata.json: %v", err)
	}
	if !strings.Contains(string(data), "New Title") {
		t.Errorf("metadata.json should contain new title, got: %s", data)
	}
}

func TestHomeServerHandleUpdateVideoInfoNotFound(t *testing.T) {
	srv := &HomeServer{streamDir: t.TempDir(), videos: make(map[string]*model.Video)}

	resp := srv.HandleUpdateVideoInfo(&pb.UpdateVideoInfoRequest{VideoId: "missing", Title: "X"})
	if resp.Success {
		t.Error("expected failure for missing video")
	}
}

type testReadWriter struct {
	r io.Reader
	w io.Writer
}

func (rw *testReadWriter) Read(p []byte) (int, error)  { return rw.r.Read(p) }
func (rw *testReadWriter) Write(p []byte) (int, error) { return rw.w.Write(p) }

func TestHomeServerHandleUploadVideo(t *testing.T) {
	dir := t.TempDir()
	videoDir := filepath.Join(dir, "videos")
	streamDir := filepath.Join(dir, "streams")
	os.MkdirAll(videoDir, 0o755)
	os.MkdirAll(streamDir, 0o755)

	conn := &mockConnector{}
	converter := &mockConverter{converted: make(map[string]bool)}
	thumbGen := &mockThumbGen{generated: make(map[string]bool)}

	srv := NewHomeServer("h1", "Test", videoDir, streamDir, dir,
		&mockScanner{}, converter, thumbGen, conn)

	fileData := []byte("fake-video-content-for-upload-test")
	hash := sha256.Sum256(fileData)
	hashStr := hex.EncodeToString(hash[:])

	var inputBuf bytes.Buffer
	chunkEnv := &pb.Envelope{
		Payload: &pb.Envelope_UploadVideoChunk{
			UploadVideoChunk: &pb.UploadVideoChunk{
				Data:       fileData,
				ChunkIndex: 0,
			},
		},
	}
	protocol.WriteMessage(&inputBuf, chunkEnv)

	var outputBuf bytes.Buffer
	stream := &testReadWriter{r: &inputBuf, w: &outputBuf}

	req := &pb.UploadVideoRequest{
		Filename:     "test-upload.mp4",
		Filesize:     int64(len(fileData)),
		Sha256:       hashStr,
		HomeServerId: "h1",
		TotalChunks:  1,
	}

	srv.HandleUploadVideo(stream, req)

	resp, err := protocol.ReadMessage(&outputBuf)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	ur := resp.GetUploadVideoResponse()
	if ur == nil {
		t.Fatal("expected UploadVideoResponse")
	}
	if !ur.Success {
		t.Fatalf("expected success, got error: %s", ur.Error)
	}
	if ur.VideoId == "" {
		t.Error("expected non-empty video_id")
	}

	uploadedPath := filepath.Join(videoDir, "test-upload.mp4")
	data, err := os.ReadFile(uploadedPath)
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if string(data) != string(fileData) {
		t.Errorf("file content mismatch")
	}
}

func TestHomeServerHandleUploadVideoHashMismatch(t *testing.T) {
	dir := t.TempDir()
	videoDir := filepath.Join(dir, "videos")
	os.MkdirAll(videoDir, 0o755)

	srv := &HomeServer{videoDir: videoDir, homeID: "h1", videos: make(map[string]*model.Video)}

	var inputBuf bytes.Buffer
	chunkEnv := &pb.Envelope{
		Payload: &pb.Envelope_UploadVideoChunk{
			UploadVideoChunk: &pb.UploadVideoChunk{
				Data:       []byte("some data"),
				ChunkIndex: 0,
			},
		},
	}
	protocol.WriteMessage(&inputBuf, chunkEnv)

	var outputBuf bytes.Buffer
	stream := &testReadWriter{r: &inputBuf, w: &outputBuf}

	req := &pb.UploadVideoRequest{
		Filename:     "bad.mp4",
		Filesize:     9,
		Sha256:       "0000000000000000000000000000000000000000000000000000000000000000",
		HomeServerId: "h1",
		TotalChunks:  1,
	}

	srv.HandleUploadVideo(stream, req)

	resp, err := protocol.ReadMessage(&outputBuf)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	ur := resp.GetUploadVideoResponse()
	if ur == nil {
		t.Fatal("expected UploadVideoResponse")
	}
	if ur.Success {
		t.Error("expected failure for hash mismatch")
	}
	if !strings.Contains(ur.Error, "hash mismatch") {
		t.Errorf("expected hash mismatch error, got: %s", ur.Error)
	}
}
