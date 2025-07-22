CREATE OR REPLACE FUNCTION set_objects_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_set_objects_updated_at
BEFORE UPDATE ON objects
FOR EACH ROW
EXECUTE FUNCTION set_objects_updated_at();