package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"codeberg.org/Sylos/Spectra/internal/auth"
	"codeberg.org/Sylos/Spectra/internal/types"
)

type authWorldKey struct{}

// AuthWorldFromContext returns the world resolved by auth middleware, if any.
func AuthWorldFromContext(ctx context.Context) string {
	v, _ := ctx.Value(authWorldKey{}).(string)
	return v
}

// Auth enforces Bearer access tokens when the auth engine is enabled.
// Exempt paths should not use this middleware (health + /auth/*).
func Auth(engine *auth.Engine) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if engine == nil || !engine.Enabled() {
				next.ServeHTTP(w, r)
				return
			}

			header := r.Header.Get("Authorization")
			token := ""
			if strings.HasPrefix(strings.ToLower(header), "bearer ") {
				token = strings.TrimSpace(header[7:])
			}

			world := extractWorldFromRequest(r)
			resolved, err := engine.ValidateBearer(token, world)
			if err != nil {
				writeUnauthorized(w, err)
				return
			}
			if resolved != "" {
				ctx := context.WithValue(r.Context(), authWorldKey{}, resolved)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractWorldFromRequest(r *http.Request) string {
	if q := r.URL.Query().Get("table_name"); q != "" {
		return q
	}
	if q := r.URL.Query().Get("world"); q != "" {
		return q
	}
	if r.Body == nil || r.ContentLength == 0 {
		return ""
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		return ""
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var probe struct {
		TableName  string `json:"table_name"`
		World      string `json:"world"`
		ParentPath string `json:"parent_path"`
	}
	_ = json.Unmarshal(body, &probe)
	if probe.TableName != "" {
		return probe.TableName
	}
	return probe.World
}

func writeUnauthorized(w http.ResponseWriter, err error) {
	msg := "unauthorized"
	if err != nil {
		msg = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(types.APIResponse{
		Success: false,
		Message: msg,
	})
}
