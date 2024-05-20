// Copyright 2023 Cloudbase Solutions SRL
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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-openstack/client"
	"github.com/cloudbase/garm-provider-openstack/config"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/testhelper"
	thclient "github.com/gophercloud/gophercloud/testhelper/client"
	"github.com/stretchr/testify/assert"
)

func TestOpenstackServerToInstance(t *testing.T) {
	srv := client.ServerWithExt{
		Server: servers.Server{
			ID:   "d9072956-1560-487c-97f2-18bdf65ec749",
			Name: "test-server",
			Addresses: map[string]interface{}{
				"network": []interface{}{
					map[string]interface{}{
						"OS-EXT-IPS:type": "fixed",
						"addr":            "10.10.0.4",
					},
				},
			},
			Metadata: map[string]string{
				"os_arch":    "amd64",
				"os_type":    "linux",
				"os_name":    "ubuntu",
				"os_version": "20.04",
			},
			Status: "ACTIVE",
		},
	}
	expectedInstance := params.ProviderInstance{
		ProviderID: "d9072956-1560-487c-97f2-18bdf65ec749",
		Name:       "test-server",
		OSArch:     "amd64",
		OSType:     "linux",
		Status:     "running",
		OSName:     "ubuntu",
		OSVersion:  "20.04",
		Addresses: []params.Address{
			{
				Type:    params.PrivateAddress,
				Address: "10.10.0.4",
			},
		},
	}

	instance := openstackServerToInstance(srv)
	assert.Equal(t, expectedInstance, instance)
}

func TestCreateInstance(t *testing.T) {
	ctx := context.Background()
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()
	provider := &openstackProvider{
		cfg: &config.Config{
			Cloud: "mycloud",
			Credentials: config.Credentials{
				Clouds: "../testdata/clouds.yaml",
			},
			DefaultNetworkID:     "test-network",
			AllowedImageOwners:   []string{"owner1", "owner2"},
			ImageVisibility:      "public",
			DisableUpdatesOnBoot: false,
			EnableBootDebug:      true,
		},
		cli:          &client.OpenstackClient{},
		controllerID: "my-controller-id",
	}
	serviceClient := thclient.ServiceClient()
	mockCli := client.NewTestOpenStackClient(serviceClient, "my-controller-id")
	provider.cli = mockCli
	data := params.BootstrapInstance{
		Name:          "test-instance",
		InstanceToken: "test-token",
		OSArch:        params.Amd64,
		OSType:        params.Linux,
		Flavor:        "m1.micro",
		Image:         "ubuntu-20.04",
		Tools: []params.RunnerApplicationDownload{
			{
				OS:                Ptr("linux"),
				Architecture:      Ptr("x64"),
				DownloadURL:       Ptr("http://test.com"),
				Filename:          Ptr("runner.tar.gz"),
				SHA256Checksum:    Ptr("sha256:1123"),
				TempDownloadToken: Ptr("test-token"),
			},
		},
		ExtraSpecs: json.RawMessage(`{
			"security_groups": ["allow_ssh", "allow_web"],
			"allowed_image_owners": ["123456"],
			"image_visibility": "public",
			"network_id": "542b68dd-4b3d-459d-8531-34d5e779d4d6",
			"storage_backend": "cinder_nvme",
			"boot_from_volume": true,
			"boot_disk_size": 150,
			"use_config_drive": false,
			"enable_boot_debug": false
		}`),
		PoolID: "test-pool",
	}
	DefaultToolFetch = func(osType params.OSType, osArch params.OSArch, tools []params.RunnerApplicationDownload) (params.RunnerApplicationDownload, error) {
		return data.Tools[0], nil
	}

	// Mock the response for flavor get by ID
	testhelper.Mux.HandleFunc("/flavors/detail", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"flavors": [
			{
				"id": "flavor-uuid",
				"name": "m1.micro",
				"ram": 1024,
				"vcpus": 1,
				"disk": 10
			}
		]
		}`)
	})

	// Mock the response for network get by ID
	testhelper.Mux.HandleFunc("/networks/542b68dd-4b3d-459d-8531-34d5e779d4d6", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"network": {
			"id": "542b68dd-4b3d-459d-8531-34d5e779d4d6",
			"name": "test-network"
		}
		}`)
	})

	// Mock the response for image get by ID
	testhelper.Mux.HandleFunc("/images", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"images": [
			{
				"name": "ubuntu-20.04",
				"id": "aee1d242-730f-431f-88c1-87630c0f07ba",
				"status": "ACTIVE",
				"owner": "123456",
				"visibility": "public"
			}
		]
		}`)
	})

	// Mock the response for server create
	testhelper.Mux.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "POST")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `
		{
		"server": {
			"id": "d9072956-1560-487c-97f2-18bdf65ec749",
			"name": "test-instance",
			"addresses": {
				"network": [
					{
						"OS-EXT-IPS:type": "fixed",
						"addr": "10.10.0.4",
						"version": 4
					}
				]
			},
			"metadata": {
				"os_arch": "amd64",
				"os_type": "linux",
				"os_name": "ubuntu",
				"os_version": "20.04"
			},
			"tags": ["garm-controller-id=my-controller-id"],
			"status": "ACTIVE"
		}
		}`)
	})

	// Mock the response for server get by ID
	testhelper.Mux.HandleFunc("/servers/d9072956-1560-487c-97f2-18bdf65ec749", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"server": {
			"id": "d9072956-1560-487c-97f2-18bdf65ec749",
			"name": "test-instance",
			"addresses": {
				"network": [
					{
						"OS-EXT-IPS:type": "fixed",
						"addr": "10.10.0.4",
						"version": 4
					}
				]
			},
			"metadata": {
				"os_arch": "amd64",
				"os_type": "linux",
				"os_name": "ubuntu",
				"os_version": "20.04"
			},
			"tags": ["garm-controller-id=my-controller-id"],
			"status": "ACTIVE"
		}
		}`)
	})

	expectedOutput := params.ProviderInstance{
		ProviderID: "d9072956-1560-487c-97f2-18bdf65ec749",
		Name:       "test-instance",
		OSArch:     "amd64",
		OSType:     "linux",
		Status:     "running",
		OSName:     "ubuntu",
		OSVersion:  "20.04",
		Addresses: []params.Address{
			{
				Type:    params.PrivateAddress,
				Address: "10.10.0.4",
			},
		},
	}

	instance, err := provider.CreateInstance(ctx, data)
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, instance)
}

