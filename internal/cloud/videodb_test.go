package cloud

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *VideoStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewVideoStore(dbPath)
	if err != nil {
		t.Fatalf("NewVideoStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestIncrementPlayCount(t *testing.T) {
	store := newTestStore(t)

	store.IncrementPlayCount("v1")
	store.IncrementPlayCount("v1")
	store.IncrementPlayCount("v1")

	stats, err := store.GetAllVideoStats()
	if err != nil {
		t.Fatalf("GetAllVideoStats: %v", err)
	}
	if stats["v1"].PlayCount != 3 {
		t.Errorf("play_count: got %d, want 3", stats["v1"].PlayCount)
	}
	if stats["v1"].CommentCount != 0 {
		t.Errorf("comment_count: got %d, want 0", stats["v1"].CommentCount)
	}
}

func TestIncrementCommentCount(t *testing.T) {
	store := newTestStore(t)

	store.IncrementCommentCount("v1")
	store.IncrementCommentCount("v1")

	stats, err := store.GetAllVideoStats()
	if err != nil {
		t.Fatalf("GetAllVideoStats: %v", err)
	}
	if stats["v1"].CommentCount != 2 {
		t.Errorf("comment_count: got %d, want 2", stats["v1"].CommentCount)
	}
	if stats["v1"].PlayCount != 0 {
		t.Errorf("play_count: got %d, want 0", stats["v1"].PlayCount)
	}
}

func TestGetAllVideoStatsEmpty(t *testing.T) {
	store := newTestStore(t)

	stats, err := store.GetAllVideoStats()
	if err != nil {
		t.Fatalf("GetAllVideoStats: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected empty map, got %v", stats)
	}
}

func TestGetVideoStats(t *testing.T) {
	store := newTestStore(t)

	store.IncrementPlayCount("v1")
	store.IncrementPlayCount("v1")
	store.IncrementPlayCount("v2")
	store.IncrementCommentCount("v3")

	stats, err := store.GetVideoStats([]string{"v1", "v3"})
	if err != nil {
		t.Fatalf("GetVideoStats: %v", err)
	}
	if stats["v1"].PlayCount != 2 {
		t.Errorf("v1 play: got %d, want 2", stats["v1"].PlayCount)
	}
	if _, ok := stats["v2"]; ok {
		t.Error("v2 should not be in results")
	}
	if stats["v3"].CommentCount != 1 {
		t.Errorf("v3 comment: got %d, want 1", stats["v3"].CommentCount)
	}
}

func TestGetVideoStatsEmpty(t *testing.T) {
	store := newTestStore(t)

	stats, err := store.GetVideoStats([]string{})
	if err != nil {
		t.Fatalf("GetVideoStats: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected empty, got %v", stats)
	}
}

func TestGetAllVideoStatsMultipleVideos(t *testing.T) {
	store := newTestStore(t)

	store.IncrementPlayCount("v1")
	store.IncrementPlayCount("v1")
	store.IncrementCommentCount("v1")
	store.IncrementPlayCount("v2")
	store.IncrementCommentCount("v2")
	store.IncrementCommentCount("v2")
	store.IncrementCommentCount("v2")

	stats, err := store.GetAllVideoStats()
	if err != nil {
		t.Fatalf("GetAllVideoStats: %v", err)
	}
	if stats["v1"].PlayCount != 2 {
		t.Errorf("v1 play: got %d, want 2", stats["v1"].PlayCount)
	}
	if stats["v1"].CommentCount != 1 {
		t.Errorf("v1 comment: got %d, want 1", stats["v1"].CommentCount)
	}
	if stats["v2"].PlayCount != 1 {
		t.Errorf("v2 play: got %d, want 1", stats["v2"].PlayCount)
	}
	if stats["v2"].CommentCount != 3 {
		t.Errorf("v2 comment: got %d, want 3", stats["v2"].CommentCount)
	}
}

func TestAddAndGetComments(t *testing.T) {
	store := newTestStore(t)

	c1, err := store.AddComment("v1", "user1", "Great video!", nil)
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if c1.Content != "Great video!" || c1.VideoID != "v1" || c1.UserID != "user1" {
		t.Errorf("unexpected comment: %+v", c1)
	}

	_, err = store.AddComment("v1", "user1", "Love it", nil)
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	comments, err := store.GetComments("v1")
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("got %d root comments, want 2", len(comments))
	}
	if comments[0].ID != c1.ID {
		t.Errorf("expected oldest first, got id=%d", comments[0].ID)
	}
}

