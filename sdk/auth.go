package sdk

import (
	"time"

	"codeberg.org/Sylos/Spectra/internal/auth"
)

// TokenPair is the credential set returned by IssueTokens / RefreshAccessToken.
type TokenPair = auth.TokenPair

// UnauthorizedError is returned when auth is enabled and credentials are missing/invalid/expired.
type UnauthorizedError = auth.ErrUnauthorized

// IsUnauthorized reports whether err is an auth failure.
func IsUnauthorized(err error) (*UnauthorizedError, bool) {
	return auth.IsUnauthorized(err)
}

// IssueTokens mints access + refresh tokens for a world and binds the access token.
func (s *SpectraFS) IssueTokens(world string) (*TokenPair, error) {
	if s == nil || s.auth == nil {
		return nil, authDisabledErr()
	}
	return s.auth.IssueTokens(world)
}

// RefreshAccessToken exchanges a refresh token for a new access token.
func (s *SpectraFS) RefreshAccessToken(world, refreshToken string) (*TokenPair, error) {
	if s == nil || s.auth == nil {
		return nil, authDisabledErr()
	}
	return s.auth.Refresh(world, refreshToken)
}

// SetAccessToken binds the access token used for subsequent ops on world.
func (s *SpectraFS) SetAccessToken(world, accessToken string) {
	if s == nil || s.auth == nil {
		return
	}
	s.auth.SetAccessToken(world, accessToken)
}

// ClearAccessToken clears the bound access token for world.
func (s *SpectraFS) ClearAccessToken(world string) {
	if s == nil || s.auth == nil {
		return
	}
	s.auth.ClearAccessToken(world)
}

// EnsureWorldAuth issues or restores tokens for world when auth is enabled.
func (s *SpectraFS) EnsureWorldAuth(world string) (*TokenPair, error) {
	if s == nil || s.auth == nil || !s.auth.Enabled() {
		return nil, nil
	}
	return s.auth.EnsureWorldTokens(world)
}

// AuthEnabled reports whether auth enforcement is active.
func (s *SpectraFS) AuthEnabled() bool {
	return s != nil && s.auth != nil && s.auth.Enabled()
}

// AuthEngine returns the auth engine for HTTP middleware wiring.
func (s *SpectraFS) AuthEngine() *auth.Engine {
	if s == nil {
		return nil
	}
	return s.auth
}

func authDisabledErr() error {
	return &UnauthorizedError{Reason: "auth is not enabled"}
}

// FormatExpiresAt formats token expiry for API responses (empty if never).
func FormatExpiresAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
