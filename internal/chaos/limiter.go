// Copyright 2025 Sylos contributors
// SPDX-License-Identifier: MIT License

package chaos

import (
	"math"
	"strings"
	"sync"
	"time"

	"codeberg.org/Sylos/Spectra/internal/types"
)

// rateMiddleware tracks fixed-window call/byte usage and exponential backoff state.
type rateMiddleware struct {
	mu sync.Mutex

	interval time.Duration
	cfg      types.ChaosConfig

	windowStart   time.Time
	globalCalls   int64
	opCalls       map[string]int64
	bytesUsed     int64

	// consecutive rejections per scope key → exponential backoff attempt index
	backoffAttempts map[string]int
}

func newRateMiddleware(cfg types.ChaosConfig) *rateMiddleware {
	n := cfg.NormalizedChaos()
	return &rateMiddleware{
		interval:        n.PollInterval(),
		cfg:             n,
		opCalls:         make(map[string]int64),
		backoffAttempts: make(map[string]int),
		windowStart:     time.Now(),
	}
}

func (m *rateMiddleware) enabled() bool {
	n := m.cfg.NormalizedChaos()
	g := n.RateLimits.Global.CallsPerSecond
	if g > 0 {
		return true
	}
	if len(n.RateLimits.Operations) > 0 {
		return true
	}
	if n.RateLimits.Bandwidth.BytesPerSecond > 0 {
		return true
	}
	return false
}

func (m *rateMiddleware) checkAndConsume(endpoint string, bytes int64) error {
	if !m.enabled() {
		return nil
	}
	opKey := operationConfigKey(endpoint)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.advanceWindowLocked(time.Now())

	if err := m.checkCallsLocked("global", m.cfg.RateLimits.Global, m.globalCalls, 1); err != nil {
		return m.rateLimitErrLocked("global", endpoint, err)
	}
	if lim, ok := m.cfg.RateLimits.Operations[opKey]; ok && lim.CallsPerSecond > 0 {
		if err := m.checkCallsLocked(opKey, lim, m.opCalls[opKey], 1); err != nil {
			return m.rateLimitErrLocked(opKey, endpoint, err)
		}
	}
	if bytes > 0 {
		if err := m.checkBytesLocked(bytes); err != nil {
			return m.rateLimitErrLocked("bandwidth", endpoint, err)
		}
	}

	m.globalCalls++
	m.opCalls[opKey]++
	if bytes > 0 {
		m.bytesUsed += bytes
	}
	m.backoffAttempts["global"] = 0
	m.backoffAttempts[opKey] = 0
	m.backoffAttempts["bandwidth"] = 0
	return nil
}

func (m *rateMiddleware) addBytes(endpoint string, bytes int64) error {
	if bytes <= 0 || m.cfg.RateLimits.Bandwidth.BytesPerSecond <= 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.advanceWindowLocked(time.Now())
	if err := m.checkBytesLocked(bytes); err != nil {
		return m.rateLimitErrLocked("bandwidth", endpoint, err)
	}
	m.bytesUsed += bytes
	m.backoffAttempts["bandwidth"] = 0
	return nil
}

func (m *rateMiddleware) checkBytesLocked(add int64) error {
	bw := m.cfg.RateLimits.Bandwidth
	if bw.BytesPerSecond <= 0 {
		return nil
	}
	limit := int64(bw.BytesPerSecond * m.interval.Seconds())
	if bw.BurstBytes > limit {
		limit = bw.BurstBytes
	}
	if limit <= 0 {
		limit = int64(bw.BytesPerSecond)
	}
	if m.bytesUsed+add > limit {
		return errOverLimit
	}
	return nil
}

var errOverLimit = &limitError{}

type limitError struct{}

func (limitError) Error() string { return "over limit" }

func (m *rateMiddleware) checkCallsLocked(scope string, lim types.ChaosCallLimit, used int64, delta int64) error {
	if lim.CallsPerSecond <= 0 {
		return nil
	}
	windowSec := m.interval.Seconds()
	if windowSec <= 0 {
		windowSec = 1
	}
	limit := int64(lim.CallsPerSecond * windowSec)
	if lim.Burst > 0 && int64(lim.Burst) > limit {
		limit = int64(lim.Burst)
	}
	if limit <= 0 {
		limit = int64(lim.CallsPerSecond)
	}
	if used+delta > limit {
		return errOverLimit
	}
	return nil
}

func (m *rateMiddleware) advanceWindowLocked(now time.Time) {
	if m.windowStart.IsZero() || now.Sub(m.windowStart) >= m.interval {
		m.windowStart = now
		m.globalCalls = 0
		m.opCalls = make(map[string]int64)
		m.bytesUsed = 0
	}
}

func (m *rateMiddleware) rateLimitErrLocked(scope, endpoint string, _ error) error {
	attempt := m.backoffAttempts[scope]
	m.backoffAttempts[scope] = attempt + 1
	retry := m.retryAfterLocked(attempt)
	return &ErrRateLimited{RetryAfter: retry, Endpoint: endpoint, Scope: scope, Attempt: attempt + 1}
}

func (m *rateMiddleware) retryAfterLocked(attempt int) time.Duration {
	base := float64(m.cfg.Backoff.BaseRetryAfterMs)
	if base <= 0 {
		base = 250
	}
	factor := m.cfg.Backoff.ExponentialFactor
	if factor <= 1 {
		factor = 2
	}
	mult := math.Pow(factor, float64(attempt))
	ms := base * mult
	if ms > float64(60*time.Second/time.Millisecond) {
		ms = float64(60 * time.Second / time.Millisecond)
	}
	return time.Duration(ms) * time.Millisecond
}

func operationConfigKey(endpoint string) string {
	switch endpoint {
	case EndpointListChildren:
		return "list_children"
	case EndpointGetNode:
		return "get_node"
	case EndpointGetFileData:
		return "get_file_data"
	case EndpointCreateFolder:
		return "create_folder"
	case EndpointUploadFile:
		return "upload_file"
	default:
		return strings.ToLower(endpoint)
	}
}
