package debugtrace

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Save(ctx context.Context, trace PromptTrace) error {
	trace = normalizeTrace(trace)

	promptConfig, err := json.Marshal(trace.PromptConfig)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO agent_traces (
			trace_id, user_id, session_id, intent, model_task,
			prompt_chars, estimated_prompt_tokens, prompt_builder_version,
			system_prompt_hash, prompt_config, prompt_text, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, ''), $12)
		ON CONFLICT (trace_id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			session_id = EXCLUDED.session_id,
			intent = EXCLUDED.intent,
			model_task = EXCLUDED.model_task,
			prompt_chars = EXCLUDED.prompt_chars,
			estimated_prompt_tokens = EXCLUDED.estimated_prompt_tokens,
			prompt_builder_version = EXCLUDED.prompt_builder_version,
			system_prompt_hash = EXCLUDED.system_prompt_hash,
			prompt_config = EXCLUDED.prompt_config,
			prompt_text = EXCLUDED.prompt_text
	`, trace.TraceID, trace.UserID, trace.SessionID, trace.Intent, trace.ModelTask,
		trace.PromptChars, trace.EstimatedPromptTokens, trace.PromptBuilderVersion,
		trace.SystemPromptHash, promptConfig, trace.Prompt, trace.CreatedAt)
	if err != nil {
		return err
	}

	if trace.ContextSnapshotEnabled {
		if _, err := tx.Exec(ctx, `DELETE FROM agent_trace_context_items WHERE trace_id = $1`, trace.TraceID); err != nil {
			return err
		}
		for _, item := range trace.ContextItems {
			metadata, err := json.Marshal(item.Metadata)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO agent_trace_context_items (
					trace_id, item_type, source_id, role, title, content, ordinal, metadata, created_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`, trace.TraceID, item.ItemType, item.SourceID, item.Role, item.Title, item.Content, item.Ordinal, metadata, trace.CreatedAt)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) Get(ctx context.Context, traceID string) (PromptTrace, bool, error) {
	var trace PromptTrace
	var promptConfigBytes []byte
	var promptText *string

	err := s.pool.QueryRow(ctx, `
		SELECT trace_id, user_id, session_id, intent, model_task,
		       prompt_chars, estimated_prompt_tokens, prompt_builder_version,
		       system_prompt_hash, prompt_config, prompt_text, created_at
		FROM agent_traces
		WHERE trace_id = $1
	`, traceID).Scan(
		&trace.TraceID,
		&trace.UserID,
		&trace.SessionID,
		&trace.Intent,
		&trace.ModelTask,
		&trace.PromptChars,
		&trace.EstimatedPromptTokens,
		&trace.PromptBuilderVersion,
		&trace.SystemPromptHash,
		&promptConfigBytes,
		&promptText,
		&trace.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return PromptTrace{}, false, nil
		}
		return PromptTrace{}, false, err
	}
	if promptText != nil {
		trace.Prompt = *promptText
	}
	if len(promptConfigBytes) > 0 {
		if err := json.Unmarshal(promptConfigBytes, &trace.PromptConfig); err != nil {
			return PromptTrace{}, false, err
		}
	}

	items, err := s.loadContextItems(ctx, traceID)
	if err != nil {
		return PromptTrace{}, false, err
	}
	trace.ContextItems = items
	trace.ContextSnapshotEnabled = len(items) > 0
	trace.UsedMemoryIDs = memoryIDsFromItems(items)
	trace.UsedHistoryIDs = historyIDsFromItems(items)
	trace.MemoryCount = len(trace.UsedMemoryIDs)
	trace.HistoryMessageCount = len(trace.UsedHistoryIDs)

	return trace, true, nil
}

func (s *PostgresStore) loadContextItems(ctx context.Context, traceID string) ([]ContextItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT item_type, source_id, role, title, content, ordinal, metadata
		FROM agent_trace_context_items
		WHERE trace_id = $1
		ORDER BY ordinal ASC
	`, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ContextItem{}
	for rows.Next() {
		var item ContextItem
		var metadataBytes []byte
		if err := rows.Scan(&item.ItemType, &item.SourceID, &item.Role, &item.Title, &item.Content, &item.Ordinal, &metadataBytes); err != nil {
			return nil, err
		}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &item.Metadata); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func memoryIDsFromItems(items []ContextItem) []int64 {
	ids := []int64{}
	for _, item := range items {
		if item.ItemType != "memory" || item.SourceID == "" {
			continue
		}
		id, err := strconv.ParseInt(item.SourceID, 10, 64)
		if err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func historyIDsFromItems(items []ContextItem) []string {
	ids := []string{}
	for _, item := range items {
		if item.ItemType == "history" && item.SourceID != "" {
			ids = append(ids, item.SourceID)
		}
	}
	return ids
}
