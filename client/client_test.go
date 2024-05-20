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

package client

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/bootfromvolume"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/testhelper"
	"github.com/gophercloud/gophercloud/testhelper/client"
	"github.com/stretchr/testify/assert"
)

func TestCreateServerFromImage(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for server creation
	testhelper.Mux.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "POST")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `
		{
		"server": {
			"id": "d9072956-1560-487c-97f2-18bdf65ec749",
			"name": "test-server",
			"status": "ACTIVE",
			"tags": ["garm-controller-id=my-controller-id"]
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
			"name": "test-server",
			"status": "ACTIVE",
			"tags": ["garm-controller-id=my-controller-id"]
		}
		}`)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}

	tags := []string{"garm-controller-id=my-controller-id"}
	createOpts := servers.CreateOpts{
		Name:      "test-server",
		ImageRef:  "aee1d242-730f-431f-88c1-87630c0f07ba",
		FlavorRef: "flavor-uuid",
		Tags:      tags,
	}

	expectedServer := ServerWithExt{
		Server: servers.Server{
			ID:     "d9072956-1560-487c-97f2-18bdf65ec749",
			Name:   "test-server",
			Status: "ACTIVE",
			Tags:   &tags,
		},
	}

	server, err := osClient.CreateServerFromImage(createOpts)

	assert.NoError(t, err)
	assert.Equal(t, server, expectedServer)
}

func TestCreateServerFromImageFailed(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for server creation
	testhelper.Mux.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "POST")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}

	tags := []string{"garm-controller-id=my-controller-id"}
	createOpts := servers.CreateOpts{
		Name:      "test-server",
		ImageRef:  "aee1d242-730f-431f-88c1-87630c0f07ba",
		FlavorRef: "flavor-uuid",
		Tags:      tags,
	}

	expectedServer := ServerWithExt{}

	server, err := osClient.CreateServerFromImage(createOpts)

	assert.ErrorContains(t, err, "failed to create server")
	assert.Equal(t, server, expectedServer)
}

func TestCreateServerFromVolume(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for server creation
	testhelper.Mux.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "POST")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `
		{
		"server": {
			"id": "d9072956-1560-487c-97f2-18bdf65ec749",
			"name": "test-server",
			"status": "ACTIVE",
			"tags": ["garm-controller-id=my-controller-id"]
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
			"name": "test-server",
			"status": "ACTIVE",
			"tags": ["garm-controller-id=my-controller-id"]
		}
		}`)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}
	createOpts := bootfromvolume.CreateOptsExt{
		CreateOptsBuilder: servers.CreateOpts{
			Name:      "test-server",
			FlavorRef: "flavor-uuid",
			ImageRef:  "aee1d242-730f-431f-88c1-87630c0f07ba",
		},
		BlockDevice: []bootfromvolume.BlockDevice{
			{
				BootIndex:           0,
				DeleteOnTermination: true,
				VolumeSize:          100,
				DeviceType:          "disk",
				DestinationType:     bootfromvolume.DestinationLocal,
				SourceType:          bootfromvolume.SourceImage,
				UUID:                "",
			},
		},
	}
	expectedServer := ServerWithExt{
		Server: servers.Server{
			ID:     "d9072956-1560-487c-97f2-18bdf65ec749",
			Name:   "test-server",
			Status: "ACTIVE",
			Tags:   &[]string{"garm-controller-id=my-controller-id"},
		},
	}

	server, err := osClient.CreateServerFromVolume(createOpts, "test-server")
	assert.NoError(t, err)
	assert.Equal(t, expectedServer, server)
}

