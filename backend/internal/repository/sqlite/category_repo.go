package sqlite

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
)

var _ repository.CategoryRepository = (*CategoryRepo)(nil)

type CategoryRepo struct {
	db          *sql.DB
	mu          sync.RWMutex
	items       map[string]*model.Category
	list        []model.Category
	initialized bool
}

func NewCategoryRepository(db *sql.DB) *CategoryRepo {
	return &CategoryRepo{
		db:    db,
		items: make(map[string]*model.Category),
	}
}

func (r *CategoryRepo) ensureCache() error {
	if r.initialized {
		return nil
	}

	rows, err := r.db.Query(`SELECT id, name, created_at FROM categories ORDER BY name ASC`)
	if err != nil {
		return fmt.Errorf("query all categories for cache: %w", err)
	}
	defer rows.Close()

	var list []model.Category
	items := make(map[string]*model.Category)

	for rows.Next() {
		var c model.Category
		var createdAtStr string
		if err := rows.Scan(&c.ID, &c.Name, &createdAtStr); err != nil {
			return fmt.Errorf("scan category row: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		if c.CreatedAt.IsZero() {
			c.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
		}
		list = append(list, c)
	}

	for i := range list {
		item := list[i]
		items[item.ID] = &item
	}

	r.list = list
	r.items = items
	r.initialized = true
	return nil
}

func (r *CategoryRepo) FindAll() ([]model.Category, error) {
	r.mu.RLock()
	if r.initialized {
		cp := make([]model.Category, len(r.list))
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
	cp := make([]model.Category, len(r.list))
	copy(cp, r.list)
	return cp, nil
}

func (r *CategoryRepo) FindByID(id string) (*model.Category, error) {
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

func (r *CategoryRepo) Create(cat *model.Category) error {
	now := time.Now().UTC()
	cat.CreatedAt = now

	// 1. Write to DB first
	_, err := r.db.Exec(`INSERT INTO categories (id, name, created_at) VALUES (?, ?, ?)`,
		cat.ID, cat.Name, cat.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert category %s: %w", cat.ID, err)
	}

	// 2. Write-Through: Update RAM cache immediately
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.ensureCache()

	cp := *cat
	r.items[cat.ID] = &cp

	exists := false
	for i := range r.list {
		if r.list[i].ID == cat.ID {
			r.list[i] = cp
			exists = true
			break
		}
	}
	if !exists {
		r.list = append(r.list, cp)
	}

	return nil
}

func (r *CategoryRepo) Delete(id string) error {
	// 1. Write to DB first
	_, err := r.db.Exec(`DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete category %s: %w", id, err)
	}

	// 2. Write-Through: Evict from RAM cache immediately
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.initialized {
		delete(r.items, id)
		newList := make([]model.Category, 0, len(r.list))
		for _, item := range r.list {
			if item.ID != id {
				newList = append(newList, item)
			}
		}
		r.list = newList
	}

	return nil
}
