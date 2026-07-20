package sqlite

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
)

var _ repository.DownloadRepository = (*DownloadRepo)(nil)

type DownloadRepo struct {
	db          *sql.DB
	mu          sync.RWMutex
	items       map[string]*model.Download
	list        []model.Download
	initialized bool
}

func NewDownloadRepository(db *sql.DB) *DownloadRepo {
	return &DownloadRepo{
		db:    db,
		items: make(map[string]*model.Download),
	}
}

const dlColumns = `id, title, status, type, progress, total_size, completed_size, download_speed, eta, dest_path, created_at, updated_at`

func scanDownload(scanner interface{ Scan(dest ...any) error }) (*model.Download, error) {
	var d model.Download
	var createdAtStr, updatedAtStr string

	err := scanner.Scan(
		&d.ID, &d.Title, &d.Status, &d.Type, &d.Progress, &d.TotalSize, &d.CompletedSize,
		&d.DownloadSpeed, &d.ETA, &d.DestPath, &createdAtStr, &updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	d.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	d.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)

	return &d, nil
}

func scanDownloadRows(rows *sql.Rows) ([]model.Download, error) {
	var list []model.Download
	for rows.Next() {
		d, err := scanDownload(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *d)
	}
	return list, nil
}

func (r *DownloadRepo) ensureCache() error {
	if r.initialized {
		return nil
	}
	rows, err := r.db.Query(`SELECT ` + dlColumns + ` FROM downloads ORDER BY created_at DESC`)
	if err != nil {
		return fmt.Errorf("query all downloads for cache: %w", err)
	}
	defer rows.Close()

	list, err := scanDownloadRows(rows)
	if err != nil {
		return fmt.Errorf("scan download rows for cache: %w", err)
	}

	r.list = list
	r.items = make(map[string]*model.Download, len(list))
	for i := range list {
		item := list[i]
		r.items[item.ID] = &item
	}
	r.initialized = true
	return nil
}

func (r *DownloadRepo) FindByID(id string) (*model.Download, error) {
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

func (r *DownloadRepo) FindAll() ([]model.Download, error) {
	r.mu.RLock()
	if r.initialized {
		cp := make([]model.Download, len(r.list))
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
	cp := make([]model.Download, len(r.list))
	copy(cp, r.list)
	return cp, nil
}

func (r *DownloadRepo) Create(d *model.Download) error {
	now := time.Now()
	d.CreatedAt = now
	d.UpdatedAt = now

	// 1. Write to database first
	_, err := r.db.Exec(
		`INSERT INTO downloads (id, title, status, type, progress, total_size, completed_size, download_speed, eta, dest_path, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Title, d.Status, d.Type, d.Progress, d.TotalSize, d.CompletedSize, d.DownloadSpeed, d.ETA, d.DestPath,
		d.CreatedAt.Format(time.RFC3339), d.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert download: %w", err)
	}

	// 2. Write-Through: Update in-memory cache immediately
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.ensureCache()

	cp := *d
	r.items[d.ID] = &cp

	// Check if already in list
	exists := false
	for i := range r.list {
		if r.list[i].ID == d.ID {
			r.list[i] = cp
			exists = true
			break
		}
	}
	if !exists {
		r.list = append([]model.Download{cp}, r.list...)
	}

	return nil
}

func (r *DownloadRepo) Update(d *model.Download) error {
	d.UpdatedAt = time.Now()

	// 1. Write to database first
	_, err := r.db.Exec(
		`UPDATE downloads SET title = ?, status = ?, type = ?, progress = ?, total_size = ?, completed_size = ?, 
		download_speed = ?, eta = ?, dest_path = ?, updated_at = ? WHERE id = ?`,
		d.Title, d.Status, d.Type, d.Progress, d.TotalSize, d.CompletedSize, d.DownloadSpeed, d.ETA, d.DestPath,
		d.UpdatedAt.Format(time.RFC3339), d.ID,
	)
	if err != nil {
		return fmt.Errorf("update download %s: %w", d.ID, err)
	}

	// 2. Write-Through: Update in-memory cache immediately
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.ensureCache()

	cp := *d
	r.items[d.ID] = &cp
	for i := range r.list {
		if r.list[i].ID == d.ID {
			r.list[i] = cp
			break
		}
	}
	return nil
}

func (r *DownloadRepo) Delete(id string) error {
	// 1. Write to database first
	_, err := r.db.Exec(`DELETE FROM downloads WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete download %s: %w", id, err)
	}

	// 2. Write-Through: Evict from in-memory cache immediately
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.initialized {
		delete(r.items, id)
		newList := make([]model.Download, 0, len(r.list))
		for _, item := range r.list {
			if item.ID != id {
				newList = append(newList, item)
			}
		}
		r.list = newList
	}
	return nil
}