func TestCreateServerFromVolumeFailed(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for server creation
	testhelper.Mux.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "POST")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `
		{
		"server": {
			"id": "d9072956-1560-487c-97f2-18bdf65ec749",
			"name": "test-server",
			"status": "ACTIVE",
			"tags": ["garm-controller-id=my-controller-id"]
		}
		}`)
	})

	// Mock the response for server get by ID
	testhelper.Mux.HandleFunc("/servers/d9072956-1560-487c-97f2-18bdf65ec749", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}
	createOpts := bootfromvolume.CreateOptsExt{
		CreateOptsBuilder: servers.CreateOpts{
			Name:      "test-server",
			FlavorRef: "flavor-uuid",
			ImageRef:  "aee1d242-730f-431f-88c1-87630c0f07ba",
		},
		BlockDevice: []bootfromvolume.BlockDevice{
			{
				BootIndex:           0,
				DeleteOnTermination: true,
				VolumeSize:          100,
				DeviceType:          "disk",
				DestinationType:     bootfromvolume.DestinationLocal,
				SourceType:          bootfromvolume.SourceImage,
				UUID:                "",
			},
		},
	}
	expectedServer := ServerWithExt{
		Server: servers.Server{
			ID:     "d9072956-1560-487c-97f2-18bdf65ec749",
			Name:   "test-server",
			Status: "ACTIVE",
			Tags:   &[]string{"garm-controller-id=my-controller-id"},
		},
	}

	server, err := osClient.CreateServerFromVolume(createOpts, "test-server")
	assert.ErrorContains(t, err, "server did not reach ACTIVE state after 120 seconds")
	assert.Equal(t, expectedServer, server)
}

func TestGetServer(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for server get by tags
	testhelper.Mux.HandleFunc("/servers/detail", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"servers": [
			{
				"id": "d9072956-1560-487c-97f2-18bdf65ec749",
				"name": "test-server",
				"status": "ACTIVE",
				"tags": ["garm-controller-id=my-controller-id"]
			}
		]
		}`)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}

	expectedServer := ServerWithExt{
		Server: servers.Server{
			ID:     "d9072956-1560-487c-97f2-18bdf65ec749",
			Name:   "test-server",
			Status: "ACTIVE",
			Tags:   &[]string{"garm-controller-id=my-controller-id"},
		},
	}

	server, err := osClient.GetServer("test-server")
	assert.NoError(t, err)
	assert.Equal(t, expectedServer, server)
}

func TestListServersWithTags(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

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
				"name": "test-server",
				"status": "ACTIVE",
				"tags": ["garm-controller-id=my-controller-id"]
			},
			{
				"id": "d9072956-1560-487c-10f2-18bdf65ec749",
				"name": "test-server-2",
				"status": "ACTIVE",
				"tags": ["garm-controller-id=my-controller-id"]
			}
		]
		}`)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}

	expectedServers := []ServerWithExt{
		{
			Server: servers.Server{
				ID:     "d9072956-1560-487c-97f2-18bdf65ec749",
				Name:   "test-server",
				Status: "ACTIVE",
				Tags:   &[]string{"garm-controller-id=my-controller-id"},
			},
		},
		{
			Server: servers.Server{
				ID:     "d9072956-1560-487c-10f2-18bdf65ec749",
				Name:   "test-server-2",
				Status: "ACTIVE",
				Tags:   &[]string{"garm-controller-id=my-controller-id"},
			},
		},
	}

	servers, err := osClient.ListServersWithTags([]string{"garm-controller-id=my-controller-id"})
	assert.NoError(t, err)
	assert.Equal(t, expectedServers, servers)
}