func TestDeleteInstance(t *testing.T) {
	ctx := context.Background()
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()
	provider := &openstackProvider{
		cfg: &config.Config{
			Cloud: "mycloud",
			Credentials: config.Credentials{
				Clouds: "../testdata/clouds.yaml",
			},
			DefaultNetworkID:     "test-network",
			AllowedImageOwners:   []string{"owner1", "owner2"},
			ImageVisibility:      "public",
			DisableUpdatesOnBoot: false,
			EnableBootDebug:      true,
		},
		controllerID: "my-controller-id",
	}
	serviceClient := thclient.ServiceClient()
	mockCli := client.NewTestOpenStackClient(serviceClient, "my-controller-id")
	provider.cli = mockCli

	// Mock the response for server get by ID
	testhelper.Mux.HandleFunc("/servers/d9072956-1560-487c-97f2-18bdf65ec749", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"server": {
			"id": "d9072956-1560-487c-97f2-18bdf65ec749",
			"name": "test-server",
			"status": "DELETED",
			"tags": ["garm-controller-id=my-controller-id"],
			"forceDelete": true
		}
		}`)
	})

	// Mock the response for server deletion
	testhelper.Mux.HandleFunc("/servers/d9072956-1560-487c-97f2-18bdf65ec749/action", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "POST")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `
		{
		"server": {
			"id": "d9072956-1560-487c-97f2-18bdf65ec749",
			"name": "test-server",
			"status": "DELETED",
			"tags": ["garm-controller-id=my-controller-id"],
			"forceDelete": true
		}
		}`)
	})

	err := provider.DeleteInstance(ctx, "d9072956-1560-487c-97f2-18bdf65ec749")
	assert.NoError(t, err)
}

func TestGetInstance(t *testing.T) {
	ctx := context.Background()
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()
	provider := &openstackProvider{
		cfg: &config.Config{
			Cloud: "mycloud",
			Credentials: config.Credentials{
				Clouds: "../testdata/clouds.yaml",
			},
			DefaultNetworkID:     "test-network",
			AllowedImageOwners:   []string{"owner1", "owner2"},
			ImageVisibility:      "public",
			DisableUpdatesOnBoot: false,
			EnableBootDebug:      true,
		},
		controllerID: "my-controller-id",
	}
	serviceClient := thclient.ServiceClient()
	mockCli := client.NewTestOpenStackClient(serviceClient, "my-controller-id")
	provider.cli = mockCli

	// Mock the response for server get by ID
	testhelper.Mux.HandleFunc("/servers/d9072956-1560-487c-97f2-18bdf65ec749", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"server": {
			"id": "d9072956-1560-487c-97f2-18bdf65ec749",
			"name": "test-instance",
			"addresses": {
				"network": [
					{
						"OS-EXT-IPS:type": "fixed",
						"addr": "10.10.0.4",
						"version": 4
					}
				]
			},
			"metadata": {
				"os_arch": "amd64",
				"os_type": "linux",
				"os_name": "ubuntu",
				"os_version": "20.04"
			},
			"tags": ["garm-controller-id=my-controller-id"],
			"status": "ACTIVE"
		}
		}`)
	})

	expectedOutput := params.ProviderInstance{
		ProviderID: "d9072956-1560-487c-97f2-18bdf65ec749",
		Name:       "test-instance",
		OSArch:     "amd64",
		OSType:     "linux",
		Status:     "running",
		OSName:     "ubuntu",
		OSVersion:  "20.04",
		Addresses: []params.Address{
			{
				Type:    params.PrivateAddress,
				Address: "10.10.0.4",
			},
		},
	}

	instance, err := provider.GetInstance(ctx, "d9072956-1560-487c-97f2-18bdf65ec749")
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, instance)
}

