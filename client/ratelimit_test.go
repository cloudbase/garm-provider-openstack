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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/testhelper"
	"github.com/gophercloud/gophercloud/v2/testhelper/client"
	"github.com/stretchr/testify/assert"

	"github.com/cloudbase/garm-provider-openstack/config"
)

const rateLimitedServerID = "d9072956-1560-487c-97f2-18bdf65ec749"

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		value string
		want  time.Duration
		ok    bool
	}{
		{name: "missing"},
		{name: "seconds", value: "7", want: 7 * time.Second, ok: true},
		{name: "zero", value: "0", ok: true},
		{name: "negative", value: "-3"},
		{name: "invalid", value: "soon"},
		{name: "http date", value: now.Add(90 * time.Second).Format(http.TimeFormat), want: 90 * time.Second, ok: true},
		{name: "past date", value: now.Add(-time.Minute).Format(http.TimeFormat), ok: true},
		{name: "large seconds", value: "9223372037", want: 9223372036 * time.Second, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := retryAfter(tt.value, now)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRateLimitDelay(t *testing.T) {
	tests := []struct {
		attempt uint
		min     time.Duration
		max     time.Duration
	}{
		{1, 1500 * time.Millisecond, 2500 * time.Millisecond},
		{2, 3 * time.Second, 5 * time.Second},
		{3, 6 * time.Second, 10 * time.Second},
		{4, 12 * time.Second, 20 * time.Second},
		{5, 24 * time.Second, 40 * time.Second},
		{6, 45 * time.Second, time.Minute},
		{10, 45 * time.Second, time.Minute},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.attempt), func(t *testing.T) {
			for range 20 {
				delay := rateLimitDelay(nil, tt.attempt, time.Now())
				assert.GreaterOrEqual(t, delay, tt.min)
				assert.LessOrEqual(t, delay, tt.max)
			}
		})
	}
}

func TestRateLimitBackoffHonorsRetryAfter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		respErr := &gophercloud.ErrUnexpectedResponseCode{
			Actual:         http.StatusTooManyRequests,
			ResponseHeader: http.Header{"Retry-After": {"45"}},
		}
		assert.NoError(t, rateLimitBackoff(context.Background(), respErr, nil, 1))
		assert.Equal(t, 45*time.Second, time.Since(start))
	})
}

func TestRateLimitDelayCapsRetryAfter(t *testing.T) {
	now := time.Date(2026, time.October, 14, 12, 0, 0, 0, time.UTC)
	for value, want := range map[string]time.Duration{
		"45":  45 * time.Second,
		"300": maxRateLimitBackoff,
		now.Add(time.Hour).Format(http.TimeFormat): maxRateLimitBackoff,
	} {
		respErr := &gophercloud.ErrUnexpectedResponseCode{
			Actual:         http.StatusTooManyRequests,
			ResponseHeader: http.Header{"Retry-After": {value}},
		}
		assert.Equal(t, want, rateLimitDelay(respErr, 1, now), value)
	}
}

func TestRateLimitBackoffCancellation(t *testing.T) {
	for _, alreadyCanceled := range []bool{false, true} {
		t.Run(fmt.Sprint(alreadyCanceled), func(t *testing.T) {
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
				respErr := &gophercloud.ErrUnexpectedResponseCode{
					Actual:         http.StatusTooManyRequests,
					ResponseHeader: http.Header{"Retry-After": {"30"}},
				}
				start := time.Now()
				assert.ErrorIs(t, rateLimitBackoff(ctx, respErr, nil, 1), context.Canceled)
				assert.LessOrEqual(t, time.Since(start), time.Second)
			})
		})
	}
}

func TestRateLimitRetriesBounded(t *testing.T) {
	for _, code := range []int{http.StatusTooManyRequests, 498, http.StatusForbidden, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			fakeServer := testhelper.SetupHTTP()
			defer fakeServer.Teardown()
			var requests atomic.Int32
			fakeServer.Mux.HandleFunc("/servers/"+rateLimitedServerID, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(code)
			})
			compute := client.ServiceClient(fakeServer)
			configureRateLimitRetries(compute.ProviderClient)
			osClient := &OpenstackClient{compute: compute}

			_, err := osClient.GetServer(context.Background(), rateLimitedServerID)
			assert.True(t, gophercloud.ResponseCodeIs(err, code), "got %v", err)
			want := int32(1)
			if code == http.StatusTooManyRequests || code == 498 {
				want = maxRateLimitRetries + 1
			}
			assert.Equal(t, want, requests.Load())
		})
	}
}

