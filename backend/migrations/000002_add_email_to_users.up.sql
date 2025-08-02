-- Add email column to users table
ALTER TABLE users ADD COLUMN email VARCHAR(255) UNIQUE NOT NULL DEFAULT '';

-- Create index on email for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Update constraint to make email the primary unique identifier
-- Note: In a real migration, you'd need to handle existing data carefully
-- For this development setup, we assume table is empty