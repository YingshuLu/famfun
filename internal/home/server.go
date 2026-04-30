package home

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yingshulu/famfun/internal/model"
	"github.com/yingshulu/famfun/internal/protocol"
	pb "github.com/yingshulu/famfun/pkg/proto"
)

type HomeServer struct {
	homeID          string
	homeName        string
	videoDir        string
	streamDir       string
	thumbDir        string
	scanner         VideoScanner
	converter       HLSConverter
	thumbGen        ThumbnailGenerator
	connector       CloudConnector
	videos          map[string]*model.Video
	mu              sync.RWMutex
	videoProcessors []func(*model.Video)
}

func NewHomeServer(
	homeID, homeName string,
	videoDir, streamDir, thumbDir string,
	scanner VideoScanner,
	converter HLSConverter,
	thumbGen ThumbnailGenerator,
	connector CloudConnector,
) *HomeServer {
	s := &HomeServer{
		homeID:    homeID,
		homeName:  homeName,
		videoDir:  videoDir,
		streamDir: streamDir,
		thumbDir:  thumbDir,
		scanner:   scanner,
		converter: converter,
		thumbGen:  thumbGen,
		connector: connector,
		videos:    make(map[string]*model.Video),
	}
	s.videoProcessors = []func(*model.Video){
		s.convertVideo,
		s.generateThumbnail,
		s.addVideo,
		s.notifyCloud,
	}
	return s
}

func (s *HomeServer) Run(ctx context.Context, cloudAddr string) error {
	s.connector.SetMessageHandler(s)

	var reconnect bool
	for {
		if err := ConnectWithRetry(ctx, s.connector, cloudAddr); err != nil {
			return fmt.Errorf("connect to cloud: %w", err)
		}

		if err := s.register(); err != nil {
			return fmt.Errorf("register: %w", err)
		}

		if reconnect {
			s.publishAllVideos()
		} else {
			go s.scanAndConvertBackground(ctx)
		}
		
		s.startHeartbeat(ctx)
		
		select {
			case <-ctx.Done():
				return s.connector.Close()
			case <-s.connector.Disconnected():
				log.Printf("disconnected from cloud, reconnecting...")
				s.connector.Close()
				log.Printf("reconnected to cloud")
				reconnect = true
		}
	
	}
}

func (s *HomeServer) scanAndConvertBackground(ctx context.Context) {
	videos, err := s.scanner.Scan(s.videoDir, s.streamDir)
	if err != nil {
		log.Printf("scan error: %v", err)
		return
	}

	log.Printf("found %d video files, processing...", len(videos))

	for _, v := range videos {
		if ctx.Err() != nil {
			return
		}
		v.HomeServerID = s.homeID
		s.processAndPublish(v)
	}

	log.Printf("finished processing all %d videos", len(videos))
}

func (s *HomeServer) processAndPublish(v *model.Video) {
	for _, p := range s.videoProcessors {
		p(v)
	}
}

func (s *HomeServer) addVideo(v *model.Video) {
	if v.Visibility == "" {
		v.Visibility = "member"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.videos[v.ID] = v
}

func (s *HomeServer) publishAllVideos() {
	ids := s.getVideoIDs()
	if len(ids) == 0 {
		return
	}
	env := &pb.Envelope{
		Payload: &pb.Envelope_VideoListUpdate{
			VideoListUpdate: &pb.VideoListUpdate{
				HomeServerId: s.homeID,
				VideoIds:     ids,
			},
		},
	}
	if err := s.connector.SendEnvelope(env); err != nil {
		log.Printf("publish all videos error: %v", err)
	}
}

func (s *HomeServer) notifyCloud(v *model.Video) {
	env := &pb.Envelope{
		Payload: &pb.Envelope_VideoListUpdate{
			VideoListUpdate: &pb.VideoListUpdate{
				HomeServerId: s.homeID,
				VideoIds:     []string{v.ID},
			},
		},
	}
	if err := s.connector.SendEnvelope(env); err != nil {
		log.Printf("notify cloud error for %s: %v", v.Filename, err)
	}
}

func (s *HomeServer) getVideoIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.videos))
	for id := range s.videos {
		ids = append(ids, id)
	}
	return ids
}

