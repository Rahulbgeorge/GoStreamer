-- Add genre column to existing media tables (safe to re-run)
-- SQLite ALTER TABLE ADD COLUMN is idempotent-safe with error handling in Go code.
ALTER TABLE media ADD COLUMN genre TEXT NOT NULL DEFAULT '';
