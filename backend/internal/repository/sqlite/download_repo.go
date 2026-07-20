package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
)

var _ repository.DownloadRepository = (*DownloadRepo)(nil)

type DownloadRepo struct {
	db *sql.DB
}

func NewDownloadRepository(db *sql.DB) *DownloadRepo {
	return &DownloadRepo{db: db}
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

func (r *DownloadRepo) FindByID(id string) (*model.Download, error) {
	row := r.db.QueryRow(`SELECT `+dlColumns+` FROM downloads WHERE id = ?`, id)

	d, err := scanDownload(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan download by id %s: %w", id, err)
	}
	return d, nil
}

func (r *DownloadRepo) FindAll() ([]model.Download, error) {
	rows, err := r.db.Query(`SELECT `+dlColumns+` FROM downloads ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query all downloads: %w", err)
	}
	defer rows.Close()

	list, err := scanDownloadRows(rows)
	if err != nil {
		return nil, fmt.Errorf("scan download rows: %w", err)
	}
	return list, nil
}

func (r *DownloadRepo) Create(d *model.Download) error {
	now := time.Now()
	d.CreatedAt = now
	d.UpdatedAt = now

	_, err := r.db.Exec(
		`INSERT INTO downloads (id, title, status, type, progress, total_size, completed_size, download_speed, eta, dest_path, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Title, d.Status, d.Type, d.Progress, d.TotalSize, d.CompletedSize, d.DownloadSpeed, d.ETA, d.DestPath,
		d.CreatedAt.Format(time.RFC3339), d.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert download: %w", err)
	}
	return nil
}

func (r *DownloadRepo) Update(d *model.Download) error {
	d.UpdatedAt = time.Now()

	_, err := r.db.Exec(
		`UPDATE downloads SET title = ?, status = ?, type = ?, progress = ?, total_size = ?, completed_size = ?, 
		download_speed = ?, eta = ?, dest_path = ?, updated_at = ? WHERE id = ?`,
		d.Title, d.Status, d.Type, d.Progress, d.TotalSize, d.CompletedSize, d.DownloadSpeed, d.ETA, d.DestPath,
		d.UpdatedAt.Format(time.RFC3339), d.ID,
	)
	if err != nil {
		return fmt.Errorf("update download %s: %w", d.ID, err)
	}
	return nil
}

func (r *DownloadRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM downloads WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete download %s: %w", id, err)
	}
	return nil
}
