package sqlite

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
)

var _ repository.MediaRepository = (*MediaRepo)(nil)

type MediaRepo struct {
	db            *sql.DB
	mu            sync.RWMutex
	items         map[string]*model.Media
	list          []model.Media
	initialized   bool
	progressCache map[string]int // dirty playback progress cache: mediaID -> position in seconds
	progressMu    sync.Mutex
}

func NewMediaRepository(db *sql.DB) *MediaRepo {
	repo := &MediaRepo{
		db:            db,
		items:         make(map[string]*model.Media),
		progressCache: make(map[string]int),
	}
	repo.startProgressFlusher()
	return repo
}

// columns is the standard column list for SELECT queries.
const columns = `id, title, original_name, year, quality, genre, file_path, file_size, 
	duration, mime_type, thumbnail_path, status, source, language, last_position, 
	created_at, updated_at`

// scanMedia scans a single row into a model.Media struct.
func scanMedia(scanner interface{ Scan(dest ...any) error }) (*model.Media, error) {
	var m model.Media
	var createdAtStr, updatedAtStr string

	err := scanner.Scan(
		&m.ID, &m.Title, &m.OriginalName, &m.Year, &m.Quality, &m.Genre, &m.FilePath, &m.FileSize,
		&m.Duration, &m.MimeType, &m.ThumbnailPath, &m.Status, &m.Source, &m.Language, &m.LastPosition,
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

// startProgressFlusher starts a background ticker that runs every 10 seconds to flush dirty playback positions to DB.
func (r *MediaRepo) startProgressFlusher() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			r.flushProgressToDB()
		}
	}()
}

