package types

// AuthConfig configures optional per-world access/refresh token simulation.
type AuthConfig struct {
	Enabled bool `json:"enabled"`
	// AccessTokenTTLSeconds is the access token lifetime in seconds.
	// -1 means never expire; when enabled must be -1 or >= 10 (MinAccessTokenTTLSeconds in auth package).
	AccessTokenTTLSeconds int64 `json:"access_token_ttl_seconds"`
}
