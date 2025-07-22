-- Objects table: stores metadata about distributed files
CREATE TABLE IF NOT EXISTS objects (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    size BIGINT NOT NULL,
    checksum TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add index on filename for efficient lookups
CREATE INDEX IF NOT EXISTS idx_objects_filename ON objects(filename);

-- Add index on created_at for time-based queries
CREATE INDEX IF NOT EXISTS idx_objects_created_at ON objects(created_at);
