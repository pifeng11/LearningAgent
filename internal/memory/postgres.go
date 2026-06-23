package memory

import (
	"context"
	"encoding/json"

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
		SELECT id, user_id, COALESCE(session_id, ''), type, title, content, scope, status,
		       confidence, source_message_ids, metadata, valid_from, valid_until, created_at, updated_at
		FROM memories
		WHERE user_id = $1
		  AND status = $2
		  AND (session_id = $3 OR scope = $4)
		  AND (valid_from IS NULL OR valid_from <= NOW())
		  AND (valid_until IS NULL OR valid_until > NOW())
		ORDER BY updated_at DESC, created_at DESC
	`, userID, StatusActive, sessionID, ScopeUser)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var entry Entry
		var metadataBytes []byte
		if err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.SessionID,
			&entry.Type,
			&entry.Title,
			&entry.Content,
			&entry.Scope,
			&entry.Status,
			&entry.Confidence,
			&entry.SourceMessageIDs,
			&metadataBytes,
			&entry.ValidFrom,
			&entry.ValidUntil,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &entry.Metadata); err != nil {
				return nil, err
			}
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func (s *PostgresStore) Save(ctx context.Context, entry Entry) error {
	entry = NormalizeEntry(entry)
	metadata, err := json.Marshal(entry.Metadata)
	if err != nil {
		return err
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
		INSERT INTO memories (
			user_id, session_id, type, title, content, scope, status, confidence,
			source_message_ids, metadata, valid_from, valid_until, created_at, updated_at
		)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, entry.UserID, entry.SessionID, entry.Type, entry.Title, entry.Content, entry.Scope, entry.Status,
		entry.Confidence, entry.SourceMessageIDs, metadata, entry.ValidFrom, entry.ValidUntil, entry.CreatedAt, entry.UpdatedAt); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
