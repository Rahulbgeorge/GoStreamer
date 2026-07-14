package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
)

var _ repository.MediaRepository = (*MediaRepo)(nil)

type MediaRepo struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepo {
	return &MediaRepo{db: db}
}

// columns is the standard column list for SELECT queries.
const columns = `id, title, original_name, year, quality, genre, file_path, file_size, 
	duration, mime_type, thumbnail_path, status, source, language, 
	created_at, updated_at`

// scanMedia scans a single row into a model.Media struct.
func scanMedia(scanner interface{ Scan(dest ...any) error }) (*model.Media, error) {
	var m model.Media
	var createdAtStr, updatedAtStr string

	err := scanner.Scan(
		&m.ID, &m.Title, &m.OriginalName, &m.Year, &m.Quality, &m.Genre, &m.FilePath, &m.FileSize,
		&m.Duration, &m.MimeType, &m.ThumbnailPath, &m.Status, &m.Source, &m.Language,
		&createdAtStr, &updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)

	return &m, nil
}

// scanMediaRows scans multiple rows into a slice of model.Media.
func scanMediaRows(rows *sql.Rows) ([]model.Media, error) {
	var list []model.Media
	for rows.Next() {
		m, err := scanMedia(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *m)
	}
	return list, nil
}

func (r *MediaRepo) FindByID(id string) (*model.Media, error) {
	row := r.db.QueryRow(`SELECT `+columns+` FROM media WHERE id = ?`, id)

	m, err := scanMedia(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan media by id %s: %w", id, err)
	}
	return m, nil
}

func (r *MediaRepo) FindAll(limit, offset int) ([]model.Media, error) {
	rows, err := r.db.Query(`SELECT `+columns+` FROM media ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query all media: %w", err)
	}
	defer rows.Close()

	list, err := scanMediaRows(rows)
	if err != nil {
		return nil, fmt.Errorf("scan media row: %w", err)
	}
	return list, nil
}

func (r *MediaRepo) FindByStatus(status model.MediaStatus) ([]model.Media, error) {
	rows, err := r.db.Query(`SELECT `+columns+` FROM media WHERE status = ? ORDER BY created_at DESC`, status)
	if err != nil {
		return nil, fmt.Errorf("query media by status: %w", err)
	}
	defer rows.Close()

	list, err := scanMediaRows(rows)
	if err != nil {
		return nil, fmt.Errorf("scan media row by status: %w", err)
	}
	return list, nil
}

func (r *MediaRepo) FindByFilePath(path string) (*model.Media, error) {
	row := r.db.QueryRow(`SELECT `+columns+` FROM media WHERE file_path = ?`, path)

	m, err := scanMedia(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan media by file path %s: %w", path, err)
	}
	return m, nil
}

func (r *MediaRepo) Search(query string, limit, offset int) ([]model.Media, error) {
	rows, err := r.db.Query(`
		SELECT `+columns+`
		FROM media 
		WHERE title LIKE ? OR original_name LIKE ?
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?`, "%"+query+"%", "%"+query+"%", limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query media by search: %w", err)
	}
	defer rows.Close()

	list, err := scanMediaRows(rows)
	if err != nil {
		return nil, fmt.Errorf("scan media row from search: %w", err)
	}
	return list, nil
}

func (r *MediaRepo) Create(m *model.Media) error {
	now := time.Now().UTC()
	m.CreatedAt = now
	m.UpdatedAt = now

	_, err := r.db.Exec(`
		INSERT INTO media (
			id, title, original_name, year, quality, genre, file_path, file_size, 
			duration, mime_type, thumbnail_path, status, source, language, 
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Title, m.OriginalName, m.Year, m.Quality, m.Genre, m.FilePath, m.FileSize,
		m.Duration, m.MimeType, m.ThumbnailPath, m.Status, m.Source, m.Language,
		m.CreatedAt.Format(time.RFC3339), m.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert media %s: %w", m.ID, err)
	}
	return nil
}

func (r *MediaRepo) Update(m *model.Media) error {
	m.UpdatedAt = time.Now().UTC()

	_, err := r.db.Exec(`
		UPDATE media SET 
			title = ?, original_name = ?, year = ?, quality = ?, genre = ?, file_path = ?, 
			file_size = ?, duration = ?, mime_type = ?, thumbnail_path = ?, 
			status = ?, source = ?, language = ?, updated_at = ?
		WHERE id = ?`,
		m.Title, m.OriginalName, m.Year, m.Quality, m.Genre, m.FilePath, m.FileSize,
		m.Duration, m.MimeType, m.ThumbnailPath, m.Status, m.Source, m.Language,
		m.UpdatedAt.Format(time.RFC3339), m.ID,
	)
	if err != nil {
		return fmt.Errorf("update media %s: %w", m.ID, err)
	}
	return nil
}

func (r *MediaRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM media WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete media %s: %w", id, err)
	}
	return nil
}

func (r *MediaRepo) Count() (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM media`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count media: %w", err)
	}
	return count, nil
}

func (r *MediaRepo) TotalSize() (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRow(`SELECT SUM(file_size) FROM media`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum file size of media: %w", err)
	}
	if total.Valid {
		return total.Int64, nil
	}
	return 0, nil
}
