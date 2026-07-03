package types

import "time"

// ChaosConfig configures simulated rate limits, latency, packet loss, and backoff for testing.
type ChaosConfig struct {
	Enabled bool `json:"enabled"`

	// RateLimits is the granular limit subsection (global, per-operation, bandwidth).
	RateLimits ChaosRateLimits `json:"rate_limits"`

	// Backoff configures exponential retry-after penalties when a limit is exceeded.
	Backoff ChaosBackoff `json:"backoff"`

	// Deprecated: use rate_limits.global and backoff.base_retry_after_ms.
	RateLimit ChaosRateLimit `json:"rate_limit,omitempty"`

	Latency    ChaosLatency    `json:"latency"`
	PacketLoss ChaosPacketLoss `json:"packet_loss"`
}

// ChaosRateLimits groups windowed call and bandwidth limits (Dropbox-style granularity).
type ChaosRateLimits struct {
	// PollIntervalMs is the fixed window size for counting usage (default 1000).
	PollIntervalMs int `json:"poll_interval_ms"`

	Global     ChaosCallLimit            `json:"global"`
	Operations map[string]ChaosCallLimit `json:"operations"`
	Bandwidth  ChaosBandwidthLimit       `json:"bandwidth"`
}

// ChaosCallLimit limits API calls per poll window for a scope (global or operation name).
type ChaosCallLimit struct {
	CallsPerSecond float64 `json:"calls_per_second"`
	Burst          int     `json:"burst,omitempty"`
}

// ChaosBandwidthLimit limits bytes moved per poll window (reads + writes).
type ChaosBandwidthLimit struct {
	BytesPerSecond float64 `json:"bytes_per_second"`
	BurstBytes     int64   `json:"burst_bytes,omitempty"`
}

// ChaosBackoff configures exponential retry-after when rate limited.
type ChaosBackoff struct {
	// BaseRetryAfterMs is the delay at the first consecutive rejection (default 250).
	BaseRetryAfterMs int `json:"base_retry_after_ms"`
	// ExponentialFactor multiplies delay per consecutive rejection: base * factor^attempt (default 2, must be > 1).
	ExponentialFactor float64 `json:"exponential_factor"`
}

// ChaosRateLimit is the legacy flat rate limit block (still supported).
type ChaosRateLimit struct {
	RequestsPerSecond float64 `json:"requests_per_second"`
	Burst             int     `json:"burst"`
	RetryAfterMs      int     `json:"retry_after_ms"`
}

type ChaosLatency struct {
	ListChildrenMs int `json:"list_children_ms"`
	JitterMs       int `json:"jitter_ms"`
}

type ChaosPacketLoss struct {
	Probability  float64 `json:"probability"`
	RetryAfterMs int     `json:"retry_after_ms"`
}

// NormalizedChaos returns config with defaults applied and legacy fields merged.
func (c *ChaosConfig) NormalizedChaos() ChaosConfig {
	if c == nil {
		return ChaosConfig{}
	}
	out := *c
	if out.RateLimits.PollIntervalMs <= 0 {
		out.RateLimits.PollIntervalMs = 1000
	}
	if out.RateLimit.RequestsPerSecond > 0 && out.RateLimits.Global.CallsPerSecond <= 0 {
		out.RateLimits.Global.CallsPerSecond = out.RateLimit.RequestsPerSecond
		out.RateLimits.Global.Burst = out.RateLimit.Burst
	}
	if out.RateLimit.RetryAfterMs > 0 && out.Backoff.BaseRetryAfterMs <= 0 {
		out.Backoff.BaseRetryAfterMs = out.RateLimit.RetryAfterMs
	}
	if out.Backoff.BaseRetryAfterMs <= 0 {
		out.Backoff.BaseRetryAfterMs = 250
	}
	if out.Backoff.ExponentialFactor <= 1 {
		out.Backoff.ExponentialFactor = 2
	}
	return out
}

// PollInterval returns the rate-limit window duration.
func (c *ChaosConfig) PollInterval() time.Duration {
	n := c.NormalizedChaos().RateLimits.PollIntervalMs
	if n <= 0 {
		return time.Second
	}
	return time.Duration(n) * time.Millisecond
}
