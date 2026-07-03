// Copyright 2025 Sylos contributors
// SPDX-License-Identifier: MIT License

package sdk

import (
	"codeberg.org/Sylos/Spectra/internal/chaos"
)

// RateLimitedError is returned when Spectra chaos rate limiting rejects a request.
type RateLimitedError = chaos.ErrRateLimited

// IsRateLimited reports whether err is a Spectra rate limit error.
func IsRateLimited(err error) (*RateLimitedError, bool) {
	return chaos.IsRateLimited(err)
}
