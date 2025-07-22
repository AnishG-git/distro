CREATE OR REPLACE FUNCTION set_shards_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_set_shards_updated_at
BEFORE UPDATE ON shards
FOR EACH ROW
EXECUTE FUNCTION set_shards_updated_at();