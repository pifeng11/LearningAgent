ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS type TEXT NOT NULL DEFAULT 'summary';

ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT 'Conversation turn';

ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';

ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS confidence NUMERIC NOT NULL DEFAULT 1.0;

-- 来源 message id 用于审计、纠错和未来重建记忆。
ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS source_message_ids BIGINT[] NOT NULL DEFAULT '{}';

ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS valid_from TIMESTAMPTZ;

ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS valid_until TIMESTAMPTZ;

ALTER TABLE memories
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE memories
SET scope = 'session'
WHERE scope = 'short_term';

UPDATE memories
SET scope = 'user'
WHERE scope = 'long_term';

CREATE INDEX IF NOT EXISTS idx_memories_user_status
ON memories (user_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_memories_user_type_status
ON memories (user_id, type, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_memories_session
ON memories (session_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_memories_metadata_gin
ON memories USING GIN (metadata);
