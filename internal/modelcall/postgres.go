package modelcall

import (
	"context"
	"encoding/json"

	"learning-agent/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) CreateModelCall(ctx context.Context, call model.ModelCall) (model.ModelCall, error) {
	requestMetadata, err := json.Marshal(call.RequestMetadata)
	if err != nil {
		return model.ModelCall{}, err
	}
	responseMetadata, err := json.Marshal(call.ResponseMetadata)
	if err != nil {
		return model.ModelCall{}, err
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO model_calls (
			trace_id, user_id, session_id, provider, model, capability, task,
			stream, status, retry_count, request_metadata, response_metadata, started_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, started_at
	`,
		call.TraceID,
		call.UserID,
		call.SessionID,
		call.Provider,
		call.Model,
		string(call.Capability),
		string(call.Task),
		call.Stream,
		call.Status,
		call.RetryCount,
		requestMetadata,
		responseMetadata,
		call.StartedAt,
	).Scan(&call.ID, &call.StartedAt)
	if err != nil {
		return model.ModelCall{}, err
	}
	return call, nil
}

func (s *PostgresStore) CompleteModelCall(ctx context.Context, id int64, update model.ModelCallUpdate) error {
	responseMetadata, err := json.Marshal(update.ResponseMetadata)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE model_calls
		SET status = $2,
		    prompt_tokens = $3,
		    completion_tokens = $4,
		    total_tokens = $5,
		    latency_ms = $6,
		    retry_count = $7,
		    error_type = NULLIF($8, ''),
		    error_message = NULLIF($9, ''),
		    response_metadata = $10,
		    completed_at = $11
		WHERE id = $1
	`,
		id,
		update.Status,
		nullableInt(update.Usage.PromptTokens),
		nullableInt(update.Usage.CompletionTokens),
		nullableInt(update.Usage.TotalTokens),
		nullableInt64(update.LatencyMS),
		update.RetryCount,
		update.ErrorType,
		update.ErrorMessage,
		responseMetadata,
		update.CompletedAt,
	)
	return err
}

func (s *PostgresStore) ListModelCalls(ctx context.Context, query model.ModelCallQuery) ([]model.ModelCall, error) {
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, trace_id, user_id, session_id, provider, model, capability, task,
		       stream, status, prompt_tokens, completion_tokens, total_tokens,
		       latency_ms, retry_count, error_type, error_message,
		       request_metadata, response_metadata, started_at, completed_at
		FROM model_calls
		WHERE ($1 = '' OR trace_id = $1)
		  AND ($2 = '' OR user_id = $2)
		  AND ($3 = '' OR session_id = $3)
		ORDER BY started_at DESC, id DESC
		LIMIT $4
	`, query.TraceID, query.UserID, query.SessionID, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	calls := []model.ModelCall{}
	for rows.Next() {
		call, err := scanModelCall(rows)
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return calls, nil
}

func (s *PostgresStore) GetModelCall(ctx context.Context, id int64) (model.ModelCall, bool, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, trace_id, user_id, session_id, provider, model, capability, task,
		       stream, status, prompt_tokens, completion_tokens, total_tokens,
		       latency_ms, retry_count, error_type, error_message,
		       request_metadata, response_metadata, started_at, completed_at
		FROM model_calls
		WHERE id = $1
	`, id)

	call, err := scanModelCall(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return model.ModelCall{}, false, nil
		}
		return model.ModelCall{}, false, err
	}
	return call, true, nil
}

type modelCallScanner interface {
	Scan(dest ...any) error
}

func scanModelCall(row modelCallScanner) (model.ModelCall, error) {
	var call model.ModelCall
	var capability string
	var task string
	var requestMetadata []byte
	var responseMetadata []byte
	var promptTokens *int
	var completionTokens *int
	var totalTokens *int
	var latencyMS *int64
	var errorType *string
	var errorMessage *string

	err := row.Scan(
		&call.ID,
		&call.TraceID,
		&call.UserID,
		&call.SessionID,
		&call.Provider,
		&call.Model,
		&capability,
		&task,
		&call.Stream,
		&call.Status,
		&promptTokens,
		&completionTokens,
		&totalTokens,
		&latencyMS,
		&call.RetryCount,
		&errorType,
		&errorMessage,
		&requestMetadata,
		&responseMetadata,
		&call.StartedAt,
		&call.CompletedAt,
	)
	if err != nil {
		return model.ModelCall{}, err
	}

	call.Capability = model.CapabilityKind(capability)
	call.Task = model.Task(task)
	call.PromptTokens = derefInt(promptTokens)
	call.CompletionTokens = derefInt(completionTokens)
	call.TotalTokens = derefInt(totalTokens)
	call.LatencyMS = derefInt64(latencyMS)
	call.ErrorType = derefString(errorType)
	call.ErrorMessage = derefString(errorMessage)
	call.RequestMetadata = map[string]any{}
	call.ResponseMetadata = map[string]any{}
	if len(requestMetadata) > 0 {
		if err := json.Unmarshal(requestMetadata, &call.RequestMetadata); err != nil {
			return model.ModelCall{}, err
		}
	}
	if len(responseMetadata) > 0 {
		if err := json.Unmarshal(responseMetadata, &call.ResponseMetadata); err != nil {
			return model.ModelCall{}, err
		}
	}
	return call, nil
}

func nullableInt(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func nullableInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
