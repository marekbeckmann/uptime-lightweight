package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB for uptime-mon
type DB struct {
	*sql.DB
}

// Open initializes the SQLite database at dbPath with WAL mode and busy_timeout=5000
func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory %s: %w", dir, err)
	}

	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to sqlite database: %w", err)
	}

	wrapped := &DB{DB: db}
	if err := wrapped.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	return wrapped, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS endpoint_state (
		endpoint_id TEXT PRIMARY KEY,
		status TEXT NOT NULL CHECK (status IN ('UP', 'DOWN', 'MAINTENANCE', 'PENDING')),
		consecutive_failures INTEGER NOT NULL DEFAULT 0,
		last_check_at DATETIME,
		last_up_at DATETIME,
		down_since DATETIME,
		last_latency_ms INTEGER DEFAULT 0,
		last_error TEXT DEFAULT '',
		tls_expiry_at DATETIME,
		last_tls_warn_threshold INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS probe_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		endpoint_id TEXT NOT NULL,
		checked_at DATETIME NOT NULL,
		status TEXT NOT NULL,
		latency_ms INTEGER NOT NULL,
		error TEXT DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_probe_history_endpoint_time 
	ON probe_history(endpoint_id, checked_at DESC);

	CREATE TABLE IF NOT EXISTS api_tokens (
		token_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		secret_hash TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		last_used_at DATETIME,
		revoked INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS maintenance_notifications (
		maintenance_key TEXT PRIMARY KEY,
		config_hash TEXT NOT NULL,
		created_sent_at DATETIME,
		last_reminder_window_start DATETIME
	);
	`

	_, err := db.Exec(schema)
	return err
}