func TestListServers(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for server get by pool-id tags
	testhelper.Mux.HandleFunc("/servers/detail", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"servers": [
			{
				"id": "d9072956-1560-487c-97f2-18bdf65ec749",
				"name": "test-server",
				"status": "ACTIVE",
				"tags": ["garm-controller-id=my-controller-id",
				"garm-pool-id=my-pool-id"]
			},
			{
				"id": "d9072956-1560-487c-10f2-18bdf65ec749",
				"name": "test-server-2",
				"status": "ACTIVE",
				"tags": ["garm-controller-id=my-controller-id",
				"garm-pool-id=my-pool-id"]
			}
		]
		}`)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}

	expectedServer := []ServerWithExt{
		{
			Server: servers.Server{
				ID:     "d9072956-1560-487c-97f2-18bdf65ec749",
				Name:   "test-server",
				Status: "ACTIVE",
				Tags: &[]string{"garm-controller-id=my-controller-id",
					"garm-pool-id=my-pool-id"},
			},
		},
		{
			Server: servers.Server{
				ID:     "d9072956-1560-487c-10f2-18bdf65ec749",
				Name:   "test-server-2",
				Status: "ACTIVE",
				Tags: &[]string{"garm-controller-id=my-controller-id",
					"garm-pool-id=my-pool-id"},
			},
		},
	}

	server, err := osClient.ListServers("my-pool-id")

	assert.NoError(t, err)
	assert.Equal(t, expectedServer, server)
}

func TestDeleteServer(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

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

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}

	err := osClient.DeleteServer("d9072956-1560-487c-97f2-18bdf65ec749", true)
	assert.NoError(t, err)
}

func TestDeleteServerNotFound(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for server get by ID
	testhelper.Mux.HandleFunc("/servers/d9072956-1560-487c-97f2-18bdf65ec749", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}

	err := osClient.DeleteServer("d9072956-1560-487c-97f2-18bdf65ec749", true)
	assert.NoError(t, err)
}

func TestGetFlavorWithID(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for flavor get by ID
	testhelper.Mux.HandleFunc("/flavors/flavor-uuid", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"flavor": {
			"id": "flavor-uuid",
			"name": "test-flavor",
			"ram": 1024,
			"vcpus": 1,
			"disk": 10
		}
		}`)
	})

	osClient := &OpenstackClient{
		compute: client.ServiceClient(),
	}

	expectedFlavor := flavors.Flavor{
		ID:    "flavor-uuid",
		Name:  "test-flavor",
		RAM:   1024,
		VCPUs: 1,
		Disk:  10,
	}

	flavor, err := osClient.GetFlavor("flavor-uuid")
	assert.NoError(t, err)
	assert.Equal(t, expectedFlavor, *flavor)
}

func TestGetFlavorWithName(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for flavor list
	testhelper.Mux.HandleFunc("/flavors/detail", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"flavors": [
			{
				"id": "flavor-uuid",
				"name": "test-flavor",
				"ram": 1024,
				"vcpus": 1,
				"disk": 10
			}
		]
		}`)
	})

	osClient := &OpenstackClient{
		compute: client.ServiceClient(),
	}

	expectedFlavor := flavors.Flavor{
		ID:    "flavor-uuid",
		Name:  "test-flavor",
		RAM:   1024,
		VCPUs: 1,
		Disk:  10,
	}

	flavor, err := osClient.GetFlavor("test-flavor")
	assert.NoError(t, err)
	assert.Equal(t, expectedFlavor, *flavor)
}

func TestGetImageWithID(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for image get by ID
	testhelper.Mux.HandleFunc("/images/aee1d242-730f-431f-88c1-87630c0f07ba", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"name": "test-image",
		"id": "aee1d242-730f-431f-88c1-87630c0f07ba",
		"status": "ACTIVE"
		}`)
	})

	osClient := &OpenstackClient{
		image: client.ServiceClient(),
	}

	expectedImage := images.Image{
		ID:         "aee1d242-730f-431f-88c1-87630c0f07ba",
		Name:       "test-image",
		Properties: map[string]interface{}{},
		Status:     "ACTIVE",
	}

	image, err := osClient.GetImage("aee1d242-730f-431f-88c1-87630c0f07ba", "")
	assert.NoError(t, err)
	assert.Equal(t, expectedImage, *image)
}

