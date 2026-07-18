package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"codeberg.org/Sylos/Spectra/internal/types"
)

// MinAccessTokenTTLSeconds is the shortest allowed access-token lifetime when auth is enabled
// (aside from -1 = never expire). Keeps room for RefreshDebounce under concurrent callers.
const MinAccessTokenTTLSeconds int64 = 10

// RefreshDebounce is how long after a successful Issue/Refresh that further Refresh
// calls for the same world reuse the current access token instead of rotating again.
// Prevents concurrent FS workers from stampeding token renewal on the same expiry.
const RefreshDebounce = 5 * time.Second

// TokenPair is the issued credential set for a world.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time // zero means never expires
	World        string
}

type worldTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time // zero means never expires
}

// Engine manages per-world access/refresh tokens.
type Engine struct {
	cfg         types.AuthConfig
	mu          sync.RWMutex
	issued      map[string]worldTokens // world -> issued tokens
	presented   map[string]string      // world -> access token presented by client
	lastRefresh map[string]time.Time   // world -> last successful Issue/Refresh
	now         func() time.Time
	persist     *Store
}

// NewEngine builds an auth engine. Nil or disabled config returns a no-op engine.
func NewEngine(raw *types.AuthConfig, persistPath string) *Engine {
	e := &Engine{
		issued:      make(map[string]worldTokens),
		presented:   make(map[string]string),
		lastRefresh: make(map[string]time.Time),
		now:         time.Now,
	}
	if raw == nil || !raw.Enabled {
		return e
	}
	e.cfg = *raw
	if persistPath != "" {
		e.persist = NewStore(persistPath)
		_ = e.loadFromDisk()
	}
	return e
}

// Enabled reports whether auth enforcement is active.
func (e *Engine) Enabled() bool {
	return e != nil && e.cfg.Enabled
}

// SetNow replaces the clock (for tests).
func (e *Engine) SetNow(fn func() time.Time) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if fn == nil {
		e.now = time.Now
		return
	}
	e.now = fn
}

// IssueTokens mints access + refresh tokens for a world and persists them.
func (e *Engine) IssueTokens(world string) (*TokenPair, error) {
	if e == nil || !e.cfg.Enabled {
		return nil, fmt.Errorf("auth is not enabled")
	}
	if world == "" {
		world = "primary"
	}

	access, err := randomToken()
	if err != nil {
		return nil, err
	}
	refresh, err := randomToken()
	if err != nil {
		return nil, err
	}

	expires := e.computeExpiry()
	pair := &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    expires,
		World:        world,
	}

	e.mu.Lock()
	e.issued[world] = worldTokens{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    expires,
	}
	e.presented[world] = access
	e.lastRefresh[world] = e.now()
	e.mu.Unlock()

	if err := e.saveToDisk(); err != nil {
		return pair, err
	}
	return pair, nil
}

// Refresh validates the refresh token and issues a new access token.
// Concurrent Refresh calls within RefreshDebounce reuse the current access token
// (same refresh token) so FS middleware stampede does not rotate tokens repeatedly.
func (e *Engine) Refresh(world, refreshToken string) (*TokenPair, error) {
	if e == nil || !e.cfg.Enabled {
		return nil, fmt.Errorf("auth is not enabled")
	}
	if world == "" {
		world = "primary"
	}
	if refreshToken == "" {
		return nil, &ErrUnauthorized{World: world, Reason: "missing refresh token"}
	}

	e.mu.Lock()
	cur, ok := e.issued[world]
	if !ok || cur.RefreshToken != refreshToken {
		e.mu.Unlock()
		return nil, &ErrUnauthorized{World: world, Reason: "invalid refresh token"}
	}

	now := e.now()
	if last, ok := e.lastRefresh[world]; ok && now.Sub(last) < RefreshDebounce {
		pair := &TokenPair{
			AccessToken:  cur.AccessToken,
			RefreshToken: cur.RefreshToken,
			ExpiresAt:    cur.ExpiresAt,
			World:        world,
		}
		e.presented[world] = cur.AccessToken
		e.mu.Unlock()
		return pair, nil
	}

	access, err := randomToken()
	if err != nil {
		e.mu.Unlock()
		return nil, err
	}
	expires := e.computeExpiryLocked()
	cur.AccessToken = access
	cur.ExpiresAt = expires
	e.issued[world] = cur
	e.presented[world] = access
	e.lastRefresh[world] = now
	pair := &TokenPair{
		AccessToken:  access,
		RefreshToken: cur.RefreshToken,
		ExpiresAt:    expires,
		World:        world,
	}
	e.mu.Unlock()

	if err := e.saveToDisk(); err != nil {
		return pair, err
	}
	return pair, nil
}

