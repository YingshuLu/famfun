package cloud

import (
	"testing"

	"github.com/quic-go/quic-go"

	"github.com/yingshulu/famfun/internal/model"
)

type mockHomeConn struct {
	id   string
	name string
}

func (m *mockHomeConn) OpenDataStream() (*quic.Stream, error) {
	return nil, nil
}
func (m *mockHomeConn) Close() error    { return nil }
func (m *mockHomeConn) HomeID() string   { return m.id }
func (m *mockHomeConn) HomeName() string { return m.name }

func TestRegisterHome(t *testing.T) {
	mgr := NewHomeManager()
	conn := &mockHomeConn{id: "h1", name: "Home 1"}

	if err := mgr.RegisterHome("h1", "Home 1", conn); err != nil {
		t.Fatalf("RegisterHome: %v", err)
	}
	if mgr.HomeCount() != 1 {
		t.Errorf("HomeCount = %d, want 1", mgr.HomeCount())
	}
}

func TestRegisterHomeDuplicate(t *testing.T) {
	mgr := NewHomeManager()
	conn := &mockHomeConn{id: "h1", name: "Home 1"}

	mgr.RegisterHome("h1", "Home 1", conn)
	err := mgr.RegisterHome("h1", "Home 1", conn)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestUnregisterHome(t *testing.T) {
	mgr := NewHomeManager()
	conn := &mockHomeConn{id: "h1", name: "Home 1"}
	mgr.RegisterHome("h1", "Home 1", conn)
	mgr.UpdateVideos("h1", []*model.Video{{ID: "v1"}})

	mgr.UnregisterHome("h1")

	if mgr.HomeCount() != 0 {
		t.Errorf("HomeCount = %d, want 0", mgr.HomeCount())
	}
	if mgr.VideoCount() != 0 {
		t.Errorf("VideoCount = %d, want 0", mgr.VideoCount())
	}
}

func TestGetHome(t *testing.T) {
	mgr := NewHomeManager()
	conn := &mockHomeConn{id: "h1", name: "Home 1"}
	mgr.RegisterHome("h1", "Home 1", conn)

	got, ok := mgr.GetHome("h1")
	if !ok {
		t.Fatal("expected to find home")
	}
	if got.HomeID() != "h1" {
		t.Errorf("HomeID = %q, want %q", got.HomeID(), "h1")
	}
}

func TestGetHomeNotFound(t *testing.T) {
	mgr := NewHomeManager()

	_, ok := mgr.GetHome("nonexistent")
	if ok {
		t.Fatal("expected home not found")
	}
}

func TestListAllVideos(t *testing.T) {
	mgr := NewHomeManager()
	conn1 := &mockHomeConn{id: "h1", name: "Home 1"}
	conn2 := &mockHomeConn{id: "h2", name: "Home 2"}
	mgr.RegisterHome("h1", "Home 1", conn1)
	mgr.RegisterHome("h2", "Home 2", conn2)

	mgr.UpdateVideos("h1", []*model.Video{{ID: "v1"}, {ID: "v2"}})
	mgr.UpdateVideos("h2", []*model.Video{{ID: "v3"}})

	all := mgr.ListAllVideos()
	if len(all) != 3 {
		t.Errorf("len = %d, want 3", len(all))
	}
}

func TestListAllVideosEmpty(t *testing.T) {
	mgr := NewHomeManager()
	all := mgr.ListAllVideos()
	if len(all) != 0 {
		t.Errorf("len = %d, want 0", len(all))
	}
}

func TestUpdateVideosReplaces(t *testing.T) {
	mgr := NewHomeManager()
	conn := &mockHomeConn{id: "h1", name: "Home 1"}
	mgr.RegisterHome("h1", "Home 1", conn)

	mgr.UpdateVideos("h1", []*model.Video{{ID: "v1"}, {ID: "v2"}})
	if mgr.VideoCount() != 2 {
		t.Errorf("VideoCount = %d, want 2", mgr.VideoCount())
	}

	mgr.UpdateVideos("h1", []*model.Video{{ID: "v3"}})
	if mgr.VideoCount() != 1 {
		t.Errorf("VideoCount = %d, want 1", mgr.VideoCount())
	}
}

func TestFindVideoByID(t *testing.T) {
	mgr := NewHomeManager()
	mgr.UpdateVideos("h1", []*model.Video{{ID: "v1", Title: "Video 1", HomeServerID: "h1"}})
	mgr.UpdateVideos("h2", []*model.Video{{ID: "v2", Title: "Video 2", HomeServerID: "h2"}})

	v, ok := mgr.FindVideoByID("v2")
	if !ok {
		t.Fatal("expected to find v2")
	}
	if v.Title != "Video 2" {
		t.Errorf("title = %q, want %q", v.Title, "Video 2")
	}

	_, ok = mgr.FindVideoByID("missing")
	if ok {
		t.Error("expected not to find missing video")
	}
}

func TestUpdateVideoInfo(t *testing.T) {
	mgr := NewHomeManager()
	mgr.UpdateVideos("h1", []*model.Video{{ID: "v1", Title: "Old", HomeServerID: "h1"}})

	ok := mgr.UpdateVideoInfo("h1", "v1", "New", "desc", "member")
	if !ok {
		t.Fatal("expected update to succeed")
	}

	v, _ := mgr.FindVideoByID("v1")
	if v.Title != "New" {
		t.Errorf("title = %q, want %q", v.Title, "New")
	}
}

func TestUpdateVideoInfoNotFound(t *testing.T) {
	mgr := NewHomeManager()

	ok := mgr.UpdateVideoInfo("h1", "missing", "X", "", "member")
	if ok {
		t.Error("expected update to fail for missing video")
	}
}

func TestListHomes(t *testing.T) {
	mgr := NewHomeManager()
	mgr.RegisterHome("h1", "Home 1", &mockHomeConn{id: "h1", name: "Home 1"})
	mgr.RegisterHome("h2", "Home 2", &mockHomeConn{id: "h2", name: "Home 2"})

	homes := mgr.ListHomes()
	if len(homes) != 2 {
		t.Fatalf("len = %d, want 2", len(homes))
	}

	found := map[string]string{}
	for _, h := range homes {
		found[h.ID] = h.Name
	}
	if found["h1"] != "Home 1" || found["h2"] != "Home 2" {
		t.Errorf("unexpected homes: %v", homes)
	}
}

func TestListHomesEmpty(t *testing.T) {
	mgr := NewHomeManager()
	homes := mgr.ListHomes()
	if len(homes) != 0 {
		t.Errorf("len = %d, want 0", len(homes))
	}
}
