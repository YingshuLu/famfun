package cloud

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

type Comment struct {
	ID        int        `json:"id"`
	VideoID   string     `json:"video_id"`
	UserID    string     `json:"user_id"`
	Username  string     `json:"username"`
	ParentID  *int       `json:"parent_id,omitempty"`
	Content   string     `json:"content"`
	CreatedAt string     `json:"created_at"`
	Replies   []*Comment `json:"replies"`
}

type VideoStore struct {
	db *sql.DB
}

func NewVideoStore(dbPath string) (*VideoStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	store := &VideoStore{db: db}
	store.seedDemoUser()
	return store, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS video_stats (
			video_id TEXT PRIMARY KEY,
			play_count INTEGER NOT NULL DEFAULT 0,
			comment_count INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'guest',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			login_at DATETIME
		);
		CREATE TABLE IF NOT EXISTS video_comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			video_id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT '',
			parent_id INTEGER REFERENCES video_comments(id),
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS home_public_keys (
			home_id TEXT PRIMARY KEY,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			public_key TEXT NOT NULL
		);
	`)
	return err
}

type VideoStats struct {
	PlayCount    int
	CommentCount int
}

func (s *VideoStore) IncrementPlayCount(videoID string) error {
	_, err := s.db.Exec(`
		INSERT INTO video_stats (video_id, play_count) VALUES (?, 1)
		ON CONFLICT(video_id) DO UPDATE SET play_count = play_count + 1
	`, videoID)
	return err
}

func (s *VideoStore) IncrementCommentCount(videoID string) error {
	_, err := s.db.Exec(`
		INSERT INTO video_stats (video_id, comment_count) VALUES (?, 1)
		ON CONFLICT(video_id) DO UPDATE SET comment_count = comment_count + 1
	`, videoID)
	return err
}

func (s *VideoStore) GetAllVideoStats() (map[string]VideoStats, error) {
	rows, err := s.db.Query("SELECT video_id, play_count, comment_count FROM video_stats")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]VideoStats)
	for rows.Next() {
		var id string
		var vs VideoStats
		if err := rows.Scan(&id, &vs.PlayCount, &vs.CommentCount); err != nil {
			return nil, err
		}
		stats[id] = vs
	}
	return stats, rows.Err()
}

func (s *VideoStore) GetVideoStats(videoIDs []string) (map[string]VideoStats, error) {
	if len(videoIDs) == 0 {
		return map[string]VideoStats{}, nil
	}

	query := "SELECT video_id, play_count, comment_count FROM video_stats WHERE video_id IN (?" + strings.Repeat(",?", len(videoIDs)-1) + ")"
	args := make([]any, len(videoIDs))
	for i, id := range videoIDs {
		args[i] = id
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]VideoStats, len(videoIDs))
	for rows.Next() {
		var id string
		var vs VideoStats
		if err := rows.Scan(&id, &vs.PlayCount, &vs.CommentCount); err != nil {
			return nil, err
		}
		stats[id] = vs
	}
	return stats, rows.Err()
}

func (s *VideoStore) AddComment(videoID, userID, content string, parentID *int) (*Comment, error) {
	result, err := s.db.Exec(
		"INSERT INTO video_comments (video_id, user_id, parent_id, content) VALUES (?, ?, ?, ?)",
		videoID, userID, parentID, content,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	row := s.db.QueryRow("SELECT created_at FROM video_comments WHERE id = ?", id)
	var createdAt string
	row.Scan(&createdAt)

	return &Comment{
		ID:        int(id),
		VideoID:   videoID,
		UserID:    userID,
		ParentID:  parentID,
		Content:   content,
		CreatedAt: createdAt,
		Replies:   []*Comment{},
	}, nil
}

func (s *VideoStore) GetComments(videoID string) ([]*Comment, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.video_id, c.user_id, COALESCE(u.username, 'Anonymous'), c.parent_id, c.content, c.created_at
		FROM video_comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.video_id = ?
		ORDER BY c.id ASC
	`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byID := make(map[int]*Comment)
	var roots []*Comment

	for rows.Next() {
		c := &Comment{Replies: []*Comment{}}
		if err := rows.Scan(&c.ID, &c.VideoID, &c.UserID, &c.Username, &c.ParentID, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		byID[c.ID] = c
		if c.ParentID == nil {
			roots = append(roots, c)
		} else if parent, ok := byID[*c.ParentID]; ok {
			parent.Replies = append(parent.Replies, c)
		} else {
			roots = append(roots, c)
		}
	}
	if roots == nil {
		roots = []*Comment{}
	}
	return roots, rows.Err()
}

func (s *VideoStore) seedDemoUser() {
	if _, err := s.GetUserByUsername("kamlu"); err != nil {
		s.CreateUser("kamlu", "LUtrend#1.", "admin")
	}
}

func (s *VideoStore) Close() error {
	return s.db.Close()
}
