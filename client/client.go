package client

import (
	"fmt"

	"github.com/cloudbase/garm-provider-openstack/config"
	"github.com/google/uuid"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/bootfromvolume"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/diskconfig"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/extendedstatus"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/images"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

func NewClient(cfg *config.Config) (*openstackClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate credentials: %w", err)
	}

	opts := clientconfig.ClientOpts{
		Cloud:    cfg.Cloud,
		YAMLOpts: &cfg.Credentials,
	}
	compute, err := clientconfig.NewServiceClient("compute", &opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get compute client: %w", err)
	}
	// Enables filter by tags, metadata property in VM list and boot from volume.
	compute.Microversion = "2.67"

	glance, err := clientconfig.NewServiceClient("image", &opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get glance client: %w", err)
	}

	neutron, err := clientconfig.NewServiceClient("network", &opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get neutron client: %w", err)
	}

	cinder, err := clientconfig.NewServiceClient("volume", &opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get cinder client: %w", err)
	}
	return &openstackClient{
		compute: compute,
		image:   glance,
		network: neutron,
		volume:  cinder,
	}, nil
}

type ServerWithExt struct {
	servers.Server
	availabilityzones.ServerAvailabilityZoneExt
	extendedstatus.ServerExtendedStatusExt
	diskconfig.ServerDiskConfigExt
}

type openstackClient struct {
	compute *gophercloud.ServiceClient
	image   *gophercloud.ServiceClient
	network *gophercloud.ServiceClient
	volume  *gophercloud.ServiceClient
}

// CreateServerFromImage creates a new server from an image.
func (o *openstackClient) CreateServerFromImage(createOpts servers.CreateOpts) (srv ServerWithExt, err error) {
	defer func() {
		if err != nil {
			if srv.ID != "" {
				o.DeleteServer(srv.ID, true)
			} else {
				o.DeleteServer(createOpts.Name, true)
			}
		}
	}()

	if err = servers.Create(o.compute, createOpts).ExtractInto(&srv); err != nil {
		return srv, fmt.Errorf("failed to create server: %w", err)
	}

	if err := servers.WaitForStatus(o.compute, srv.ID, "ACTIVE", 120); err != nil {
		return srv, fmt.Errorf("server did not reach ACTIVE state after 120 seconds")
	}

	return o.GetServer(srv.ID)
}

// CreateServerFromVolume creates a new server from a volume.
func (o *openstackClient) CreateServerFromVolume(createOpts bootfromvolume.CreateOptsExt, name string) (srv ServerWithExt, err error) {
	defer func() {
		if err != nil {
			if srv.ID != "" {
				o.DeleteServer(srv.ID, true)
			} else {
				o.DeleteServer(name, true)
			}
		}
	}()

	if err = bootfromvolume.Create(o.compute, createOpts).ExtractInto(&srv); err != nil {
		return srv, fmt.Errorf("failed to create server: %w", err)
	}

	if err := servers.WaitForStatus(o.compute, srv.ID, "ACTIVE", 120); err != nil {
		return srv, fmt.Errorf("server did not reach ACTIVE state after 120 seconds")
	}

	return o.GetServer(srv.ID)
}

// GetServer creates a new server.
func (o *openstackClient) GetServer(nameOrId string) (ServerWithExt, error) {
	if isUUID(nameOrId) {
		var srv ServerWithExt
		if err := servers.Get(o.compute, nameOrId).ExtractInto(&srv); err != nil {
			return ServerWithExt{}, fmt.Errorf("failed to get server: %w", err)
		}
		return srv, nil
	}

	tags := []string{
		"instance-name=" + nameOrId,
	}

	srvResults, err := o.ListServers(tags)
	if err != nil {
		return ServerWithExt{}, fmt.Errorf("failed to find server by name: %w", err)
	}

	if len(srvResults) == 0 {
		return ServerWithExt{}, fmt.Errorf("failed to find server with name or id %s", nameOrId)
	}

	if len(srvResults) > 1 {
		return ServerWithExt{}, fmt.Errorf("multiple servers with name %s were found", nameOrId)
	}

	return srvResults[0], nil
}

// ListServers creates a new server.
func (o *openstackClient) ListServers(tags []string) ([]ServerWithExt, error) {
	var srvResults []ServerWithExt
	pages, err := servers.List(o.compute, nil).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	err = servers.ExtractServersInto(pages, &srvResults)
	if err != nil {
		return nil, fmt.Errorf("failed to extract server info: %w", err)
	}

	return srvResults, nil
}

