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

package provider

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/stretchr/testify/assert"

	"github.com/cloudbase/garm-provider-openstack/client"
)

func TestOpenstackServerToInstanceReportsFaultOfErroredServer(t *testing.T) {
	srv := client.ServerWithExt{
		Server: servers.Server{
			ID:     "d9072956-1560-487c-97f2-18bdf65ec749",
			Name:   "test-server",
			Status: "ERROR",
			Fault:  servers.Fault{Code: 500, Message: "No valid host was found.", Details: "scheduler"},
		},
	}

	instance := openstackServerToInstance(srv)

	var fault servers.Fault
	if assert.NoError(t, json.Unmarshal(instance.ProviderFault, &fault)) {
		assert.Equal(t, 500, fault.Code)
		assert.Equal(t, "No valid host was found.", fault.Message)
		assert.Equal(t, "scheduler", fault.Details)
	}
}

func TestOpenstackServerToInstanceHasNoFaultWhenHealthy(t *testing.T) {
	srv := client.ServerWithExt{Server: servers.Server{ID: "id", Name: "n", Status: "ACTIVE"}}
	assert.Empty(t, openstackServerToInstance(srv).ProviderFault)

	// An errored server without a recorded fault has nothing to report.
	srv.Status = "ERROR"
	assert.Empty(t, openstackServerToInstance(srv).ProviderFault)
}

func TestMergeExtraSpecsCreateTimeout(t *testing.T) {
	m := &machineSpec{}
	m.MergeExtraSpecs(extraSpecs{})
	assert.Equal(t, time.Duration(0), m.CreateTimeout, "unset keeps the client default")

	m.MergeExtraSpecs(extraSpecs{CreateTimeout: Ptr(int64(600))})
	assert.Equal(t, 10*time.Minute, m.CreateTimeout)
}
