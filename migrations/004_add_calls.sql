-- +goose Up
-- Calls and call participants tables for voice/video calls

CREATE TABLE calls (
    id                TEXT PRIMARY KEY,  -- UUID
    type              TEXT NOT NULL,     -- direct, room
    mode              TEXT NOT NULL DEFAULT 'livekit',
    initiator_user_id INTEGER NOT NULL,
    room_id           INTEGER,           -- nullable for direct calls
    status            TEXT NOT NULL DEFAULT 'ringing',  -- ringing, active, ended, failed
    external_room_id  TEXT,              -- LiveKit room name
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at          DATETIME,
    FOREIGN KEY (initiator_user_id) REFERENCES users(id),
    FOREIGN KEY (room_id) REFERENCES rooms(id)
);

CREATE INDEX idx_calls_status ON calls(status);
CREATE INDEX idx_calls_initiator ON calls(initiator_user_id);

CREATE TABLE call_participants (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    call_id   TEXT NOT NULL,
    user_id   INTEGER NOT NULL,
    joined_at DATETIME,
    left_at   DATETIME,
    reason    TEXT,
    UNIQUE(call_id, user_id),
    FOREIGN KEY (call_id) REFERENCES calls(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_call_participants_call ON call_participants(call_id);
CREATE INDEX idx_call_participants_user ON call_participants(user_id);

-- +goose Down
DROP TABLE IF EXISTS call_participants;
DROP TABLE IF EXISTS calls;
