-- Migration 007: Add media_categories junction table for assigning whole videos to categories

CREATE TABLE IF NOT EXISTS media_categories (
    media_id TEXT NOT NULL,
    category_id TEXT NOT NULL,
    PRIMARY KEY (media_id, category_id),
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_media_categories_media_id ON media_categories(media_id);
CREATE INDEX IF NOT EXISTS idx_media_categories_category_id ON media_categories(category_id);