// SetAccessToken binds the access token the caller will present for a world.
func (e *Engine) SetAccessToken(world, accessToken string) {
	if e == nil {
		return
	}
	if world == "" {
		world = "primary"
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if accessToken == "" {
		delete(e.presented, world)
		return
	}
	e.presented[world] = accessToken
}

// ClearAccessToken clears the presented token for a world.
func (e *Engine) ClearAccessToken(world string) {
	e.SetAccessToken(world, "")
}

// HasPresentedToken reports whether a world has a bound access token.
func (e *Engine) HasPresentedToken(world string) bool {
	if e == nil || !e.cfg.Enabled {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.presented[world]
	return ok
}

// ValidateWorld checks the presented access token for world.
func (e *Engine) ValidateWorld(world string) error {
	if e == nil || !e.cfg.Enabled {
		return nil
	}
	if world == "" {
		world = "primary"
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	presented, ok := e.presented[world]
	if !ok || presented == "" {
		return &ErrUnauthorized{World: world, Reason: "missing access token"}
	}
	issued, ok := e.issued[world]
	if !ok || issued.AccessToken != presented {
		return &ErrUnauthorized{World: world, Reason: "invalid access token"}
	}
	now := e.now()
	if !issued.ExpiresAt.IsZero() && !now.Before(issued.ExpiresAt) {
		return &ErrUnauthorized{World: world, Reason: "access token expired"}
	}
	return nil
}

// ValidateBearer checks a Bearer access token, optionally for a specific world.
// If world is empty, the token may match any world's issued access token.
func (e *Engine) ValidateBearer(accessToken, world string) (string, error) {
	if e == nil || !e.cfg.Enabled {
		return world, nil
	}
	if accessToken == "" {
		return "", &ErrUnauthorized{Reason: "missing access token"}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	now := e.now()

	if world != "" {
		issued, ok := e.issued[world]
		if !ok || issued.AccessToken != accessToken {
			return "", &ErrUnauthorized{World: world, Reason: "invalid access token"}
		}
		if !issued.ExpiresAt.IsZero() && !now.Before(issued.ExpiresAt) {
			return "", &ErrUnauthorized{World: world, Reason: "access token expired"}
		}
		return world, nil
	}

	for w, issued := range e.issued {
		if issued.AccessToken != accessToken {
			continue
		}
		if !issued.ExpiresAt.IsZero() && !now.Before(issued.ExpiresAt) {
			return "", &ErrUnauthorized{World: w, Reason: "access token expired"}
		}
		return w, nil
	}
	return "", &ErrUnauthorized{Reason: "invalid access token"}
}

// EnsureWorldTokens issues tokens for world if none exist yet.
func (e *Engine) EnsureWorldTokens(world string) (*TokenPair, error) {
	if e == nil || !e.cfg.Enabled {
		return nil, nil
	}
	if world == "" {
		world = "primary"
	}
	e.mu.RLock()
	_, ok := e.issued[world]
	presented := e.presented[world]
	refresh := ""
	if ok {
		refresh = e.issued[world].RefreshToken
	}
	e.mu.RUnlock()
	if ok && presented != "" {
		if err := e.ValidateWorld(world); err == nil {
			e.mu.RLock()
			cur := e.issued[world]
			e.mu.RUnlock()
			return &TokenPair{
				AccessToken:  cur.AccessToken,
				RefreshToken: cur.RefreshToken,
				ExpiresAt:    cur.ExpiresAt,
				World:        world,
			}, nil
		}
		if refresh != "" {
			return e.Refresh(world, refresh)
		}
	}
	return e.IssueTokens(world)
}

func (e *Engine) computeExpiry() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.computeExpiryLocked()
}

func (e *Engine) computeExpiryLocked() time.Time {
	ttl := e.cfg.AccessTokenTTLSeconds
	if ttl < 0 {
		return time.Time{}
	}
	return e.now().Add(time.Duration(ttl) * time.Second)
}

func (e *Engine) snapshotLocked() AuthFile {
	out := AuthFile{Worlds: make(map[string]PersistedWorld)}
	for w, t := range e.issued {
		pw := PersistedWorld{
			AccessToken:  t.AccessToken,
			RefreshToken: t.RefreshToken,
		}
		if !t.ExpiresAt.IsZero() {
			s := t.ExpiresAt.UTC().Format(time.RFC3339)
			pw.ExpiresAt = &s
		}
		out.Worlds[w] = pw
	}
	return out
}

func (e *Engine) loadFromDisk() error {
	if e.persist == nil {
		return nil
	}
	file, err := e.persist.Load()
	if err != nil || file == nil {
		return err
	}
	now := e.now()
	dirty := false
	for world, pw := range file.Worlds {
		var expires time.Time
		if pw.ExpiresAt != nil && *pw.ExpiresAt != "" {
			expires, _ = time.Parse(time.RFC3339, *pw.ExpiresAt)
		}
		e.issued[world] = worldTokens{
			AccessToken:  pw.AccessToken,
			RefreshToken: pw.RefreshToken,
			ExpiresAt:    expires,
		}
		if !expires.IsZero() && !now.Before(expires) && pw.RefreshToken != "" {
			access, err := randomToken()
			if err != nil {
				continue
			}
			newExp := e.computeExpiryLocked()
			e.issued[world] = worldTokens{
				AccessToken:  access,
				RefreshToken: pw.RefreshToken,
				ExpiresAt:    newExp,
			}
			e.presented[world] = access
			e.lastRefresh[world] = now
			dirty = true
			continue
		}
		if pw.AccessToken != "" {
			e.presented[world] = pw.AccessToken
		}
	}
	if dirty {
		return e.saveToDisk()
	}
	return nil
}

func (e *Engine) saveToDisk() error {
	if e == nil || e.persist == nil || !e.cfg.Enabled {
		return nil
	}
	e.mu.RLock()
	snapshot := e.snapshotLocked()
	persist := e.persist
	e.mu.RUnlock()
	return persist.Save(snapshot)
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
