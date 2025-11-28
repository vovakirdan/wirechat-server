package core

import "errors"

// Error codes for domain errors.
const (
	ErrCodeRoomNotFound  = "room_not_found"
	ErrCodeAlreadyJoined = "already_joined"
	ErrCodeNotInRoom     = "not_in_room"
	ErrCodeBadRequest    = "bad_request"
)

var (
	ErrRoomNotFound  = errors.New("room not found")
	ErrAlreadyJoined = errors.New("already joined")
	ErrNotInRoom     = errors.New("not in room")
	ErrBadRequest    = errors.New("bad request")
)

// CoreError wraps a code and human-readable message.
type CoreError struct {
	Code    string
	Message string
}

func (e *CoreError) Error() string {
	return e.Message
}

func coreError(code, msg string) *CoreError {
	return &CoreError{Code: code, Message: msg}
}
