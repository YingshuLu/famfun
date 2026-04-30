package cloud

import (
	"fmt"
	"sync"

	"github.com/quic-go/quic-go"

	"github.com/yingshulu/famfun/internal/model"
)

type HomeConnection interface {
	OpenDataStream() (*quic.Stream, error)
	Close() error
	HomeID() string
	HomeName() string
}

type HomeInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type HomeRegistry interface {
	RegisterHome(homeID, name string, conn HomeConnection) error
	UnregisterHome(homeID string)
	GetHome(homeID string) (HomeConnection, bool)
	ListAllVideos() []*model.Video
	QueryVideo(homeID, videoID string) (*model.Video, bool)
	UpdateVideos(homeID string, videos []*model.Video)
	UpdateVideoInfo(homeID, videoID, title, description, visibility string) bool
	FindVideoByID(videoID string) (*model.Video, bool)
	ListHomes() []HomeInfo
	HomeCount() int
	VideoCount() int
}

type HomeManager struct {
	mu     sync.RWMutex
	homes  map[string]HomeConnection
	videos map[string][]*model.Video
}

func NewHomeManager() *HomeManager {
	return &HomeManager{
		homes:  make(map[string]HomeConnection),
		videos: make(map[string][]*model.Video),
	}
}

func (m *HomeManager) RegisterHome(homeID, name string, conn HomeConnection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.homes[homeID]; exists {
		return fmt.Errorf("home %q already registered", homeID)
	}
	m.homes[homeID] = conn
	return nil
}

func (m *HomeManager) UnregisterHome(homeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.homes, homeID)
	delete(m.videos, homeID)
}

func (m *HomeManager) GetHome(homeID string) (HomeConnection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, ok := m.homes[homeID]
	return conn, ok
}

func (m *HomeManager) ListAllVideos() []*model.Video {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []*model.Video
	for _, vids := range m.videos {
		all = append(all, vids...)
	}
	return all
}

func (m *HomeManager) QueryVideo(homeID, videoID string) (*model.Video, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vids, ok := m.videos[homeID]
	if !ok {
		return nil, false
	}
	for _, v := range vids {
		if v.ID == videoID {
			return v, true
		}
	}
	return nil, false
}

func (m *HomeManager) UpdateVideos(homeID string, videos []*model.Video) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.videos[homeID] = videos
}

func (m *HomeManager) FindVideoByID(videoID string) (*model.Video, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, vids := range m.videos {
		for _, v := range vids {
			if v.ID == videoID {
				return v, true
			}
		}
	}
	return nil, false
}

func (m *HomeManager) UpdateVideoInfo(homeID, videoID, title, description, visibility string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	vids, ok := m.videos[homeID]
	if !ok {
		return false
	}
	for _, v := range vids {
		if v.ID == videoID {
			v.Title = title
			v.Description = description
			v.Visibility = visibility
			return true
		}
	}
	return false
}

func (m *HomeManager) ListHomes() []HomeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	homes := make([]HomeInfo, 0, len(m.homes))
	for _, conn := range m.homes {
		homes = append(homes, HomeInfo{
			ID:   conn.HomeID(),
			Name: conn.HomeName(),
		})
	}
	return homes
}

func (m *HomeManager) HomeCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.homes)
}

func (m *HomeManager) VideoCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, vids := range m.videos {
		count += len(vids)
	}
	return count
}
