-- Update the status column to include 'created' status
ALTER TABLE trips DROP CONSTRAINT IF EXISTS trips_status_check;
ALTER TABLE trips ADD CONSTRAINT trips_status_check CHECK (status IN ('created', 'active', 'ended'));

-- Update default value to 'created'
ALTER TABLE trips ALTER COLUMN status SET DEFAULT 'created';