func (s *HomeServer) getVideos() []*model.Video {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*model.Video, 0, len(s.videos))
	for _, v := range s.videos {
		result = append(result, v)
	}
	return result
}

func (s *HomeServer) convertVideo(v *model.Video) {
	if s.converter.IsConverted(v, s.streamDir) {
		return
	}
	log.Printf("converting %s to HLS...", v.Filename)
	if err := s.converter.Convert(v, s.streamDir); err != nil {
		log.Printf("conversion error for %s: %v", v.Filename, err)
	}
}

func (s *HomeServer) generateThumbnail(v *model.Video) {
	if s.thumbGen.IsGenerated(v, s.thumbDir) {
		return
	}
	log.Printf("generating thumbnail for %s...", v.Filename)
	if _, err := s.thumbGen.Generate(v, s.thumbDir); err != nil {
		log.Printf("thumbnail error for %s: %v", v.Filename, err)
	}
}

func (s *HomeServer) register() error {
	env := &pb.Envelope{
		Payload: &pb.Envelope_RegisterRequest{
			RegisterRequest: &pb.RegisterRequest{
				HomeServerId: s.homeID,
				Name:         s.homeName,
			},
		},
	}
	return s.connector.SendEnvelope(env)
}

func (s *HomeServer) startHeartbeat(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.sendHeartbeat()
			}
		}
	}()
}

func (s *HomeServer) sendHeartbeat() {
	env := &pb.Envelope{
		Payload: &pb.Envelope_HeartbeatRequest{
			HeartbeatRequest: &pb.HeartbeatRequest{
				HomeServerId: s.homeID,
				VideoIds:     s.getVideoIDs(),
			},
		},
	}
	if err := s.connector.SendEnvelope(env); err != nil {
		log.Printf("heartbeat error: %v", err)
	}
}

func (s *HomeServer) HandleGetPlaylist(req *pb.GetPlaylistRequest) *pb.GetPlaylistResponse {
	playlistPath := filepath.Join(s.streamDir, req.VideoId, "index.m3u8")
	data, err := os.ReadFile(playlistPath)
	if err != nil {
		return &pb.GetPlaylistResponse{
			RequestId: req.RequestId,
			Success:   false,
			Error:     fmt.Sprintf("read playlist: %v", err),
		}
	}

	return &pb.GetPlaylistResponse{
		RequestId: req.RequestId,
		Success:   true,
		Content:   data,
	}
}

func (s *HomeServer) HandleGetVideoInfo(req *pb.GetVideoInfoRequest) *pb.GetVideoInfoResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.videos[req.VideoId]
	if !ok {
		return &pb.GetVideoInfoResponse{
			Success: false,
			Error:   fmt.Sprintf("video %q not found", req.VideoId),
		}
	}
	return &pb.GetVideoInfoResponse{
		Success: true,
		Video:   model.VideoToProto(v),
	}
}

func (s *HomeServer) HandleGetThumbnail(req *pb.GetThumbnailRequest) *pb.GetThumbnailResponse {
	path := filepath.Join(s.thumbDir, req.VideoId+".png")
	data, err := os.ReadFile(path)
	if err != nil {
		return &pb.GetThumbnailResponse{
			Success: false,
			Error:   fmt.Sprintf("read thumbnail: %v", err),
		}
	}
	return &pb.GetThumbnailResponse{
		Success: true,
		Content: data,
	}
}

