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
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/testhelper"
	"github.com/gophercloud/gophercloud/v2/testhelper/client"
	"github.com/stretchr/testify/assert"
)

const faultServerID = "d9072956-1560-487c-97f2-18bdf65ec749"

// setupServer registers a server whose GET returns body until it is deleted.
func setupServer(t *testing.T, body string) (*OpenstackClient, *atomic.Bool, func()) {
	t.Helper()
	fakeServer := testhelper.SetupHTTP()
	var deleted atomic.Bool

	fakeServer.Mux.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `{"server":{"id":%q}}`, faultServerID)
	})
	fakeServer.Mux.HandleFunc("/servers/"+faultServerID, func(w http.ResponseWriter, r *http.Request) {
		if deleted.Load() {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	})
	fakeServer.Mux.HandleFunc("/servers/"+faultServerID+"/action", func(w http.ResponseWriter, r *http.Request) {
		deleted.Store(true)
		w.WriteHeader(http.StatusAccepted)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(fakeServer),
		controllerID: "my-controller-id",
	}
	return osClient, &deleted, fakeServer.Teardown
}

func TestCreateServerFromImageReportsNovaFault(t *testing.T) {
	osClient, deleted, teardown := setupServer(t, fmt.Sprintf(`{"server":{
		"id":%q,
		"status":"ERROR",
		"tags":["garm-controller-id=my-controller-id"],
		"fault":{"code":500,"message":"No valid host was found.","details":"Traceback: scheduler"}
	}}`, faultServerID))
	defer teardown()

	_, err := osClient.CreateServerFromImage(context.Background(), servers.CreateOpts{
		Name:      "test-server",
		ImageRef:  "aee1d242-730f-431f-88c1-87630c0f07ba",
		FlavorRef: "flavor-uuid",
	})
	if !assert.Error(t, err) {
		return
	}

	var faultErr *ServerFaultError
	if !assert.True(t, errors.As(err, &faultErr), "error %q does not carry a ServerFaultError", err) {
		return
	}
	assert.Equal(t, faultServerID, faultErr.ServerID)
	assert.Equal(t, 500, faultErr.Fault.Code)
	assert.Equal(t, "No valid host was found.", faultErr.Fault.Message)
	assert.Contains(t, err.Error(), "instance in ERROR state: No valid host was found. (code 500)")
	assert.Contains(t, err.Error(), "Traceback: scheduler")
	assert.True(t, deleted.Load(), "server was not deleted after ERROR")
}

func TestServerFaultErrorWithoutFault(t *testing.T) {
	assert.Equal(t, "instance in ERROR state", (&ServerFaultError{}).Error())
}

func TestServerFaultErrorTruncatesDetails(t *testing.T) {
	e := &ServerFaultError{Fault: servers.Fault{Code: 500, Message: "boom", Details: strings.Repeat("x", 10*maxFaultDetails)}}
	assert.Less(t, len(e.Error()), 2*maxFaultDetails)
	assert.True(t, strings.HasSuffix(e.Error(), "..."))
}

func TestCreateServerFromVolumeActiveTimeoutIsConfigurable(t *testing.T) {
	osClient, deleted, teardown := setupServer(t, fmt.Sprintf(`{"server":{
		"id":%q,
		"status":"BUILD",
		"tags":["garm-controller-id=my-controller-id"]
	}}`, faultServerID))
	defer teardown()

	start := time.Now()
	_, err := osClient.CreateServerFromVolume(context.Background(), servers.CreateOpts{
		Name:      "test-server",
		FlavorRef: "flavor-uuid",
	}, "test-server", WithActiveTimeout(50*time.Millisecond))
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "did not reach ACTIVE state after 0.05 seconds")
	assert.Less(t, time.Since(start), 30*time.Second)
	assert.True(t, deleted.Load(), "server was not deleted after the timeout")
}

func TestNewCreateSettingsDefault(t *testing.T) {
	assert.Equal(t, DefaultActiveTimeout, newCreateSettings(nil).activeTimeout)
	assert.Equal(t, DefaultActiveTimeout, newCreateSettings([]CreateOption{WithActiveTimeout(0)}).activeTimeout)
	assert.Equal(t, 5*time.Minute, newCreateSettings([]CreateOption{WithActiveTimeout(5 * time.Minute)}).activeTimeout)
}
