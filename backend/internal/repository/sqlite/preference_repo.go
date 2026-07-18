package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
)

var _ repository.PreferenceRepository = (*PreferenceRepo)(nil)

type PreferenceRepo struct {
	db *sql.DB
}

// NewPreferenceRepository instantiates a new sqlite-backed repository for key-value preferences.
func NewPreferenceRepository(db *sql.DB) *PreferenceRepo {
	return &PreferenceRepo{db: db}
}

func (r *PreferenceRepo) Get(key string) (*model.Preference, error) {
	row := r.db.QueryRow(`SELECT key, value FROM preferences WHERE key = ?`, key)
	var p model.Preference
	err := row.Scan(&p.Key, &p.Value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get preference key %s: %w", key, err)
	}
	return &p, nil
}

func (r *PreferenceRepo) Set(key string, value string) error {
	_, err := r.db.Exec(`
		INSERT INTO preferences (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("set preference %s=%s: %w", key, value, err)
	}
	return nil
}

func (r *PreferenceRepo) GetAll() ([]model.Preference, error) {
	rows, err := r.db.Query(`SELECT key, value FROM preferences`)
	if err != nil {
		return nil, fmt.Errorf("query all preferences: %w", err)
	}
	defer rows.Close()

	var list []model.Preference
	for rows.Next() {
		var p model.Preference
		if err := rows.Scan(&p.Key, &p.Value); err != nil {
			return nil, fmt.Errorf("scan preference row: %w", err)
		}
		list = append(list, p)
	}
	return list, nil
}

func (r *PreferenceRepo) Delete(key string) error {
	_, err := r.db.Exec(`DELETE FROM preferences WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete preference key %s: %w", key, err)
	}
	return nil
}
