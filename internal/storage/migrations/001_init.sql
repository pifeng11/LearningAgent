CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    session_id TEXT NOT NULL REFERENCES sessions(id),
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'completed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS memories (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    session_id TEXT,
    -- memories 保存从原始 messages 提炼出的可复用记忆，不再承担完整对话日志职责。
    type TEXT NOT NULL DEFAULT 'summary',
    title TEXT NOT NULL DEFAULT 'Conversation turn',
    content TEXT NOT NULL,
    -- scope 表示记忆注入范围；来源 session 和长期 user 级记忆可以分离。
    scope TEXT NOT NULL,
    -- status 用于软删除、过期和被新记忆取代，避免直接丢失可追溯信息。
    status TEXT NOT NULL DEFAULT 'active',
    confidence NUMERIC NOT NULL DEFAULT 1.0,
    -- 记录来源消息，便于用户纠错、审计和未来重新抽取记忆。
    source_message_ids BIGINT[] NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    valid_from TIMESTAMPTZ,
    valid_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS learning_profiles (
    user_id TEXT PRIMARY KEY REFERENCES users(id),
    goals JSONB NOT NULL DEFAULT '[]'::JSONB,
    preferences JSONB NOT NULL DEFAULT '{}'::JSONB,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS learning_progress (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    topic TEXT NOT NULL,
    status TEXT NOT NULL,
    score NUMERIC,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS skill_runs (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    session_id TEXT,
    skill_name TEXT NOT NULL,
    status TEXT NOT NULL,
    input JSONB NOT NULL DEFAULT '{}'::JSONB,
    output JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
