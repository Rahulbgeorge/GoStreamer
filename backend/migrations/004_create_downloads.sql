CREATE TABLE IF NOT EXISTS downloads (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    type TEXT NOT NULL,
    progress REAL NOT NULL DEFAULT 0.0,
    total_size INTEGER NOT NULL DEFAULT 0,
    completed_size INTEGER NOT NULL DEFAULT 0,
    download_speed REAL NOT NULL DEFAULT 0.0,
    eta TEXT NOT NULL DEFAULT '',
    dest_path TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
