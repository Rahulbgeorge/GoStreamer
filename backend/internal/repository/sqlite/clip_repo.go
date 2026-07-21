package sqlite

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
)

var _ repository.ClipRepository = (*ClipRepo)(nil)

type ClipRepo struct {
	db          *sql.DB
	mu          sync.RWMutex
	items       map[string]*model.Clip
	list        []model.Clip
	initialized bool
}

func NewClipRepository(db *sql.DB) *ClipRepo {
	return &ClipRepo{
		db:    db,
		items: make(map[string]*model.Clip),
	}
}

func (r *ClipRepo) ensureCache() error {
	if r.initialized {
		return nil
	}

	rows, err := r.db.Query(`
		SELECT id, media_id, title, start_time, end_time, thumbnail_path, created_at, updated_at 
		FROM clips 
		ORDER BY created_at DESC
	`)
	if err != nil {
		return fmt.Errorf("query all clips for cache: %w", err)
	}
	defer rows.Close()

	var list []model.Clip
	for rows.Next() {
		var c model.Clip
		var createdAtStr, updatedAtStr string
		if err := rows.Scan(&c.ID, &c.MediaID, &c.Title, &c.StartTime, &c.EndTime, &c.ThumbnailPath, &createdAtStr, &updatedAtStr); err != nil {
			return fmt.Errorf("scan clip row: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
		list = append(list, c)
	}

	// Fetch category associations for all clips
	catRows, err := r.db.Query(`
		SELECT cc.clip_id, cat.id, cat.name, cat.created_at
		FROM clip_categories cc
		JOIN categories cat ON cc.category_id = cat.id
	`)
	if err == nil {
		defer catRows.Close()
		clipCategories := make(map[string][]model.Category)
		clipCatIDs := make(map[string][]string)

		for catRows.Next() {
			var clipID string
			var cat model.Category
			var catCreatedAtStr string
			if scanErr := catRows.Scan(&clipID, &cat.ID, &cat.Name, &catCreatedAtStr); scanErr == nil {
				cat.CreatedAt, _ = time.Parse(time.RFC3339, catCreatedAtStr)
				if cat.CreatedAt.IsZero() {
					cat.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", catCreatedAtStr)
				}
				clipCategories[clipID] = append(clipCategories[clipID], cat)
				clipCatIDs[clipID] = append(clipCatIDs[clipID], cat.ID)
			}
		}

		for i := range list {
			cid := list[i].ID
			list[i].CategoryIDs = clipCatIDs[cid]
			list[i].Categories = clipCategories[cid]
		}
	}

	items := make(map[string]*model.Clip, len(list))
	for i := range list {
		item := list[i]
		items[item.ID] = &item
	}

	r.list = list
	r.items = items
	r.initialized = true
	return nil
}

func (r *ClipRepo) FindAll() ([]model.Clip, error) {
	r.mu.RLock()
	if r.initialized {
		cp := make([]model.Clip, len(r.list))
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
	cp := make([]model.Clip, len(r.list))
	copy(cp, r.list)
	return cp, nil
}

func (r *ClipRepo) FindByID(id string) (*model.Clip, error) {
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

func (r *ClipRepo) FindByMediaID(mediaID string) ([]model.Clip, error) {
	r.mu.RLock()
	if r.initialized {
		var res []model.Clip
		for _, item := range r.list {
			if item.MediaID == mediaID {
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
	var res []model.Clip
	for _, item := range r.list {
		if item.MediaID == mediaID {
			res = append(res, item)
		}
	}
	return res, nil
}

func (r *ClipRepo) FindByCategoryID(categoryID string) ([]model.Clip, error) {
	r.mu.RLock()
	if r.initialized {
		var res []model.Clip
		for _, item := range r.list {
			for _, cid := range item.CategoryIDs {
				if cid == categoryID {
					res = append(res, item)
					break
				}
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
	var res []model.Clip
	for _, item := range r.list {
		for _, cid := range item.CategoryIDs {
			if cid == categoryID {
				res = append(res, item)
				break
			}
		}
	}
	return res, nil
}

func (r *ClipRepo) Create(clip *model.Clip) error {
	now := time.Now().UTC()
	clip.CreatedAt = now
	clip.UpdatedAt = now

	// 1. Transaction to write clip and junction records
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction for create clip: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO clips (id, media_id, title, start_time, end_time, thumbnail_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, clip.ID, clip.MediaID, clip.Title, clip.StartTime, clip.EndTime, clip.ThumbnailPath,
		clip.CreatedAt.Format(time.RFC3339), clip.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert clip %s: %w", clip.ID, err)
	}

	for _, catID := range clip.CategoryIDs {
		_, err = tx.Exec(`INSERT INTO clip_categories (clip_id, category_id) VALUES (?, ?)`, clip.ID, catID)
		if err != nil {
			return fmt.Errorf("insert clip category %s -> %s: %w", clip.ID, catID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create clip transaction: %w", err)
	}

	// Fetch categories details to attach to clip model
	if len(clip.CategoryIDs) > 0 {
		var cats []model.Category
		rows, err := r.db.Query(`SELECT id, name, created_at FROM categories WHERE id IN (`+joinPlaceholders(len(clip.CategoryIDs))+`)`, toArgs(clip.CategoryIDs)...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cat model.Category
				var createdAtStr string
				if rows.Scan(&cat.ID, &cat.Name, &createdAtStr) == nil {
					cat.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
					cats = append(cats, cat)
				}
			}
			clip.Categories = cats
		}
	}

	// 2. Write-Through: Update RAM cache immediately
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.ensureCache()

	cp := *clip
	r.items[clip.ID] = &cp

	exists := false
	for i := range r.list {
		if r.list[i].ID == clip.ID {
			r.list[i] = cp
			exists = true
			break
		}
	}
	if !exists {
		r.list = append([]model.Clip{cp}, r.list...)
	}

	return nil
}

func (r *ClipRepo) Update(clip *model.Clip) error {
	clip.UpdatedAt = time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction for update clip: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE clips SET title = ?, start_time = ?, end_time = ?, thumbnail_path = ?, updated_at = ?
		WHERE id = ?
	`, clip.Title, clip.StartTime, clip.EndTime, clip.ThumbnailPath, clip.UpdatedAt.Format(time.RFC3339), clip.ID)
	if err != nil {
		return fmt.Errorf("update clip %s: %w", clip.ID, err)
	}

	_, err = tx.Exec(`DELETE FROM clip_categories WHERE clip_id = ?`, clip.ID)
	if err != nil {
		return fmt.Errorf("clear clip categories for %s: %w", clip.ID, err)
	}

	for _, catID := range clip.CategoryIDs {
		_, err = tx.Exec(`INSERT INTO clip_categories (clip_id, category_id) VALUES (?, ?)`, clip.ID, catID)
		if err != nil {
			return fmt.Errorf("update clip category %s -> %s: %w", clip.ID, catID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update clip transaction: %w", err)
	}

	// Update categories slice
	if len(clip.CategoryIDs) > 0 {
		var cats []model.Category
		rows, err := r.db.Query(`SELECT id, name, created_at FROM categories WHERE id IN (`+joinPlaceholders(len(clip.CategoryIDs))+`)`, toArgs(clip.CategoryIDs)...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cat model.Category
				var createdAtStr string
				if rows.Scan(&cat.ID, &cat.Name, &createdAtStr) == nil {
					cat.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
					cats = append(cats, cat)
				}
			}
			clip.Categories = cats
		}
	}

	// Write-Through: Update RAM cache immediately
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.ensureCache()

	cp := *clip
	r.items[clip.ID] = &cp
	for i := range r.list {
		if r.list[i].ID == clip.ID {
			r.list[i] = cp
			break
		}
	}
	return nil
}

func (r *ClipRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM clips WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete clip %s: %w", id, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.initialized {
		delete(r.items, id)
		newList := make([]model.Clip, 0, len(r.list))
		for _, item := range r.list {
			if item.ID != id {
				newList = append(newList, item)
			}
		}
		r.list = newList
	}
	return nil
}

func joinPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	res := "?"
	for i := 1; i < n; i++ {
		res += ", ?"
	}
	return res
}

func toArgs(strs []string) []any {
	args := make([]any, len(strs))
	for i, s := range strs {
		args[i] = s
	}
	return args
}
