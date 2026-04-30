package cloud

import (
	"fmt"
	"io"
	"math"

	"github.com/yingshulu/famfun/internal/model"
	"github.com/yingshulu/famfun/internal/protocol"
	pb "github.com/yingshulu/famfun/pkg/proto"
)

type StreamProxyService interface {
	GetPlaylist(homeID, videoID string) ([]byte, error)
	GetSegment(homeID, videoID, segmentName string) ([]byte, error)
	GetThumbnail(homeID, videoID string) ([]byte, error)
	UpdateVideoInfo(homeID, videoID, title, description, visibility string) (*model.Video, error)
	UploadVideo(homeID, filename, title, description string, filesize int64, sha256Hash string, body io.Reader) (string, error)
	ScanAndConvert(homeID string) error
}

type StreamProxy struct {
	homeRegistry HomeRegistry
	cache        SegmentCache
}

func NewStreamProxy(registry HomeRegistry, cache SegmentCache) *StreamProxy {
	return &StreamProxy{
		homeRegistry: registry,
		cache:        cache,
	}
}

func (p *StreamProxy) GetPlaylist(homeID, videoID string) ([]byte, error) {
	conn, err := p.getHomeConnection(homeID)
	if err != nil {
		return nil, err
	}

	req := &pb.Envelope{
		Payload: &pb.Envelope_GetPlaylistRequest{
			GetPlaylistRequest: &pb.GetPlaylistRequest{
				VideoId: videoID,
			},
		},
	}

	resp, err := sendDataRequest(conn, req)
	if err != nil {
		return nil, fmt.Errorf("request playlist: %w", err)
	}

	return extractPlaylistContent(resp)
}

func (p *StreamProxy) GetSegment(homeID, videoID, segmentName string) ([]byte, error) {
	cacheKey := buildCacheKey(homeID, videoID, segmentName)

	if data, ok := p.cache.Get(cacheKey); ok {
		return data, nil
	}

	data, err := p.fetchSegmentFromHome(homeID, videoID, segmentName)
	if err != nil {
		return nil, err
	}

	p.cache.Put(cacheKey, data)
	return data, nil
}

func (p *StreamProxy) GetThumbnail(homeID, videoID string) ([]byte, error) {
	cacheKey := buildCacheKey(homeID, videoID, "thumbnail")

	if data, ok := p.cache.Get(cacheKey); ok {
		return data, nil
	}

	conn, err := p.getHomeConnection(homeID)
	if err != nil {
		return nil, err
	}

	req := &pb.Envelope{
		Payload: &pb.Envelope_GetThumbnailRequest{
			GetThumbnailRequest: &pb.GetThumbnailRequest{
				VideoId: videoID,
			},
		},
	}

	resp, err := sendDataRequest(conn, req)
	if err != nil {
		return nil, fmt.Errorf("request thumbnail: %w", err)
	}

	tr := resp.GetGetThumbnailResponse()
	if tr == nil {
		return nil, fmt.Errorf("unexpected response type: %T", resp.Payload)
	}
	if !tr.Success {
		return nil, fmt.Errorf("thumbnail error: %s", tr.Error)
	}

	p.cache.Put(cacheKey, tr.Content)
	return tr.Content, nil
}

func (p *StreamProxy) UpdateVideoInfo(homeID, videoID, title, description, visibility string) (*model.Video, error) {
	conn, err := p.getHomeConnection(homeID)
	if err != nil {
		return nil, err
	}

	req := &pb.Envelope{
		Payload: &pb.Envelope_UpdateVideoInfoRequest{
			UpdateVideoInfoRequest: &pb.UpdateVideoInfoRequest{
				VideoId:     videoID,
				Title:       title,
				Description: description,
				Visibility:  visibility,
			},
		},
	}

	resp, err := sendDataRequest(conn, req)
	if err != nil {
		return nil, fmt.Errorf("request update title: %w", err)
	}

	tr := resp.GetUpdateVideoInfoResponse()
	if tr == nil {
		return nil, fmt.Errorf("unexpected response type: %T", resp.Payload)
	}
	if !tr.Success {
		return nil, fmt.Errorf("update title error: %s", tr.Error)
	}

	return model.ProtoToVideo(tr.Video), nil
}

const uploadChunkSize = 512 * 1024

