-- +goose Up
-- Add call privacy settings to users

ALTER TABLE users ADD COLUMN allow_calls_from TEXT NOT NULL DEFAULT 'everyone';
-- values: 'everyone', 'friends_only'

-- +goose Down
-- SQLite does not support DROP COLUMN directly
-- For dev environment, recreate database if needed
-- ALTER TABLE users DROP COLUMN allow_calls_from;