func (s *HomeServer) HandleUpdateVideoInfo(req *pb.UpdateVideoInfoRequest) *pb.UpdateVideoInfoResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.videos[req.VideoId]
	if !ok {
		return &pb.UpdateVideoInfoResponse{
			Success: false,
			Error:   fmt.Sprintf("video %q not found", req.VideoId),
		}
	}
	v.Title = req.Title
	v.Description = req.Description
	v.Visibility = req.Visibility
	if v.Visibility == "" {
		v.Visibility = "member"
	}
	saveCachedMetadata(s.streamDir, v.ID, v)
	return &pb.UpdateVideoInfoResponse{
		Success: true,
		Video:   model.VideoToProto(v),
	}
}

func (s *HomeServer) HandleGetSegment(req *pb.GetSegmentRequest) *pb.GetSegmentResponse {
	segmentPath := filepath.Join(s.streamDir, req.VideoId, req.SegmentName)
	data, err := os.ReadFile(segmentPath)
	if err != nil {
		return &pb.GetSegmentResponse{
			RequestId: req.RequestId,
			Success:   false,
			Error:     fmt.Sprintf("read segment: %v", err),
		}
	}

	return &pb.GetSegmentResponse{
		RequestId: req.RequestId,
		Success:   true,
		Content:   data,
	}
}

func (s *HomeServer) HandleUploadVideo(stream io.ReadWriter, req *pb.UploadVideoRequest) {
	sendResponse := func(success bool, errMsg, videoID string) {
		resp := &pb.Envelope{
			Payload: &pb.Envelope_UploadVideoResponse{
				UploadVideoResponse: &pb.UploadVideoResponse{
					Success: success,
					Error:   errMsg,
					VideoId: videoID,
				},
			},
		}
		protocol.WriteMessage(stream, resp)
	}

	tmpFile, err := os.CreateTemp(s.videoDir, "upload-*.tmp")
	if err != nil {
		sendResponse(false, fmt.Sprintf("create temp file: %v", err), "")
		return
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	for i := uint32(0); i < req.TotalChunks; i++ {
		env, err := protocol.ReadMessage(stream)
		if err != nil {
			sendResponse(false, fmt.Sprintf("read chunk %d: %v", i, err), "")
			return
		}
		chunk := env.GetUploadVideoChunk()
		if chunk == nil {
			sendResponse(false, fmt.Sprintf("expected UploadVideoChunk, got %T", env.Payload), "")
			return
		}
		if _, err := writer.Write(chunk.Data); err != nil {
			sendResponse(false, fmt.Sprintf("write chunk %d: %v", i, err), "")
			return
		}
	}

	tmpFile.Close()

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != req.Sha256 {
		sendResponse(false, fmt.Sprintf("hash mismatch: expected %s, got %s", req.Sha256, actualHash), "")
		return
	}

	destPath := s.uniqueFilePath(req.Filename)
	if err := os.Rename(tmpPath, destPath); err != nil {
		sendResponse(false, fmt.Sprintf("move file: %v", err), "")
		return
	}

	relPath, _ := filepath.Rel(s.videoDir, destPath)
	videoID := generateVideoID(relPath)

	sendResponse(true, "", videoID)

	go func() {
		probe, err := runFFProbe(destPath)
		if err != nil {
			log.Printf("probe uploaded video %s: %v", req.Filename, err)
			return
		}
		video := buildVideoFromProbe(videoID, relPath, destPath, probe)
		video.HomeServerID = s.homeID
		if req.Title != "" {
			video.Title = req.Title
		}
		video.Description = req.Description
		s.processAndPublish(video)
	}()
}

func (s *HomeServer) HandleScan(req *pb.ScanRequest) *pb.ScanResponse {
	go s.scanAndConvertBackground(context.Background())
	return &pb.ScanResponse{Success: true}
}

func (s *HomeServer) uniqueFilePath(filename string) string {
	dest := filepath.Join(s.videoDir, filename)
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return dest
	}

	ext := filepath.Ext(filename)
	base := filename[:len(filename)-len(ext)]
	for i := 1; ; i++ {
		dest = filepath.Join(s.videoDir, fmt.Sprintf("%s(%d)%s", base, i, ext))
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			return dest
		}
	}
}
