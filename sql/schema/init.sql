-- Create all tables in order
-- This script assumes the workers table already exists

-- Load objects table
\i 01_objects.sql

-- Load shards table  
\i 02_shards.sql

-- Update functions for automatic updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add triggers for automatic updated_at timestamps
DROP TRIGGER IF EXISTS trigger_set_updated_at ON workers;
CREATE TRIGGER update_workers_updated_at
    BEFORE UPDATE ON workers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_objects_updated_at ON objects;
CREATE TRIGGER update_objects_updated_at 
    BEFORE UPDATE ON objects 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_shards_updated_at ON shards;
CREATE TRIGGER update_shards_updated_at 
    BEFORE UPDATE ON shards 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();
