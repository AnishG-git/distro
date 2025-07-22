-- Shards table: stores information about Reed-Solomon encoded shards
CREATE TABLE IF NOT EXISTS shards (
    id TEXT PRIMARY KEY,
    object_id TEXT NOT NULL REFERENCES objects(id) ON DELETE CASCADE,
    worker_id TEXT NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    shard_index INTEGER NOT NULL,
    size BIGINT NOT NULL,
    checksum TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add composite index for efficient shard lookups by object
CREATE INDEX IF NOT EXISTS idx_shards_object_id ON shards(object_id);

-- Add composite index for efficient shard lookups by worker
CREATE INDEX IF NOT EXISTS idx_shards_worker_id ON shards(worker_id);

-- Add unique constraint to prevent duplicate shards for same object index
CREATE UNIQUE INDEX IF NOT EXISTS idx_shards_object_shard_unique 
    ON shards(object_id, shard_index);

-- Add index for efficient queries by shard index
CREATE INDEX IF NOT EXISTS idx_shards_shard_index ON shards(shard_index);
