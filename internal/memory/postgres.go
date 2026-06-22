package memory

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Load(ctx context.Context, userID string, sessionID string) ([]Entry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT user_id, COALESCE(session_id, ''), scope, content, created_at
		FROM memories
		WHERE user_id = $1
		  AND (session_id = $2 OR scope = $3)
		ORDER BY created_at ASC
	`, userID, sessionID, ScopeLongTerm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.UserID, &entry.SessionID, &entry.Scope, &entry.Content, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func (s *PostgresStore) Save(ctx context.Context, entry Entry) error {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id)
		VALUES ($1)
		ON CONFLICT (id) DO NOTHING
	`, entry.UserID); err != nil {
		return err
	}

	if entry.SessionID != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO sessions (id, user_id)
			VALUES ($1, $2)
			ON CONFLICT (id) DO UPDATE SET updated_at = NOW()
		`, entry.SessionID, entry.UserID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO memories (user_id, session_id, scope, content, created_at)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5)
	`, entry.UserID, entry.SessionID, entry.Scope, entry.Content, entry.CreatedAt); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
