DROP TRIGGER IF EXISTS update_trips_updated_at ON trips;
DROP INDEX IF EXISTS idx_trips_status;
DROP INDEX IF EXISTS idx_trips_creator_id;
DROP TABLE IF EXISTS trips;