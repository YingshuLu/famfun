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
func (m *mockHomeConn) Close() error     { return nil }
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

func TestListAllVideosSorted(t *testing.T) {
	mgr := NewHomeManager()
	mgr.UpdateVideos("h1", []*model.Video{
		{ID: "b", CreatedAt: "2026-01-01", HomeServerID: "h1"},
		{ID: "a", CreatedAt: "2026-01-03", HomeServerID: "h1"},
	})
	mgr.UpdateVideos("h2", []*model.Video{
		{ID: "d", CreatedAt: "2026-01-02", HomeServerID: "h2"},
		{ID: "c", CreatedAt: "2026-01-02", HomeServerID: "h2"},
	})

	all := mgr.ListAllVideos()
	if len(all) != 4 {
		t.Fatalf("len = %d, want 4", len(all))
	}

	wantOrder := []string{"a", "c", "d", "b"}
	for i, id := range wantOrder {
		if all[i].ID != id {
			t.Errorf("position %d: got %q, want %q", i, all[i].ID, id)
		}
	}
}

func TestListVideosPage(t *testing.T) {
	mgr := NewHomeManager()
	mgr.UpdateVideos("h1", []*model.Video{
		{ID: "a", CreatedAt: "2026-01-03", Visibility: "guest", HomeServerID: "h1"},
		{ID: "b", CreatedAt: "2026-01-02", Visibility: "member", HomeServerID: "h1"},
		{ID: "c", CreatedAt: "2026-01-01", Visibility: "guest", HomeServerID: "h1"},
		{ID: "d", CreatedAt: "2026-01-04", Visibility: "admin", HomeServerID: "h1"},
		{ID: "e", CreatedAt: "2026-01-05", Visibility: "guest", HomeServerID: "h1"},
	})

	// sorted order: e(05), d(04), a(03), b(02), c(01)
	// guest filter keeps: e, a, c

	page, total := mgr.ListVideosPage(0, 2, func(v *model.Video) bool {
		return v.Visibility == "guest"
	})
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(page) != 2 {
		t.Fatalf("page len = %d, want 2", len(page))
	}
	if page[0].ID != "e" || page[1].ID != "a" {
		t.Errorf("page = [%s, %s], want [e, a]", page[0].ID, page[1].ID)
	}

	page2, total2 := mgr.ListVideosPage(2, 2, func(v *model.Video) bool {
		return v.Visibility == "guest"
	})
	if total2 != 3 {
		t.Errorf("total2 = %d, want 3", total2)
	}
	if len(page2) != 1 || page2[0].ID != "c" {
		t.Errorf("page2 = %v, want [c]", page2)
	}

	// no filter returns all
	all, allTotal := mgr.ListVideosPage(0, 100, nil)
	if allTotal != 5 || len(all) != 5 {
		t.Errorf("no filter: total=%d len=%d, want 5/5", allTotal, len(all))
	}

	// out of range offset
	empty, emptyTotal := mgr.ListVideosPage(10, 5, nil)
	if emptyTotal != 5 || len(empty) != 0 {
		t.Errorf("out of range: total=%d len=%d, want 5/0", emptyTotal, len(empty))
	}
}

func TestSortedUpdatedAfterUnregister(t *testing.T) {
	mgr := NewHomeManager()
	conn := &mockHomeConn{id: "h1", name: "Home 1"}
	mgr.RegisterHome("h1", "Home 1", conn)
	mgr.UpdateVideos("h1", []*model.Video{{ID: "v1", HomeServerID: "h1"}})
	mgr.UpdateVideos("h2", []*model.Video{{ID: "v2", HomeServerID: "h2"}})

	mgr.UnregisterHome("h1")

	all := mgr.ListAllVideos()
	if len(all) != 1 || all[0].ID != "v2" {
		t.Errorf("after unregister: got %v, want [v2]", all)
	}
}

func TestSyncHomeVideos(t *testing.T) {
	mgr := NewHomeManager()
	mgr.UpdateVideos("h1", []*model.Video{
		{ID: "a", CreatedAt: "2026-01-03", HomeServerID: "h1"},
		{ID: "b", CreatedAt: "2026-01-02", HomeServerID: "h1"},
		{ID: "c", CreatedAt: "2026-01-01", HomeServerID: "h1"},
	})

	// Remove "b", add "d"
	mgr.SyncHomeVideos("h1",
		[]*model.Video{{ID: "d", CreatedAt: "2026-01-04", HomeServerID: "h1"}},
		[]string{"b"},
	)

	if mgr.VideoCount() != 3 {
		t.Fatalf("VideoCount = %d, want 3", mgr.VideoCount())
	}

	_, ok := mgr.QueryVideo("h1", "b")
	if ok {
		t.Error("expected 'b' to be removed")
	}

	d, ok := mgr.QueryVideo("h1", "d")
	if !ok || d.ID != "d" {
		t.Error("expected 'd' to be added")
	}

	// Sorted order should be: d(04), a(03), c(01)
	all := mgr.ListAllVideos()
	if len(all) != 3 {
		t.Fatalf("len = %d, want 3", len(all))
	}
	if all[0].ID != "d" || all[1].ID != "a" || all[2].ID != "c" {
		t.Errorf("order = [%s, %s, %s], want [d, a, c]", all[0].ID, all[1].ID, all[2].ID)
	}
}

func TestSyncHomeVideosNoChanges(t *testing.T) {
	mgr := NewHomeManager()
	mgr.UpdateVideos("h1", []*model.Video{
		{ID: "a", HomeServerID: "h1"},
	})

	// No additions or removals — should be a no-op
	mgr.SyncHomeVideos("h1", nil, nil)

	if mgr.VideoCount() != 1 {
		t.Errorf("VideoCount = %d, want 1", mgr.VideoCount())
	}
}

func TestSyncHomeVideosNewHome(t *testing.T) {
	mgr := NewHomeManager()

	// Sync to a home that has no cached videos yet
	mgr.SyncHomeVideos("h1",
		[]*model.Video{{ID: "a", HomeServerID: "h1"}},
		nil,
	)

	if mgr.VideoCount() != 1 {
		t.Errorf("VideoCount = %d, want 1", mgr.VideoCount())
	}
	v, ok := mgr.QueryVideo("h1", "a")
	if !ok || v.ID != "a" {
		t.Error("expected 'a' to be added")
	}
}
