// Copyright 2026 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package client

import (
	"context"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/gophercloud/gophercloud/v2"
)

const (
	maxRateLimitRetries  = 10
	baseRateLimitBackoff = 2 * time.Second
	maxRateLimitBackoff  = time.Minute
)

func configureRateLimitRetries(provider *gophercloud.ProviderClient) {
	provider.MaxBackoffRetries = maxRateLimitRetries
	provider.RetryBackoffFunc = rateLimitBackoff
}

func rateLimitBackoff(ctx context.Context, respErr *gophercloud.ErrUnexpectedResponseCode, _ error, failCount uint) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	timer := time.NewTimer(rateLimitDelay(respErr, failCount, time.Now()))
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	return ctx.Err()
}

func rateLimitDelay(respErr *gophercloud.ErrUnexpectedResponseCode, failCount uint, now time.Time) time.Duration {
	if respErr != nil {
		if delay, ok := retryAfter(respErr.ResponseHeader.Get("Retry-After"), now); ok {
			// Honor the server's delay up to the backoff ceiling, so that all
			// retries together stay well inside GARM's runner bootstrap timeout.
			return min(delay, maxRateLimitBackoff)
		}
	}

	delay := baseRateLimitBackoff
	for attempt := uint(1); attempt < failCount && delay < maxRateLimitBackoff; attempt++ {
		delay *= 2
	}
	delay = min(delay, maxRateLimitBackoff)
	// Spread retries from independent provider processes over a 50% window.
	jitter := time.Duration(rand.Int64N(int64(delay / 2))) //nolint:gosec // Retry jitter does not require cryptographic randomness.
	return min(delay*3/4+jitter, maxRateLimitBackoff)
}

func retryAfter(value string, now time.Time) (time.Duration, bool) {
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil {
		// Saturate before converting seconds to a duration to avoid overflow.
		seconds = min(seconds, uint64((1<<63-1)/time.Second))
		return time.Duration(seconds) * time.Second, true
	}
	if when, err := http.ParseTime(value); err == nil {
		return max(when.Sub(now), 0), true
	}
	return 0, false
}
