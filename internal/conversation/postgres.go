package conversation

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) CreateMessage(ctx context.Context, message Message) (Message, error) {
	message = normalizeMessage(message)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Message{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id)
		VALUES ($1)
		ON CONFLICT (id) DO NOTHING
	`, message.UserID); err != nil {
		return Message{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET updated_at = NOW()
	`, message.SessionID, message.UserID); err != nil {
		return Message{}, err
	}

	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO messages (user_id, session_id, role, content, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, message.UserID, message.SessionID, message.Role, message.Content, message.Status, message.CreatedAt, message.UpdatedAt).Scan(&id); err != nil {
		return Message{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Message{}, err
	}

	message.ID = strconv.FormatInt(id, 10)
	return message, nil
}

func (s *PostgresStore) CompleteMessage(ctx context.Context, messageID string, content string) error {
	id, err := strconv.ParseInt(messageID, 10, 64)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE messages
		SET content = $1,
		    status = 'completed',
		    updated_at = $2
		WHERE id = $3
	`, content, time.Now(), id)
	return err
}
