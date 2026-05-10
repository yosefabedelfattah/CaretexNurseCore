package errors

import "errors"

var (
	ErrNotFound      = errors.New("resource not found")
	ErrConflict      = errors.New("resource conflict")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInternal      = errors.New("internal error")
	ErrTokenInvalid  = errors.New("invalid token")
	ErrTokenExpired  = errors.New("token expired")
	ErrTokenRevoked  = errors.New("token revoked")
)

// Is wraps errors.Is for convenience.
func Is(err, target error) bool {
	return errors.Is(err, target)
}
