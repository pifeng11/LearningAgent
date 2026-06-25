CREATE TABLE IF NOT EXISTS agent_traces (
    id BIGSERIAL PRIMARY KEY,
    trace_id TEXT NOT NULL UNIQUE,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    intent TEXT NOT NULL,
    model_task TEXT NOT NULL,
    prompt_chars INTEGER NOT NULL DEFAULT 0,
    estimated_prompt_tokens INTEGER NOT NULL DEFAULT 0,
    prompt_builder_version TEXT NOT NULL DEFAULT 'v1',
    system_prompt_hash TEXT NOT NULL DEFAULT '',
    prompt_config JSONB NOT NULL DEFAULT '{}',
    prompt_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_trace_context_items (
    id BIGSERIAL PRIMARY KEY,
    trace_id TEXT NOT NULL REFERENCES agent_traces(trace_id) ON DELETE CASCADE,
    item_type TEXT NOT NULL,
    source_id TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_traces_user_session_created
ON agent_traces (user_id, session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_trace_context_items_trace
ON agent_trace_context_items (trace_id, ordinal);

CREATE INDEX IF NOT EXISTS idx_agent_trace_context_items_source
ON agent_trace_context_items (item_type, source_id);
