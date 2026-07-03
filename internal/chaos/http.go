// Copyright 2025 Sylos contributors
// SPDX-License-Identifier: MIT License

package chaos

import (
	"net/http"
	"strconv"
)

// WriteRateLimitedResponse writes a 429 with Retry-After header.
func WriteRateLimitedResponse(w http.ResponseWriter, rl *ErrRateLimited) {
	if rl == nil {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	secs := int(rl.RetryAfter.Seconds())
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"success":false,"message":"rate limited"}`))
}

// HTTPMiddleware applies chaos rate limiting to HTTP requests before handlers run.
func HTTPMiddleware(engine *Engine) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if engine == nil || !engine.Enabled() {
				next.ServeHTTP(w, r)
				return
			}
			endpoint := r.Method + " " + r.URL.Path
			if err := engine.BeforeOperation(endpoint, 0); err != nil {
				if rl, ok := IsRateLimited(err); ok {
					WriteRateLimitedResponse(w, rl)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
