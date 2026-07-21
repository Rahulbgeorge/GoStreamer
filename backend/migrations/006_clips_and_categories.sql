-- Migration 006: Add default_start_time to media, and create categories, clips, clip_categories tables

-- 1. Categories table
CREATE TABLE IF NOT EXISTS categories (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL
);

-- Seed default categories
INSERT OR IGNORE INTO categories (id, name, created_at) VALUES 
('cat_songs', 'Songs', CURRENT_TIMESTAMP),
('cat_highlights', 'Highlights', CURRENT_TIMESTAMP),
('cat_action', 'Action', CURRENT_TIMESTAMP),
('cat_dialogues', 'Dialogues', CURRENT_TIMESTAMP),
('cat_other', 'Other', CURRENT_TIMESTAMP);

-- 2. Clips table
CREATE TABLE IF NOT EXISTS clips (
    id TEXT PRIMARY KEY,
    media_id TEXT NOT NULL,
    title TEXT NOT NULL,
    start_time REAL NOT NULL DEFAULT 0.0,
    end_time REAL NOT NULL DEFAULT 0.0,
    thumbnail_path TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_clips_media_id ON clips(media_id);

-- 3. Clip - Category Junction table (Many-to-Many)
CREATE TABLE IF NOT EXISTS clip_categories (
    clip_id TEXT NOT NULL,
    category_id TEXT NOT NULL,
    PRIMARY KEY (clip_id, category_id),
    FOREIGN KEY (clip_id) REFERENCES clips(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_clip_categories_clip_id ON clip_categories(clip_id);
CREATE INDEX IF NOT EXISTS idx_clip_categories_category_id ON clip_categories(category_id);
