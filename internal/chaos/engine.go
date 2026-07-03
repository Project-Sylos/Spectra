// Copyright 2025 Sylos contributors
// SPDX-License-Identifier: MIT License

package chaos

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"codeberg.org/Sylos/Spectra/internal/types"
)

// ErrRateLimited indicates the request was throttled. RetryAfter may be zero.
type ErrRateLimited struct {
	RetryAfter time.Duration
	Endpoint   string
	Scope      string // global, operation key, or bandwidth
	Attempt    int    // consecutive rejection count for this scope
}

func (e *ErrRateLimited) Error() string {
	if e == nil {
		return "spectra: rate limited"
	}
	if e.Endpoint != "" && e.Scope != "" {
		return fmt.Sprintf("spectra: rate limited on %s scope=%s attempt=%d (retry after %s)", e.Endpoint, e.Scope, e.Attempt, e.RetryAfter)
	}
	return fmt.Sprintf("spectra: rate limited (retry after %s)", e.RetryAfter)
}

// IsRateLimited reports whether err is a rate limit from chaos simulation.
func IsRateLimited(err error) (*ErrRateLimited, bool) {
	var rl *ErrRateLimited
	if errors.As(err, &rl) && rl != nil {
		return rl, true
	}
	return nil, false
}

// Endpoint names used for per-operation limits.
const (
	EndpointListChildren = "ListChildren"
	EndpointGetNode      = "GetNode"
	EndpointGetFileData  = "GetFileData"
	EndpointCreateFolder = "CreateFolder"
	EndpointUploadFile   = "UploadFile"
)

// Engine applies configured chaos (rate limits, latency, packet loss) to FS operations.
type Engine struct {
	cfg    types.ChaosConfig
	seed   int64
	limits *rateMiddleware
	mu     sync.Mutex
	rng    *rand.Rand
}

// NewEngine builds a chaos engine from config. Nil or disabled config returns a no-op engine.
func NewEngine(raw *types.ChaosConfig, globalSeed int64) *Engine {
	if raw == nil || !raw.Enabled {
		return &Engine{rng: rand.New(rand.NewSource(globalSeed))}
	}
	n := raw.NormalizedChaos()
	e := &Engine{
		cfg:  n,
		seed: globalSeed,
		rng:  rand.New(rand.NewSource(globalSeed)),
	}
	if n.RateLimits.Global.CallsPerSecond > 0 ||
		len(n.RateLimits.Operations) > 0 ||
		n.RateLimits.Bandwidth.BytesPerSecond > 0 ||
		raw.RateLimit.RequestsPerSecond > 0 {
		e.limits = newRateMiddleware(n)
	}
	return e
}

// Enabled reports whether chaos is active.
func (e *Engine) Enabled() bool {
	return e != nil && e.cfg.Enabled
}

// BeforeOperation runs latency, packet loss, and rate limit checks. bytes is transfer size when known (writes).
func (e *Engine) BeforeOperation(endpoint string, bytes int64) error {
	if e == nil || !e.cfg.Enabled {
		return nil
	}
	if err := e.maybePacketLoss(endpoint); err != nil {
		return err
	}
	if err := e.applyLatency(endpoint); err != nil {
		return err
	}
	if e.limits != nil {
		return e.limits.checkAndConsume(endpoint, bytes)
	}
	return nil
}

// AfterOperation records byte usage for bandwidth limits (e.g. after a read completes).
func (e *Engine) AfterOperation(endpoint string, bytes int64) error {
	if e == nil || !e.cfg.Enabled || e.limits == nil {
		return nil
	}
	return e.limits.addBytes(endpoint, bytes)
}

func (e *Engine) packetLossRetryAfter() time.Duration {
	ms := e.cfg.PacketLoss.RetryAfterMs
	if ms <= 0 {
		ms = 500
	}
	return time.Duration(ms) * time.Millisecond
}

func (e *Engine) applyLatency(endpoint string) error {
	if endpoint != EndpointListChildren {
		return nil
	}
	base := e.cfg.Latency.ListChildrenMs
	if base <= 0 && e.cfg.Latency.JitterMs <= 0 {
		return nil
	}
	delay := base
	if e.cfg.Latency.JitterMs > 0 {
		e.mu.Lock()
		j := e.rng.Intn(e.cfg.Latency.JitterMs + 1)
		e.mu.Unlock()
		delay += j
	}
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return nil
}

func (e *Engine) maybePacketLoss(endpoint string) error {
	p := e.cfg.PacketLoss.Probability
	if p <= 0 {
		return nil
	}
	e.mu.Lock()
	roll := e.rng.Float64()
	e.mu.Unlock()
	if roll < p {
		return &ErrRateLimited{RetryAfter: e.packetLossRetryAfter(), Endpoint: endpoint + ":packet_loss", Scope: "packet_loss", Attempt: 1}
	}
	return nil
}
