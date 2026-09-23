package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store is a Postgres-backed store scoped to one account (Neon).
type Store struct {
	SQL       *sql.DB
	AccountID string
}

// DB is a shared Postgres connection for multi-tenant cloud sync.
type DB struct {
	SQL *sql.DB
}

// Open connects to Neon/Postgres and applies migrations.
func Open(ctx context.Context, dsn string) (*DB, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("database URL is empty")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{SQL: db}, nil
}

// Close closes the database.
func (db *DB) Close() error {
	if db == nil || db.SQL == nil {
		return nil
	}
	return db.SQL.Close()
}

// Store returns a tenant-scoped store.
func (db *DB) Store(accountID string) *Store {
	if strings.TrimSpace(accountID) == "" {
		accountID = "default"
	}
	return &Store{SQL: db.SQL, AccountID: accountID}
}

// ListAccountIDs returns all registered cloud accounts.
func (db *DB) ListAccountIDs(ctx context.Context) ([]string, error) {
	rows, err := db.SQL.QueryContext(ctx, `SELECT id FROM accounts ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// OpenFromEnv connects using DATABASE_URL and returns a shared DB handle.
func OpenFromEnv(ctx context.Context) (*DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	return Open(ctx, dsn)
}

const DailySyncLockKey int64 = 0x4A53594E // "JSYN"

func (db *DB) TryAdvisoryLock(ctx context.Context, key int64) (unlock func(), ok bool, err error) {
	if db == nil || db.SQL == nil {
		return nil, false, fmt.Errorf("database is nil")
	}
	conn, err := db.SQL.Conn(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("lock conn: %w", err)
	}
	var got bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&got); err != nil {
		_ = conn.Close()
		return nil, false, fmt.Errorf("advisory lock: %w", err)
	}
	if !got {
		_ = conn.Close()
		return func() {}, false, nil
	}
	unlock = func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, key)
		_ = conn.Close()
	}
	return unlock, true, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	body, err := migrationFS.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(body)); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	return nil
}
