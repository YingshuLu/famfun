package cloud

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	PasswordHash string `json:"-"`
	CreatedAt    string `json:"created_at"`
	LoginAt      string `json:"login_at,omitempty"`
}

func (s *VideoStore) CreateUser(username, password, role string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	if role != "admin" && role != "member" && role != "guest" {
		role = "guest"
	}

	id := uuid.New().String()
	_, err = s.db.Exec(
		"INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)",
		id, username, string(hash), role,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return &User{ID: id, Username: username, Role: role, PasswordHash: string(hash)}, nil
}

func (s *VideoStore) GetUserByUsername(username string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		"SELECT id, username, role, password_hash, created_at FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.Role, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user %q not found", username)
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *VideoStore) GetUserByID(id string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		"SELECT id, username, role, password_hash, created_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Username, &u.Role, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *VideoStore) UpdateLoginAt(userID string) error {
	_, err := s.db.Exec("UPDATE users SET login_at = CURRENT_TIMESTAMP WHERE id = ?", userID)
	return err
}
