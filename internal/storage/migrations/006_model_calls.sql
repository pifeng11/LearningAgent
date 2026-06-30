CREATE TABLE IF NOT EXISTS model_calls (
    id BIGSERIAL PRIMARY KEY,
    trace_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',

    provider TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    capability TEXT NOT NULL,
    task TEXT NOT NULL,

    stream BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL,

    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,

    latency_ms BIGINT,
    retry_count INTEGER NOT NULL DEFAULT 0,

    error_type TEXT,
    error_message TEXT,

    request_metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    response_metadata JSONB NOT NULL DEFAULT '{}'::JSONB,

    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_model_calls_trace_id
ON model_calls (trace_id);

CREATE INDEX IF NOT EXISTS idx_model_calls_user_session_started
ON model_calls (user_id, session_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_calls_provider_model_started
ON model_calls (provider, model, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_calls_status_started
ON model_calls (status, started_at DESC);
