package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	AuthJSONFileName = ".spectra-auth.json"
	AuthEnvFileName  = ".spectra-auth.env"
)

// PersistedWorld is one world's tokens on disk.
type PersistedWorld struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpiresAt    *string `json:"expires_at,omitempty"` // RFC3339; nil/omit = never
}

// AuthFile is the sidecar JSON schema.
type AuthFile struct {
	Worlds map[string]PersistedWorld `json:"worlds"`
}

// Store persists auth tokens to a sidecar file.
type Store struct {
	path string
}

// NewStore creates a store at the given path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// PersistPathForConfig returns the sidecar path next to a Spectra config file.
func PersistPathForConfig(configPath string) string {
	if configPath == "" {
		return AuthEnvFileName
	}
	dir := filepath.Dir(configPath)
	if dir == "" || dir == "." {
		return filepath.Join(".", AuthJSONFileName)
	}
	return filepath.Join(dir, AuthJSONFileName)
}

// Load reads tokens from disk. Returns nil, nil if the file does not exist.
func (s *Store) Load() (*AuthFile, error) {
	if s == nil || s.path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if strings.HasSuffix(s.path, ".env") {
		return parseEnvAuth(data)
	}
	var file AuthFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse auth file: %w", err)
	}
	if file.Worlds == nil {
		file.Worlds = map[string]PersistedWorld{}
	}
	return &file, nil
}

// Save writes tokens to disk with mode 0600.
func (s *Store) Save(file AuthFile) error {
	if s == nil || s.path == "" {
		return nil
	}
	if file.Worlds == nil {
		file.Worlds = map[string]PersistedWorld{}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil && filepath.Dir(s.path) != "." {
		// ignore for relative "."
	}
	if strings.HasSuffix(s.path, ".env") {
		return writeEnvAuth(s.path, file)
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

func parseEnvAuth(data []byte) (*AuthFile, error) {
	file := &AuthFile{Worlds: map[string]PersistedWorld{}}
	lines := strings.Split(string(data), "\n")
	type partial struct {
		access, refresh, expires string
	}
	tmp := map[string]*partial{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// SPECTRA_AUTH_<WORLD>_ACCESS_TOKEN
		parts := strings.Split(k, "_")
		if len(parts) < 4 || parts[0] != "SPECTRA" || parts[1] != "AUTH" {
			continue
		}
		world := strings.ToLower(parts[2])
		field := strings.Join(parts[3:], "_")
		p := tmp[world]
		if p == nil {
			p = &partial{}
			tmp[world] = p
		}
		switch field {
		case "ACCESS_TOKEN":
			p.access = v
		case "REFRESH_TOKEN":
			p.refresh = v
		case "EXPIRES_AT":
			p.expires = v
		}
	}
	for world, p := range tmp {
		pw := PersistedWorld{AccessToken: p.access, RefreshToken: p.refresh}
		if p.expires != "" {
			e := p.expires
			pw.ExpiresAt = &e
		}
		file.Worlds[world] = pw
	}
	return file, nil
}

func writeEnvAuth(path string, file AuthFile) error {
	var b strings.Builder
	b.WriteString("# Spectra auth tokens — do not commit\n")
	for world, pw := range file.Worlds {
		w := strings.ToUpper(world)
		fmt.Fprintf(&b, "SPECTRA_AUTH_%s_ACCESS_TOKEN=%s\n", w, pw.AccessToken)
		fmt.Fprintf(&b, "SPECTRA_AUTH_%s_REFRESH_TOKEN=%s\n", w, pw.RefreshToken)
		if pw.ExpiresAt != nil {
			fmt.Fprintf(&b, "SPECTRA_AUTH_%s_EXPIRES_AT=%s\n", w, *pw.ExpiresAt)
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0600)
}
