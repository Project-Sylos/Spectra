package auth

import (
	"errors"
	"fmt"
)

// ErrUnauthorized is returned when an access token is missing, invalid, or expired.
type ErrUnauthorized struct {
	World  string
	Reason string
}

func (e *ErrUnauthorized) Error() string {
	if e == nil {
		return "spectra: unauthorized"
	}
	if e.World != "" && e.Reason != "" {
		return fmt.Sprintf("spectra: unauthorized world=%s: %s", e.World, e.Reason)
	}
	if e.Reason != "" {
		return fmt.Sprintf("spectra: unauthorized: %s", e.Reason)
	}
	return "spectra: unauthorized"
}

// IsUnauthorized reports whether err is an auth failure.
func IsUnauthorized(err error) (*ErrUnauthorized, bool) {
	var u *ErrUnauthorized
	if errors.As(err, &u) && u != nil {
		return u, true
	}
	return nil, false
}
