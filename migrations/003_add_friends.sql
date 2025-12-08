-- +goose Up
-- Friends table for user relationships

CREATE TABLE friends (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL,
    friend_id  INTEGER NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',  -- pending, accepted, blocked
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, friend_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (friend_id) REFERENCES users(id)
);

CREATE INDEX idx_friends_user ON friends(user_id);
CREATE INDEX idx_friends_friend ON friends(friend_id);
CREATE INDEX idx_friends_status ON friends(status);

-- +goose Down
DROP TABLE IF EXISTS friends;