func TestGetImageWithName(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for image list
	testhelper.Mux.HandleFunc("/images", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"images": [
			{
				"name": "test-image",
				"id": "aee1d242-730f-431f-88c1-87630c0f07ba",
				"status": "ACTIVE",
				"visibility": "public"
			}
		]
		}`)
	})

	osClient := &OpenstackClient{
		image: client.ServiceClient(),
	}

	expectedImage := images.Image{
		ID:         "aee1d242-730f-431f-88c1-87630c0f07ba",
		Name:       "test-image",
		Visibility: "public",
		Properties: map[string]interface{}{},
		Status:     "ACTIVE",
	}

	image, err := osClient.GetImage("test-image", "")
	assert.NoError(t, err)
	assert.Equal(t, expectedImage, *image)
}

func TestGetNetworkWithID(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for network get by ID
	testhelper.Mux.HandleFunc("/networks/aee1d242-730f-431f-88c1-87630c0f20ca", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"network": {
			"id": "aee1d242-730f-431f-88c1-87630c0f20ca",
			"name": "test-network",
			"status": "ACTIVE"
		}
		}`)
	})

	osClient := &OpenstackClient{
		network: client.ServiceClient(),
	}

	expectedNetwork := networks.Network{
		ID:     "aee1d242-730f-431f-88c1-87630c0f20ca",
		Name:   "test-network",
		Status: "ACTIVE",
	}

	network, err := osClient.GetNetwork("aee1d242-730f-431f-88c1-87630c0f20ca")
	assert.NoError(t, err)
	assert.Equal(t, expectedNetwork, *network)
}

func TestGetNetworkWithName(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for network list
	testhelper.Mux.HandleFunc("/networks", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `
		{
		"networks": [
			{
				"id": "aee1d242-730f-431f-88c1-87630c0f20ca",
				"name": "test-network",
				"status": "ACTIVE"
			}
		]
		}`)
	})

	osClient := &OpenstackClient{
		network: client.ServiceClient(),
	}

	expectedNetwork := networks.Network{
		ID:     "aee1d242-730f-431f-88c1-87630c0f20ca",
		Name:   "test-network",
		Status: "ACTIVE",
	}

	network, err := osClient.GetNetwork("test-network")
	assert.NoError(t, err)
	assert.Equal(t, expectedNetwork, *network)
}

func TestStopServer(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

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
			"status": "ACTIVE",
			"tags": ["garm-controller-id=my-controller-id"]
		}
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
			"name": "test-server",
			"status": "SHUTOFF",
			"tags": ["garm-controller-id=my-controller-id"]
		}
		}`)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}

	err := osClient.StopServer("d9072956-1560-487c-97f2-18bdf65ec749")
	assert.NoError(t, err)
}

func TestStopServerNotFound(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	// Mock the response for server get by ID
	testhelper.Mux.HandleFunc("/servers/d9072956-1560-487c-97f2-18bdf65ec749", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "GET")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}

	err := osClient.StopServer("d9072956-1560-487c-97f2-18bdf65ec749")
	assert.ErrorContains(t, err, "failed to get server")
}

func TestStartServer(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

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
			"status": "SHUTOFF",
			"tags": ["garm-controller-id=my-controller-id"]
		}
		}`)
	})

	// Mock the response for server start
	testhelper.Mux.HandleFunc("/servers/d9072956-1560-487c-97f2-18bdf65ec749/action", func(w http.ResponseWriter, r *http.Request) {
		testhelper.TestMethod(t, r, "POST")
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, `
		{
		"server": {
			"id": "d9072956-1560-487c-97f2-18bdf65ec749",
			"name": "test-server",
			"status": "ACTIVE",
			"tags": ["garm-controller-id=my-controller-id"]
		}
		}`)
	})

	osClient := &OpenstackClient{
		compute:      client.ServiceClient(),
		controllerID: "my-controller-id",
	}

	err := osClient.StartServer("d9072956-1560-487c-97f2-18bdf65ec749")
	assert.NoError(t, err)
}
