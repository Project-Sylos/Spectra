// Copyright 2025 Sylos contributors
// SPDX-License-Identifier: MIT License

package chaos

import (
	"testing"
	"time"

	"codeberg.org/Sylos/Spectra/internal/types"
)

func TestEngineGlobalRateLimit(t *testing.T) {
	cfg := &types.ChaosConfig{
		Enabled: true,
		RateLimits: types.ChaosRateLimits{
			PollIntervalMs: 1000,
			Global: types.ChaosCallLimit{
				CallsPerSecond: 2,
				Burst:          2,
			},
		},
		Backoff: types.ChaosBackoff{
			BaseRetryAfterMs:  100,
			ExponentialFactor: 2,
		},
	}
	engine := NewEngine(cfg, 42)

	for i := 0; i < 2; i++ {
		if err := engine.BeforeOperation(EndpointListChildren, 0); err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}
	err := engine.BeforeOperation(EndpointListChildren, 0)
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	rl, ok := IsRateLimited(err)
	if !ok || rl.RetryAfter != 100*time.Millisecond {
		t.Fatalf("first backoff got (%v, %v)", rl, ok)
	}
}

func TestEngineExponentialBackoff(t *testing.T) {
	cfg := &types.ChaosConfig{
		Enabled: true,
		RateLimits: types.ChaosRateLimits{
			Global: types.ChaosCallLimit{CallsPerSecond: 1, Burst: 1},
		},
		Backoff: types.ChaosBackoff{
			BaseRetryAfterMs:  100,
			ExponentialFactor: 2,
		},
	}
	engine := NewEngine(cfg, 42)
	_ = engine.BeforeOperation(EndpointListChildren, 0)

	err1 := engine.BeforeOperation(EndpointListChildren, 0)
	rl1, _ := IsRateLimited(err1)
	if rl1.RetryAfter != 100*time.Millisecond {
		t.Fatalf("attempt 1 retry=%v want 100ms", rl1.RetryAfter)
	}

	err2 := engine.BeforeOperation(EndpointListChildren, 0)
	rl2, _ := IsRateLimited(err2)
	if rl2.RetryAfter != 200*time.Millisecond {
		t.Fatalf("attempt 2 retry=%v want 200ms", rl2.RetryAfter)
	}

	err3 := engine.BeforeOperation(EndpointListChildren, 0)
	rl3, _ := IsRateLimited(err3)
	if rl3.RetryAfter != 400*time.Millisecond {
		t.Fatalf("attempt 3 retry=%v want 400ms", rl3.RetryAfter)
	}
}

func TestEngineFractionalBackoffFactor(t *testing.T) {
	cfg := &types.ChaosConfig{
		Enabled: true,
		RateLimits: types.ChaosRateLimits{
			Global: types.ChaosCallLimit{CallsPerSecond: 1, Burst: 1},
		},
		Backoff: types.ChaosBackoff{
			BaseRetryAfterMs:  100,
			ExponentialFactor: 1.5,
		},
	}
	engine := NewEngine(cfg, 42)
	_ = engine.BeforeOperation(EndpointListChildren, 0)
	_ = engine.BeforeOperation(EndpointListChildren, 0)
	err := engine.BeforeOperation(EndpointListChildren, 0)
	rl, _ := IsRateLimited(err)
	want := time.Duration(150) * time.Millisecond
	if rl.RetryAfter != want {
		t.Fatalf("retry=%v want %v", rl.RetryAfter, want)
	}
}

func TestEngineOperationSpecificLimit(t *testing.T) {
	cfg := &types.ChaosConfig{
		Enabled: true,
		RateLimits: types.ChaosRateLimits{
			Operations: map[string]types.ChaosCallLimit{
				"create_folder": {CallsPerSecond: 1, Burst: 1},
			},
		},
		Backoff: types.ChaosBackoff{BaseRetryAfterMs: 50, ExponentialFactor: 2},
	}
	engine := NewEngine(cfg, 42)
	if err := engine.BeforeOperation(EndpointCreateFolder, 0); err != nil {
		t.Fatal(err)
	}
	if err := engine.BeforeOperation(EndpointCreateFolder, 0); err == nil {
		t.Fatal("expected create_folder limit")
	}
	// list_children not limited
	if err := engine.BeforeOperation(EndpointListChildren, 0); err != nil {
		t.Fatalf("list should pass: %v", err)
	}
}

func TestLegacyRateLimitConfig(t *testing.T) {
	cfg := &types.ChaosConfig{
		Enabled: true,
		RateLimit: types.ChaosRateLimit{
			RequestsPerSecond: 1,
			Burst:             1,
			RetryAfterMs:      75,
		},
	}
	n := cfg.NormalizedChaos()
	if n.RateLimits.Global.CallsPerSecond != 1 {
		t.Fatalf("global=%v", n.RateLimits.Global.CallsPerSecond)
	}
	if n.Backoff.BaseRetryAfterMs != 75 {
		t.Fatalf("base=%v", n.Backoff.BaseRetryAfterMs)
	}
}
