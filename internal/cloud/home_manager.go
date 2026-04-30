package cloud

import (
	"fmt"
	"sort"
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
	ListVideosPage(offset, limit int, filter func(*model.Video) bool) (page []*model.Video, total int)
	QueryVideo(homeID, videoID string) (*model.Video, bool)
	UpdateVideos(homeID string, videos []*model.Video)
	HomeVideoIDs(homeID string) map[string]struct{}
	SyncHomeVideos(homeID string, toAdd []*model.Video, toRemove []string)
	UpdateVideoInfo(homeID, videoID, title, description, visibility string) bool
	FindVideoByID(videoID string) (*model.Video, bool)
	ListHomes() []HomeInfo
	HomeCount() int
	VideoCount() int
}

type HomeManager struct {
	mu     sync.RWMutex
	homes  map[string]HomeConnection
	videos map[string]map[string]*model.Video
	sorted []*model.Video
}

func NewHomeManager() *HomeManager {
	return &HomeManager{
		homes:  make(map[string]HomeConnection),
		videos: make(map[string]map[string]*model.Video),
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
	m.rebuildSorted()
}

func (m *HomeManager) GetHome(homeID string) (HomeConnection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, ok := m.homes[homeID]
	return conn, ok
}

func (m *HomeManager) rebuildSorted() {
	var all []*model.Video
	for _, vids := range m.videos {
		for _, v := range vids {
			all = append(all, v)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt != all[j].CreatedAt {
			return all[i].CreatedAt > all[j].CreatedAt
		}
		return all[i].ID < all[j].ID
	})
	m.sorted = all
}

func (m *HomeManager) ListAllVideos() []*model.Video {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*model.Video, len(m.sorted))
	copy(result, m.sorted)
	return result
}

func (m *HomeManager) ListVideosPage(offset, limit int, filter func(*model.Video) bool) (page []*model.Video, total int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	matched := 0
	for _, v := range m.sorted {
		if filter != nil && !filter(v) {
			continue
		}
		if matched >= offset && len(page) < limit {
			page = append(page, v)
		}
		matched++
	}
	return page, matched
}

func (m *HomeManager) QueryVideo(homeID, videoID string) (*model.Video, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vids, ok := m.videos[homeID]
	if !ok {
		return nil, false
	}
	v, ok := vids[videoID]
	return v, ok
}

func (m *HomeManager) UpdateVideos(homeID string, videos []*model.Video) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm := make(map[string]*model.Video, len(videos))
	for _, v := range videos {
		vm[v.ID] = v
	}
	m.videos[homeID] = vm
	m.rebuildSorted()
}

func (m *HomeManager) HomeVideoIDs(homeID string) map[string]struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vids := m.videos[homeID]
	result := make(map[string]struct{}, len(vids))
	for id := range vids {
		result[id] = struct{}{}
	}
	return result
}

func (m *HomeManager) SyncHomeVideos(homeID string, toAdd []*model.Video, toRemove []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vids := m.videos[homeID]
	if vids == nil {
		vids = make(map[string]*model.Video)
		m.videos[homeID] = vids
	}
	for _, id := range toRemove {
		delete(vids, id)
	}
	for _, v := range toAdd {
		vids[v.ID] = v
	}
	if len(toAdd) > 0 || len(toRemove) > 0 {
		m.rebuildSorted()
	}
}

func (m *HomeManager) FindVideoByID(videoID string) (*model.Video, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, vids := range m.videos {
		if v, ok := vids[videoID]; ok {
			return v, true
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
	v, ok := vids[videoID]
	if !ok {
		return false
	}
	v.Title = title
	v.Description = description
	v.Visibility = visibility
	m.rebuildSorted()
	return true
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

	return len(m.sorted)
}