func TestCreateServerRetriesRateLimits(t *testing.T) {
	fakeServer := testhelper.SetupHTTP()
	defer fakeServer.Teardown()
	var creates, gets atomic.Int32
	fakeServer.Mux.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, http.MethodPost)
		var body struct {
			Server map[string]interface{} `json:"server"`
		}
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}
		assert.Equal(t, "test-server", body.Server["name"])
		if creates.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"server":{"id":%q}}`, rateLimitedServerID)
	})
	fakeServer.Mux.HandleFunc("/servers/"+rateLimitedServerID, func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, http.MethodGet)
		if gets.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"server":{"id":%q,"status":"ACTIVE","tags":["garm-controller-id=test-controller"]}}`, rateLimitedServerID)
	})
	compute := client.ServiceClient(fakeServer)
	configureRateLimitRetries(compute.ProviderClient)
	osClient := &OpenstackClient{compute: compute, controllerID: "test-controller"}

	srv, err := osClient.CreateServerFromImage(context.Background(), servers.CreateOpts{Name: "test-server", ImageRef: "image", FlavorRef: "flavor"})
	assert.NoError(t, err)
	assert.Equal(t, rateLimitedServerID, srv.ID)
	assert.Equal(t, int32(2), creates.Load())
	assert.Equal(t, int32(3), gets.Load())
}

func TestDeleteServerRetriesRateLimits(t *testing.T) {
	fakeServer := testhelper.SetupHTTP()
	defer fakeServer.Teardown()
	var lists, deletes, polls atomic.Int32
	fakeServer.Mux.HandleFunc("/servers/detail", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, http.MethodGet)
		if lists.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"servers":[{"id":%q,"name":"test-server"}]}`, rateLimitedServerID)
	})
	fakeServer.Mux.HandleFunc("/servers/"+rateLimitedServerID+"/action", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, http.MethodPost)
		if deletes.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	fakeServer.Mux.HandleFunc("/servers/"+rateLimitedServerID, func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, http.MethodGet)
		if polls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	compute := client.ServiceClient(fakeServer)
	configureRateLimitRetries(compute.ProviderClient)
	osClient := &OpenstackClient{compute: compute, controllerID: "test-controller"}

	assert.NoError(t, osClient.DeleteServer(context.Background(), "test-server", true))
	assert.Equal(t, int32(2), lists.Load())
	assert.Equal(t, int32(2), deletes.Load())
	assert.Equal(t, int32(2), polls.Load())
}

func TestNewClientConfiguresRateLimitRetries(t *testing.T) {
	for _, env := range os.Environ() {
		name, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(name, "OS_") {
			t.Setenv(name, "")
		}
	}
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("/v3/auth/tokens", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, http.MethodPost)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Subject-Token", "test-token")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"token":{"expires_at":"2099-01-01T00:00:00Z","catalog":[
			{"type":"compute","endpoints":[{"interface":"public","url":"%[1]s/compute/v2.1/"}]},
			{"type":"image","endpoints":[{"interface":"public","url":"%[1]s/image/v2/"}]},
			{"type":"network","endpoints":[{"interface":"public","url":"%[1]s/network/v2.0/"}]},
			{"type":"volumev3","endpoints":[{"interface":"public","url":"%[1]s/volume/v3/"}]}
		]}}`, server.URL)
	})
	clouds := filepath.Join(t.TempDir(), "clouds.yaml")
	err := os.WriteFile(clouds, []byte(fmt.Sprintf(`clouds:
  test:
    auth_type: v3password
    auth:
      auth_url: %s/v3/
      username: test-user
      password: test-password
      user_domain_name: Default
`, server.URL)), 0o600)
	if !assert.NoError(t, err) {
		return
	}
	osClient, err := NewClient(context.Background(), &config.Config{
		Cloud: "test", Credentials: config.Credentials{Clouds: clouds}, DefaultNetworkID: "test-network",
	}, "test-controller")
	if !assert.NoError(t, err) {
		return
	}
	for name, service := range map[string]*gophercloud.ServiceClient{
		"compute": osClient.compute, "image": osClient.image, "network": osClient.network, "volume": osClient.volume,
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotNil(t, service.RetryBackoffFunc)
			assert.Equal(t, uint(maxRateLimitRetries), service.MaxBackoffRetries)
		})
	}
}