func (o *openstackClient) waitForStatus(id, status string, secs int) error {
	return gophercloud.WaitFor(secs, func() (bool, error) {
		current, err := servers.Get(o.compute, id).Extract()
		if err != nil {
			return false, err
		}

		if current.Status == status {
			return true, nil
		}

		if current.Status == "ERROR" {
			return false, fmt.Errorf("instance in ERROR state")
		}

		return false, nil
	})
}

func (o *openstackClient) deleteServerByID(id string, waitForDelete bool) error {
	response := servers.ForceDelete(o.compute, id)
	if response.StatusCode == 404 {
		return nil
	}

	if err := response.ExtractErr(); err != nil {
		return err
	}

	if waitForDelete {
		if err := o.waitForStatus(id, "DELETED", 120); err != nil {
			return fmt.Errorf("failed to delete server: %w", err)
		}
	}

	return nil
}

func (o *openstackClient) deleteServerByName(name string, waitForDelete bool) error {
	tags := []string{
		"instance-name=" + name,
	}
	results, err := o.ListServers(tags)
	if err != nil {
		return fmt.Errorf("failed while searching for server: %w", err)
	}
	if len(results) == 0 {
		return nil
	}

	for _, srv := range results {
		if err := o.deleteServerByID(srv.ID, waitForDelete); err != nil {
			return fmt.Errorf("failed to delete server %s (ID: %s): %w", srv.Name, srv.ID, err)
		}
	}

	return nil
}

// DeleteServer server creates a new server.
func (o *openstackClient) DeleteServer(nameOrID string, waitForDelete bool) error {
	if isUUID(nameOrID) {
		return o.deleteServerByID(nameOrID, waitForDelete)
	}
	return o.deleteServerByName(nameOrID, waitForDelete)
}

// GetFlavor resolves a flavor name or ID to a flavor.
func (o *openstackClient) GetFlavor(nameOrId string) (*flavors.Flavor, error) {
	var flavor *flavors.Flavor
	var err error
	flavor, err = flavors.Get(o.compute, nameOrId).Extract()
	if err == nil {
		return flavor, nil
	}

	if err := flavors.ListDetail(o.compute, nil).EachPage(func(page pagination.Page) (bool, error) {
		flavorResults, err := flavors.ExtractFlavors(page)
		if err != nil {
			return false, fmt.Errorf("failed to extract flavors: %w", err)
		}

		for _, res := range flavorResults {
			if res.ID == nameOrId || res.Name == nameOrId {
				// return the first one we find.
				flavor = &res
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list flavors: %w", err)
	}

	if flavor == nil {
		return nil, fmt.Errorf("failed to find flavor with name or id %s", nameOrId)
	}

	return flavor, nil
}

// GetImage gets details of an image passed in by ID.
func (o *openstackClient) GetImage(nameOrID string) (*images.Image, error) {
	var result *images.Image
	var err error

	if isUUID(nameOrID) {
		result, err = images.Get(o.image, nameOrID).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to find image: %w", err)
		}
		return result, nil
	}

	// perhaps it's a name. List all images and look for the image by name.
	if err := images.ListDetail(o.image, nil).EachPage(func(page pagination.Page) (bool, error) {
		imgResults, err := images.ExtractImages(page)
		if err != nil {
			return false, err
		}
		for _, img := range imgResults {
			if img.ID == nameOrID || img.Name == nameOrID {
				// return the first one we find.
				result = &img
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to get image with name or id %s: %w", nameOrID, err)
	}

	if result == nil {
		return nil, fmt.Errorf("failed to find image with name or id %s", nameOrID)
	}

	return result, nil
}

// GetNetwork returns network details
func (o *openstackClient) GetNetwork(nameOrID string) (*networks.Network, error) {
	var net *networks.Network
	var err error

	if isUUID(nameOrID) {
		net, err = networks.Get(o.network, nameOrID).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to get network: %w", err)
		}
		return net, nil
	}

	if err := networks.List(o.network, nil).EachPage(func(page pagination.Page) (bool, error) {
		netResults, err := networks.ExtractNetworks(page)
		if err != nil {
			return false, fmt.Errorf("failed to extract networks: %w", err)
		}

		for _, network := range netResults {
			if network.ID == nameOrID || network.Name == nameOrID {
				// return the first one we find.
				net = &network
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	if net == nil {
		return nil, fmt.Errorf("failed to find network with name or id %s", nameOrID)
	}

	return net, nil
}

func isUUID(data string) bool {
	if _, err := uuid.Parse(data); err == nil {
		return true
	}

	return false
}
