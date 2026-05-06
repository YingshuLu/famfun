package cloud

import (
	"database/sql"
	"fmt"
	"strings"
)

type HomePublicKey struct {
	HomeID    string `json:"home_id"`
	UpdatedAt string `json:"updated_at"`
	PublicKey string `json:"public_key"`
}

func (s *VideoStore) UpsertHomePublicKey(homeID, publicKey string) error {
	homeID = strings.TrimSpace(homeID)
	publicKey = strings.TrimSpace(publicKey)
	if homeID == "" {
		return fmt.Errorf("home_id is required")
	}
	if publicKey == "" {
		return fmt.Errorf("public_key is required")
	}

	_, err := s.db.Exec(`
		INSERT INTO home_public_keys (home_id, public_key, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(home_id) DO UPDATE SET
			public_key = excluded.public_key,
			updated_at = CURRENT_TIMESTAMP
	`, homeID, publicKey)
	return err
}

func (s *VideoStore) GetHomePublicKey(homeID string) (*HomePublicKey, error) {
	row := s.db.QueryRow(
		"SELECT home_id, updated_at, public_key FROM home_public_keys WHERE home_id = ?",
		homeID,
	)

	key := &HomePublicKey{}
	if err := row.Scan(&key.HomeID, &key.UpdatedAt, &key.PublicKey); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("home %q public key not found", homeID)
		}
		return nil, err
	}
	return key, nil
}

func (s *VideoStore) ListHomePublicKeys() ([]HomePublicKey, error) {
	rows, err := s.db.Query(`
		SELECT home_id, updated_at, public_key
		FROM home_public_keys
		ORDER BY home_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []HomePublicKey
	for rows.Next() {
		var key HomePublicKey
		if err := rows.Scan(&key.HomeID, &key.UpdatedAt, &key.PublicKey); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *VideoStore) DeleteHomePublicKey(homeID string) error {
	_, err := s.db.Exec("DELETE FROM home_public_keys WHERE home_id = ?", strings.TrimSpace(homeID))
	return err
}
