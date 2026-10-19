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
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitForPollInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var checks []time.Duration
		err := waitFor(context.Background(), func(context.Context) (bool, error) {
			checks = append(checks, time.Since(start))
			if len(checks) == 2 {
				time.Sleep(12 * time.Second)
			}
			return len(checks) == 3, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, []time.Duration{0, 5 * time.Second, 22 * time.Second}, checks)
	})
}

func TestWaitForStopsOnError(t *testing.T) {
	want := errors.New("poll failed")
	calls := 0
	err := waitFor(context.Background(), func(context.Context) (bool, error) {
		calls++
		return false, want
	})
	assert.ErrorIs(t, err, want)
	assert.Equal(t, 1, calls)
}

func TestWaitForCancellation(t *testing.T) {
	for _, alreadyCanceled := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if alreadyCanceled {
				cancel()
			} else {
				go func() {
					time.Sleep(time.Second)
					cancel()
				}()
			}
			calls := 0
			err := waitFor(ctx, func(context.Context) (bool, error) {
				calls++
				return false, nil
			})
			assert.ErrorIs(t, err, context.Canceled)
			wantCalls := 1
			if alreadyCanceled {
				wantCalls = 0
			}
			assert.Equal(t, wantCalls, calls)
		})
	}
}
