// Package store holds the per-machine sync state (PRD §5.1).
//
// Backed by a single SQLite file (~/.dropboy/state.db) using modernc.org/sqlite
// — pure Go, no CGO, so the binary stays statically linkable.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Entry struct {
	Path         string
	LocalMtime   time.Time
	LocalSize    int64
	LocalHash    string // hex SHA-256 of plaintext
	S3Key        string // full object key inside the bucket
	S3ETag       string
	S3VersionID  string
	NonceB64     string // payload nonce
	DEKNonceB64  string // wrap-DEK nonce
	SealedDEKB64 string // wrapped DEK
	LastSyncedAt time.Time
}

type Store interface {
	Get(ctx context.Context, path string) (Entry, error)
	Put(ctx context.Context, e Entry) error
	List(ctx context.Context) ([]Entry, error)
	Delete(ctx context.Context, path string) error
	Close() error
}

var ErrNotFound = errors.New("state entry not found")

type sqliteStore struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS entries (
    path           TEXT PRIMARY KEY,
    local_mtime    INTEGER NOT NULL,
    local_size     INTEGER NOT NULL,
    local_hash     TEXT NOT NULL,
    s3_key         TEXT NOT NULL,
    s3_etag        TEXT NOT NULL,
    s3_version_id  TEXT NOT NULL,
    nonce_b64      TEXT NOT NULL,
    dek_nonce_b64  TEXT NOT NULL,
    sealed_dek_b64 TEXT NOT NULL,
    last_synced_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_entries_s3_key ON entries(s3_key);
`

// Open returns a Store backed by the given SQLite file. The file is created
// with WAL journaling for durability under crash.
func Open(path string) (Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open state db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) Get(ctx context.Context, path string) (Entry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT path, local_mtime, local_size, local_hash, s3_key, s3_etag,
		       s3_version_id, nonce_b64, dek_nonce_b64, sealed_dek_b64, last_synced_at
		FROM entries WHERE path = ?`, path)
	var e Entry
	var mtime, synced int64
	err := row.Scan(&e.Path, &mtime, &e.LocalSize, &e.LocalHash, &e.S3Key,
		&e.S3ETag, &e.S3VersionID, &e.NonceB64, &e.DEKNonceB64, &e.SealedDEKB64, &synced)
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, ErrNotFound
	}
	if err != nil {
		return Entry{}, err
	}
	e.LocalMtime = time.Unix(0, mtime).UTC()
	e.LastSyncedAt = time.Unix(0, synced).UTC()
	return e, nil
}

func (s *sqliteStore) Put(ctx context.Context, e Entry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO entries(path, local_mtime, local_size, local_hash, s3_key, s3_etag,
		                    s3_version_id, nonce_b64, dek_nonce_b64, sealed_dek_b64, last_synced_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET
		    local_mtime=excluded.local_mtime,
		    local_size=excluded.local_size,
		    local_hash=excluded.local_hash,
		    s3_key=excluded.s3_key,
		    s3_etag=excluded.s3_etag,
		    s3_version_id=excluded.s3_version_id,
		    nonce_b64=excluded.nonce_b64,
		    dek_nonce_b64=excluded.dek_nonce_b64,
		    sealed_dek_b64=excluded.sealed_dek_b64,
		    last_synced_at=excluded.last_synced_at`,
		e.Path, e.LocalMtime.UnixNano(), e.LocalSize, e.LocalHash, e.S3Key,
		e.S3ETag, e.S3VersionID, e.NonceB64, e.DEKNonceB64, e.SealedDEKB64,
		e.LastSyncedAt.UnixNano())
	return err
}

func (s *sqliteStore) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, local_mtime, local_size, local_hash, s3_key, s3_etag,
		       s3_version_id, nonce_b64, dek_nonce_b64, sealed_dek_b64, last_synced_at
		FROM entries`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var mtime, synced int64
		if err := rows.Scan(&e.Path, &mtime, &e.LocalSize, &e.LocalHash, &e.S3Key,
			&e.S3ETag, &e.S3VersionID, &e.NonceB64, &e.DEKNonceB64, &e.SealedDEKB64, &synced); err != nil {
			return nil, err
		}
		e.LocalMtime = time.Unix(0, mtime).UTC()
		e.LastSyncedAt = time.Unix(0, synced).UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *sqliteStore) Delete(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM entries WHERE path = ?`, path)
	return err
}

func (s *sqliteStore) Close() error { return s.db.Close() }
