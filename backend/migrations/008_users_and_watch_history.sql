-- Migration 008: Create users, watch_history, and user_media_preferences tables

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed default single user
INSERT OR IGNORE INTO users (id, username, display_name, created_at, updated_at)
VALUES ('user_default', 'admin', 'Default User', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- Watch history tracking table (User x Media)
CREATE TABLE IF NOT EXISTS watch_history (
    user_id TEXT NOT NULL,
    media_id TEXT NOT NULL,
    last_position INTEGER NOT NULL DEFAULT 0,
    completed BOOLEAN NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, media_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_watch_history_user_id ON watch_history(user_id);
CREATE INDEX IF NOT EXISTS idx_watch_history_media_id ON watch_history(media_id);

-- User media preferences (Favorites, Ratings, Customizations per user)
CREATE TABLE IF NOT EXISTS user_media_preferences (
    user_id TEXT NOT NULL,
    media_id TEXT NOT NULL,
    favorite BOOLEAN NOT NULL DEFAULT 0,
    rating INTEGER DEFAULT 0,
    notes TEXT DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, media_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);
