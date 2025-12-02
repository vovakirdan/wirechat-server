-- +goose Up
-- Add direct_key for direct message room deduplication

ALTER TABLE rooms ADD COLUMN direct_key TEXT;

-- Ensure only one direct room exists per user pair
CREATE UNIQUE INDEX idx_rooms_direct_key ON rooms(direct_key) WHERE direct_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_rooms_direct_key;
ALTER TABLE rooms DROP COLUMN direct_key;
