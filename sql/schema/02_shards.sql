-- Shards table: stores information about Reed-Solomon encoded shards
CREATE TABLE IF NOT EXISTS shards (
    shard_id uuid PRIMARY KEY,
    object_id uuid NOT NULL REFERENCES objects(object_id) ON DELETE CASCADE,
    worker_id uuid NOT NULL REFERENCES workers(worker_id) ON DELETE CASCADE,
    shard_size BIGINT NOT NULL,
    status TEXT DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add composite index for efficient shard lookups by object
CREATE INDEX IF NOT EXISTS idx_shards_object_id ON shards(object_id);

-- Add composite index for efficient shard lookups by worker
CREATE INDEX IF NOT EXISTS idx_shards_worker_id ON shards(worker_id);