WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY user_id, type, title, scope
               ORDER BY updated_at DESC, id DESC
           ) AS rn
    FROM memories
    WHERE status = 'active'
)
UPDATE memories
SET status = 'superseded',
    updated_at = NOW()
WHERE id IN (
    SELECT id FROM ranked WHERE rn > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_memories_active_identity
ON memories (user_id, type, title, scope)
WHERE status = 'active';
