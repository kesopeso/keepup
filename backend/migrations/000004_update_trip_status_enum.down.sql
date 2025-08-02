-- Revert to original status constraint
ALTER TABLE trips DROP CONSTRAINT IF EXISTS trips_status_check;
ALTER TABLE trips ADD CONSTRAINT trips_status_check CHECK (status IN ('active', 'ended'));

-- Revert default value to 'active'
ALTER TABLE trips ALTER COLUMN status SET DEFAULT 'active';