func TestListInstances(t *testing.T) {
	ctx := context.Background()
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()
	provider := &openstackProvider{
		cfg: &config.Config{
			Cloud: "mycloud",
			Credentials: config.Credentials{
				Clouds: "../testdata/clouds.yaml",
			},
			DefaultNetworkID:     "test-network",
			AllowedImageOwners:   []string{"owner1", "owner2"},
			ImageVisibility:      "public",
			DisableUpdatesOnBoot: false,
			EnableBootDebug:      true,
		},
		controllerID: "my-controller-id",
	}
	serviceClient := thclient.ServiceClient()
	mockCli := client.NewTestOpenStackClient(serviceClient, "my-controller-id")
	provider.cli = mockCli

	// Mock the response for server list
	testhelper.Mux.HandleFunc("/servers/detail", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"servers": [
			{
				"id": "d9072956-1560-487c-97f2-18bdf65ec749",
				"name": "test-instance",
				"addresses": {
					"network": [
						{
							"OS-EXT-IPS:type": "fixed",
							"addr": "10.10.0.4"
						}
					]
				},
				"metadata": {
					"os_arch": "amd64",
					"os_type": "linux",
					"os_name": "ubuntu",
					"os_version": "20.04"
				},
				"tags": ["garm-controller-id=my-controller-id",
				"garm-pool-id=test-pool"],
				"status": "ACTIVE"
			}
		]
		}`)
	})

	expectedOutput := []params.ProviderInstance{
		{
			ProviderID: "d9072956-1560-487c-97f2-18bdf65ec749",
			Name:       "test-instance",
			OSArch:     "amd64",
			OSType:     "linux",
			Status:     "running",
			OSName:     "ubuntu",
			OSVersion:  "20.04",
			Addresses: []params.Address{
				{
					Type:    params.PrivateAddress,
					Address: "10.10.0.4",
				},
			},
		},
	}

	instances, err := provider.ListInstances(ctx, "test-pool")
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, instances)
}

func TestStart(t *testing.T) {
	ctx := context.Background()
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()
	provider := &openstackProvider{
		cfg: &config.Config{
			Cloud: "mycloud",
			Credentials: config.Credentials{
				Clouds: "../testdata/clouds.yaml",
			},
			DefaultNetworkID:     "test-network",
			AllowedImageOwners:   []string{"owner1", "owner2"},
			ImageVisibility:      "public",
			DisableUpdatesOnBoot: false,
			EnableBootDebug:      true,
		},
		controllerID: "my-controller-id",
	}
	serviceClient := thclient.ServiceClient()
	mockCli := client.NewTestOpenStackClient(serviceClient, "my-controller-id")
	provider.cli = mockCli

	// Mock the response for server list
	testhelper.Mux.HandleFunc("/servers/detail", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"servers": [
			{
				"id": "d9072956-1560-487c-97f2-18bdf65ec749",
				"name": "test-instance",
				"addresses": {
					"network": [
						{
							"OS-EXT-IPS:type": "fixed",
							"addr": "10.10.0.4"
						}
					]
				},
				"metadata": {
					"os_arch": "amd64",
					"os_type": "linux",
					"os_name": "ubuntu",
					"os_version": "20.04"
				},
				"tags": ["garm-controller-id=my-controller-id",
				"garm-pool-id=test-pool"],
				"status": "SHUTOFF"
			}
		]
		}`)
	})

	// Mock the response for server stop
	testhelper.Mux.HandleFunc("/servers/d9072956-1560-487c-97f2-18bdf65ec749/action", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "POST")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `
		{
		"server": {
			"id": "d9072956-1560-487c-97f2-18bdf65ec749",
			"name": "test-instance",
			"addresses": {
				"network": [
					{
						"OS-EXT-IPS:type": "fixed",
						"addr": "10.10.0.4",
						"version": 4
					}
				]
			},
			"metadata": {
				"os_arch": "amd64",
				"os_type": "linux",
				"os_name": "ubuntu",
				"os_version": "20.04"
			},
			"tags": ["garm-controller-id=my-controller-id"],
			"status": "ACTIVE"
		}
		}`)
	})

	err := provider.Start(ctx, "test-instance")
	assert.NoError(t, err)
}