func (p *StreamProxy) UploadVideo(homeID, filename, title, description string, filesize int64, sha256Hash string, body io.Reader) (string, error) {
	conn, err := p.getHomeConnection(homeID)
	if err != nil {
		return "", err
	}

	stream, err := conn.OpenDataStream()
	if err != nil {
		return "", fmt.Errorf("open data stream: %w", err)
	}
	defer stream.Close()

	totalChunks := uint32(math.Ceil(float64(filesize) / float64(uploadChunkSize)))

	header := &pb.Envelope{
		Payload: &pb.Envelope_UploadVideoRequest{
			UploadVideoRequest: &pb.UploadVideoRequest{
				Filename:     filename,
				Filesize:     filesize,
				Sha256:       sha256Hash,
				HomeServerId: homeID,
				TotalChunks:  totalChunks,
				Title:        title,
				Description:  description,
			},
		},
	}
	if err := protocol.WriteMessage(stream, header); err != nil {
		return "", fmt.Errorf("write upload header: %w", err)
	}

	buf := make([]byte, uploadChunkSize)
	chunkIndex := uint32(0)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			chunk := &pb.Envelope{
				Payload: &pb.Envelope_UploadVideoChunk{
					UploadVideoChunk: &pb.UploadVideoChunk{
						Data:       buf[:n],
						ChunkIndex: chunkIndex,
					},
				},
			}
			if err := protocol.WriteMessage(stream, chunk); err != nil {
				return "", fmt.Errorf("write chunk %d: %w", chunkIndex, err)
			}
			chunkIndex++
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("read body: %w", readErr)
		}
	}

	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return "", fmt.Errorf("read upload response: %w", err)
	}

	ur := resp.GetUploadVideoResponse()
	if ur == nil {
		return "", fmt.Errorf("unexpected response type: %T", resp.Payload)
	}
	if !ur.Success {
		return "", fmt.Errorf("upload error: %s", ur.Error)
	}

	return ur.VideoId, nil
}

func (p *StreamProxy) ScanAndConvert(homeID string) error {
	conn, err := p.getHomeConnection(homeID)
	if err != nil {
		return err
	}

	req := &pb.Envelope{
		Payload: &pb.Envelope_ScanRequest{
			ScanRequest: &pb.ScanRequest{
				HomeServerId: homeID,
			},
		},
	}

	resp, err := sendDataRequest(conn, req)
	if err != nil {
		return fmt.Errorf("request scan: %w", err)
	}

	sr := resp.GetScanResponse()
	if sr == nil {
		return fmt.Errorf("unexpected response type: %T", resp.Payload)
	}
	if !sr.Success {
		return fmt.Errorf("scan error: %s", sr.Error)
	}

	return nil
}

func (p *StreamProxy) getHomeConnection(homeID string) (HomeConnection, error) {
	conn, ok := p.homeRegistry.GetHome(homeID)
	if !ok {
		return nil, fmt.Errorf("home server %q not connected", homeID)
	}
	return conn, nil
}

func (p *StreamProxy) fetchSegmentFromHome(homeID, videoID, segmentName string) ([]byte, error) {
	conn, err := p.getHomeConnection(homeID)
	if err != nil {
		return nil, err
	}

	req := &pb.Envelope{
		Payload: &pb.Envelope_GetSegmentRequest{
			GetSegmentRequest: &pb.GetSegmentRequest{
				VideoId:     videoID,
				SegmentName: segmentName,
			},
		},
	}

	resp, err := sendDataRequest(conn, req)
	if err != nil {
		return nil, fmt.Errorf("request segment: %w", err)
	}

	return extractSegmentContent(resp)
}

func sendDataRequest(conn HomeConnection, req *pb.Envelope) (*pb.Envelope, error) {
	stream, err := conn.OpenDataStream()
	if err != nil {
		return nil, fmt.Errorf("open data stream: %w", err)
	}
	defer stream.Close()

	if err := protocol.WriteMessage(stream, req); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	resp, err := protocol.ReadMessage(stream)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return resp, nil
}

func extractPlaylistContent(resp *pb.Envelope) ([]byte, error) {
	pr := resp.GetGetPlaylistResponse()
	if pr == nil {
		return nil, fmt.Errorf("unexpected response type: %T", resp.Payload)
	}
	if !pr.Success {
		return nil, fmt.Errorf("playlist error: %s", pr.Error)
	}
	return pr.Content, nil
}

func extractSegmentContent(resp *pb.Envelope) ([]byte, error) {
	sr := resp.GetGetSegmentResponse()
	if sr == nil {
		return nil, fmt.Errorf("unexpected response type: %T", resp.Payload)
	}
	if !sr.Success {
		return nil, fmt.Errorf("segment error: %s", sr.Error)
	}
	return sr.Content, nil
}

func buildCacheKey(homeID, videoID, segmentName string) string {
	return homeID + "/" + videoID + "/" + segmentName
}
