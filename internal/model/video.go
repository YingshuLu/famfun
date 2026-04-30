package model

import (
	pb "github.com/yingshulu/famfun/pkg/proto"
)

type Video struct {
	ID           string
	Filename     string
	Title        string
	Description  string
	Duration     int64
	Filesize     int64
	Resolution   string
	CreatedAt    string
	HomeServerID string
	Visibility   string
	SourcePath   string
}

type CacheStats struct {
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	Entries   int     `json:"entries"`
	SizeBytes int64   `json:"size_bytes"`
	MaxBytes  int64   `json:"max_bytes"`
	HitRate   float64 `json:"hit_rate"`
}

func VideoToProto(v *Video) *pb.VideoInfo {
	return &pb.VideoInfo{
		Id:           v.ID,
		Filename:     v.Filename,
		Title:        v.Title,
		Description:  v.Description,
		Duration:     v.Duration,
		Filesize:     v.Filesize,
		Resolution:   v.Resolution,
		CreatedAt:    v.CreatedAt,
		HomeServerId: v.HomeServerID,
		Visibility:   v.Visibility,
	}
}

func ProtoToVideo(p *pb.VideoInfo) *Video {
	return &Video{
		ID:           p.Id,
		Filename:     p.Filename,
		Title:        p.Title,
		Description:  p.Description,
		Duration:     p.Duration,
		Filesize:     p.Filesize,
		Resolution:   p.Resolution,
		CreatedAt:    p.CreatedAt,
		HomeServerID: p.HomeServerId,
		Visibility:   p.Visibility,
	}
}

func VideosToProto(vs []*Video) []*pb.VideoInfo {
	result := make([]*pb.VideoInfo, len(vs))
	for i, v := range vs {
		result[i] = VideoToProto(v)
	}
	return result
}

func ProtoToVideos(ps []*pb.VideoInfo) []*Video {
	result := make([]*Video, len(ps))
	for i, p := range ps {
		result[i] = ProtoToVideo(p)
	}
	return result
}