func (r *MediaRepo) flushProgressToDB() {
	r.progressMu.Lock()
	if len(r.progressCache) == 0 {
		r.progressMu.Unlock()
		return
	}

	// Copy dirty progress entries and reset in-memory progress cache map
	dirty := make(map[string]int, len(r.progressCache))
	for k, v := range r.progressCache {
		dirty[k] = v
	}
	r.progressCache = make(map[string]int)
	r.progressMu.Unlock()

	nowStr := time.Now().UTC().Format(time.RFC3339)
	stmt, err := r.db.Prepare(`UPDATE media SET last_position = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		slog.Error("Failed to prepare progress update statement", "err", err)
		return
	}
	defer stmt.Close()

	for id, pos := range dirty {
		if _, err := stmt.Exec(pos, nowStr, id); err != nil {
			slog.Error("Failed to flush media playback progress to DB", "id", id, "pos", pos, "err", err)
		}
	}
	slog.Debug("Flushed media playback progress to DB", "count", len(dirty))
}

func (r *MediaRepo) ensureCache() error {
	if r.initialized {
		return nil
	}
	rows, err := r.db.Query(`SELECT ` + columns + ` FROM media ORDER BY created_at DESC`)
	if err != nil {
		return fmt.Errorf("query all media for cache: %w", err)
	}
	defer rows.Close()

	list, err := scanMediaRows(rows)
	if err != nil {
		return fmt.Errorf("scan media rows for cache: %w", err)
	}

	r.list = list
	r.items = make(map[string]*model.Media, len(list))
	for i := range list {
		item := list[i]
		r.items[item.ID] = &item
	}
	r.initialized = true
	return nil
}

func (r *MediaRepo) FindByID(id string) (*model.Media, error) {
	r.mu.RLock()
	if r.initialized {
		if item, found := r.items[id]; found {
			cp := *item
			r.mu.RUnlock()
			return &cp, nil
		}
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureCache(); err != nil {
		return nil, err
	}
	if item, found := r.items[id]; found {
		cp := *item
		return &cp, nil
	}
	return nil, nil
}

func (r *MediaRepo) FindAll(limit, offset int) ([]model.Media, error) {
	r.mu.RLock()
	if r.initialized && limit == 0 && offset == 0 {
		cp := make([]model.Media, len(r.list))
		copy(cp, r.list)
		r.mu.RUnlock()
		return cp, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureCache(); err != nil {
		return nil, err
	}

	if limit == 0 && offset == 0 {
		cp := make([]model.Media, len(r.list))
		copy(cp, r.list)
		return cp, nil
	}

	n := len(r.list)
	if offset >= n {
		return []model.Media{}, nil
	}
	end := offset + limit
	if end > n || limit <= 0 {
		end = n
	}
	cp := make([]model.Media, end-offset)
	copy(cp, r.list[offset:end])
	return cp, nil
}

func (r *MediaRepo) FindByStatus(status model.MediaStatus) ([]model.Media, error) {
	r.mu.RLock()
	if r.initialized {
		var res []model.Media
		for _, item := range r.list {
			if item.Status == status {
				res = append(res, item)
			}
		}
		r.mu.RUnlock()
		return res, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureCache(); err != nil {
		return nil, err
	}
	var res []model.Media
	for _, item := range r.list {
		if item.Status == status {
			res = append(res, item)
		}
	}
	return res, nil
}

func (r *MediaRepo) FindByFilePath(path string) (*model.Media, error) {
	r.mu.RLock()
	if r.initialized {
		for _, item := range r.items {
			if item.FilePath == path {
				cp := *item
				r.mu.RUnlock()
				return &cp, nil
			}
		}
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureCache(); err != nil {
		return nil, err
	}
	for _, item := range r.items {
		if item.FilePath == path {
			cp := *item
			return &cp, nil
		}
	}
	return nil, nil
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

	// 1. Write to database first
	_, err := r.db.Exec(`
		INSERT INTO media (
			id, title, original_name, year, quality, genre, file_path, file_size, 
			duration, mime_type, thumbnail_path, status, source, language, last_position, 
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Title, m.OriginalName, m.Year, m.Quality, m.Genre, m.FilePath, m.FileSize,
		m.Duration, m.MimeType, m.ThumbnailPath, m.Status, m.Source, m.Language, m.LastPosition,
		m.CreatedAt.Format(time.RFC3339), m.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert media %s: %w", m.ID, err)
	}

	// 2. Write-Through: Update in-memory cache immediately
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.ensureCache()

	cp := *m
	r.items[m.ID] = &cp

	exists := false
	for i := range r.list {
		if r.list[i].ID == m.ID {
			r.list[i] = cp
			exists = true
			break
		}
	}
	if !exists {
		r.list = append([]model.Media{cp}, r.list...)
	}

	return nil
}

func (r *MediaRepo) Update(m *model.Media) error {
	m.UpdatedAt = time.Now().UTC()

	// 1. Write to database first
	_, err := r.db.Exec(`
		UPDATE media SET 
			title = ?, original_name = ?, year = ?, quality = ?, genre = ?, file_path = ?, 
			file_size = ?, duration = ?, mime_type = ?, thumbnail_path = ?, 
			status = ?, source = ?, language = ?, last_position = ?, updated_at = ?
		WHERE id = ?`,
		m.Title, m.OriginalName, m.Year, m.Quality, m.Genre, m.FilePath, m.FileSize,
		m.Duration, m.MimeType, m.ThumbnailPath, m.Status, m.Source, m.Language,
		m.LastPosition, m.UpdatedAt.Format(time.RFC3339), m.ID,
	)
	if err != nil {
		return fmt.Errorf("update media %s: %w", m.ID, err)
	}

	// 2. Write-Through: Update in-memory cache immediately
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.ensureCache()

	cp := *m
	r.items[m.ID] = &cp
	for i := range r.list {
		if r.list[i].ID == m.ID {
			r.list[i] = cp
			break
		}
	}
	return nil
}

// UpdateProgress updates the playback position in RAM memory cache immediately and marks it dirty for the 10s background flusher.
func (r *MediaRepo) UpdateProgress(id string, position int) error {
	if id == "" {
		return nil
	}

	// 1. Store in memory progressCache map
	r.progressMu.Lock()
	r.progressCache[id] = position
	r.progressMu.Unlock()

	// 2. Immediately update in-memory item & list structs so any GET query reads the new position
	r.mu.Lock()
	if r.initialized {
		if item, found := r.items[id]; found {
			item.LastPosition = position
		}
		for i := range r.list {
			if r.list[i].ID == id {
				r.list[i].LastPosition = position
				break
			}
		}
	}
	r.mu.Unlock()

	return nil
}

func (r *MediaRepo) Delete(id string) error {
	// 1. Write to database first
	_, err := r.db.Exec(`DELETE FROM media WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete media %s: %w", id, err)
	}

	// 2. Write-Through: Evict from in-memory cache immediately
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.initialized {
		delete(r.items, id)
		newList := make([]model.Media, 0, len(r.list))
		for _, item := range r.list {
			if item.ID != id {
				newList = append(newList, item)
			}
		}
		r.list = newList
	}
	return nil
}

func (r *MediaRepo) Count() (int, error) {
	r.mu.RLock()
	if r.initialized {
		n := len(r.list)
		r.mu.RUnlock()
		return n, nil
	}
	r.mu.RUnlock()

	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM media`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count media: %w", err)
	}
	return count, nil
}

func (r *MediaRepo) TotalSize() (int64, error) {
	r.mu.RLock()
	if r.initialized {
		var sum int64
		for _, item := range r.list {
			sum += item.FileSize
		}
		r.mu.RUnlock()
		return sum, nil
	}
	r.mu.RUnlock()

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