func TestGetCommentsEmpty(t *testing.T) {
	store := newTestStore(t)

	comments, err := store.GetComments("nonexistent")
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected empty, got %d", len(comments))
	}
}

func TestGetCommentsDifferentVideos(t *testing.T) {
	store := newTestStore(t)

	store.AddComment("v1", "user1", "comment on v1", nil)
	store.AddComment("v2", "user1", "comment on v2", nil)

	c1, _ := store.GetComments("v1")
	c2, _ := store.GetComments("v2")

	if len(c1) != 1 || c1[0].Content != "comment on v1" {
		t.Errorf("v1 comments: %+v", c1)
	}
	if len(c2) != 1 || c2[0].Content != "comment on v2" {
		t.Errorf("v2 comments: %+v", c2)
	}
}

func TestCommentTreeNesting(t *testing.T) {
	store := newTestStore(t)

	parent, _ := store.AddComment("v1", "user1", "parent comment", nil)
	store.AddComment("v1", "user2", "reply 1", &parent.ID)
	store.AddComment("v1", "user1", "reply 2", &parent.ID)

	comments, err := store.GetComments("v1")
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d root comments, want 1", len(comments))
	}
	if comments[0].Content != "parent comment" {
		t.Errorf("root content: %q", comments[0].Content)
	}
	if len(comments[0].Replies) != 2 {
		t.Fatalf("got %d replies, want 2", len(comments[0].Replies))
	}
	if comments[0].Replies[0].Content != "reply 1" {
		t.Errorf("reply 1 content: %q", comments[0].Replies[0].Content)
	}
	if comments[0].Replies[1].Content != "reply 2" {
		t.Errorf("reply 2 content: %q", comments[0].Replies[1].Content)
	}
}

func TestCommentTreeNestedReplies(t *testing.T) {
	store := newTestStore(t)

	root, _ := store.AddComment("v1", "user1", "root", nil)
	reply, _ := store.AddComment("v1", "user2", "reply", &root.ID)
	store.AddComment("v1", "user1", "nested reply", &reply.ID)

	comments, err := store.GetComments("v1")
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d roots, want 1", len(comments))
	}
	if len(comments[0].Replies) != 1 {
		t.Fatalf("got %d replies on root, want 1", len(comments[0].Replies))
	}
	if len(comments[0].Replies[0].Replies) != 1 {
		t.Fatalf("got %d nested replies, want 1", len(comments[0].Replies[0].Replies))
	}
	if comments[0].Replies[0].Replies[0].Content != "nested reply" {
		t.Errorf("nested reply content: %q", comments[0].Replies[0].Replies[0].Content)
	}
}

func TestGetCommentsIncludesUsername(t *testing.T) {
	store := newTestStore(t)

	user, err := store.CreateUser("testuser", "password123", "admin")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = store.AddComment("v1", user.ID, "hello", nil)
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	comments, err := store.GetComments("v1")
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
	if comments[0].Username != "testuser" {
		t.Errorf("got username %q, want %q", comments[0].Username, "testuser")
	}
}

func TestGetCommentsAnonymousForUnknownUser(t *testing.T) {
	store := newTestStore(t)

	_, err := store.AddComment("v1", "", "legacy comment", nil)
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	comments, err := store.GetComments("v1")
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if comments[0].Username != "Anonymous" {
		t.Errorf("got username %q, want %q", comments[0].Username, "Anonymous")
	}
}

func TestNewVideoStoreInvalidPath(t *testing.T) {
	_, err := NewVideoStore(filepath.Join(string(os.PathSeparator), "nonexistent", "dir", "test.db"))
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}
