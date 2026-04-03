package domain

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrChatNotFound    = errors.New("chat not found")
	ErrUserBanned      = errors.New("user is banned")
	ErrUserMuted       = errors.New("user is muted")
	ErrInvalidFormat   = errors.New("invalid export format")
	ErrEmptyMessage    = errors.New("message text is empty")
	ErrMessageTooLong  = errors.New("message text exceeds 4096 characters")
)
