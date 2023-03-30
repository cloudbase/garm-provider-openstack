package client

import (
	"fmt"
	"strings"

	gErrors "errors"

	"github.com/cloudbase/garm-provider-openstack/config"
	"github.com/google/uuid"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/bootfromvolume"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/diskconfig"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/extendedstatus"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/startstop"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

const (
	controllerIDTagName = "garm-controller-id"
	poolIDTagName       = "garm-pool-id"
)

func NewClient(cfg *config.Config, controllerID string) (*OpenstackClient, error) {
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
	return &OpenstackClient{
		compute:      compute,
		image:        glance,
		network:      neutron,
		volume:       cinder,
		controllerID: controllerID,
	}, nil
}

type ServerWithExt struct {
	servers.Server
	availabilityzones.ServerAvailabilityZoneExt
	extendedstatus.ServerExtendedStatusExt
	diskconfig.ServerDiskConfigExt
}

type OpenstackClient struct {
	compute *gophercloud.ServiceClient
	image   *gophercloud.ServiceClient
	network *gophercloud.ServiceClient
	volume  *gophercloud.ServiceClient

	controllerID string
}

// CreateServerFromImage creates a new server from an image.
func (o *OpenstackClient) CreateServerFromImage(createOpts servers.CreateOpts) (srv ServerWithExt, err error) {
	defer func() {
		if err != nil {
			if srv.ID != "" {
				_ = o.DeleteServer(srv.ID, true)
			} else {
				_ = o.DeleteServer(createOpts.Name, true)
			}
		}
	}()

	if err = servers.Create(o.compute, createOpts).ExtractInto(&srv); err != nil {
		return srv, fmt.Errorf("failed to create server: %w", err)
	}

	if err := o.waitForStatus(srv.ID, "ACTIVE", 120); err != nil {
		return srv, fmt.Errorf("server did not reach ACTIVE state after 120 seconds: %w", err)
	}

	return o.GetServer(srv.ID)
}

// CreateServerFromVolume creates a new server from a volume.
func (o *OpenstackClient) CreateServerFromVolume(createOpts bootfromvolume.CreateOptsExt, name string) (srv ServerWithExt, err error) {
	defer func() {
		if err != nil {
			if srv.ID != "" {
				_ = o.DeleteServer(srv.ID, true)
			} else {
				_ = o.DeleteServer(name, true)
			}
		}
	}()

	if err = bootfromvolume.Create(o.compute, createOpts).ExtractInto(&srv); err != nil {
		return srv, fmt.Errorf("failed to create server: %w", err)
	}

	if err := o.waitForStatus(srv.ID, "ACTIVE", 120); err != nil {
		return srv, fmt.Errorf("server did not reach ACTIVE state after 120 seconds: %w", err)
	}

	return o.GetServer(srv.ID)
}

// GetServer creates a new server.
func (o *OpenstackClient) GetServer(nameOrId string) (ServerWithExt, error) {
	results, err := o.ListServersWithNameOrID(nameOrId)
	if err != nil {
		return ServerWithExt{}, fmt.Errorf("failed to find server: %w", err)
	}

	if len(results) == 0 {
		return ServerWithExt{}, fmt.Errorf("failed to find server with name or id %s", nameOrId)
	}

	if len(results) > 1 {
		return ServerWithExt{}, fmt.Errorf("multiple servers with name or id %s; manual intervention required", nameOrId)
	}

	return results[0], nil
}

func (o *OpenstackClient) ListServersWithTags(tags []string) ([]ServerWithExt, error) {
	var srvResults []ServerWithExt
	opts := servers.ListOpts{
		Tags: strings.Join(tags, ","),
	}
	pages, err := servers.List(o.compute, opts).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	err = servers.ExtractServersInto(pages, &srvResults)
	if err != nil {
		return nil, fmt.Errorf("failed to extract server info: %w", err)
	}

	return srvResults, nil
}

// ListServersWithNameOrID will return an array of servers that match a name or ID. When passing
// in an ID, there is no chance that this function will return an array larger than one element.
// When passing in a name, the function may return an array larger than 1 element.
func (o *OpenstackClient) ListServersWithNameOrID(nameOrId string) ([]ServerWithExt, error) {
	if isUUID(nameOrId) {
		var srv ServerWithExt
		if err := servers.Get(o.compute, nameOrId).ExtractInto(&srv); err != nil {
			return nil, fmt.Errorf("failed to get server: %w", err)
		}
		var controllerIDValue string
		if srv.Tags != nil {
			for _, tag := range *srv.Tags {
				if strings.HasPrefix(tag, controllerIDTagName+"=") {
					parts := strings.SplitN(tag, "=", 2)
					controllerIDValue = parts[1]
					break
				}
			}
		}
		if controllerIDValue != o.controllerID {
			return nil, fmt.Errorf("server with name or ID %s not found", nameOrId)
		}
		return []ServerWithExt{srv}, nil
	}

	tags := []string{
		controllerIDTagName + "=" + o.controllerID,
	}

	srvResults, err := o.ListServersWithTags(tags)
	if err != nil {
		return nil, fmt.Errorf("failed to find server by name: %w", err)
	}

	results := []ServerWithExt{}
	for _, result := range srvResults {
		if result.Name == nameOrId {
			results = append(results, result)
		}
	}

	return results, nil
}

// ListServers creates a new server.
func (o *OpenstackClient) ListServers(poolID string) ([]ServerWithExt, error) {
	tags := []string{
		poolIDTagName + "=" + poolID,
		controllerIDTagName + "=" + o.controllerID,
	}

	return o.ListServersWithTags(tags)
}

func (o *OpenstackClient) waitForStatus(id, status string, secs int) error {
	return gophercloud.WaitFor(secs, func() (bool, error) {
		result := servers.Get(o.compute, id)

		current, err := result.Extract()
		if err != nil {
			if _, ok := err.(gophercloud.ErrDefault404); ok && status == "DELETED" {
				return true, nil
			}
			return false, fmt.Errorf("could not find server %s: %w", id, err)
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

func (o *OpenstackClient) deleteServerByID(id string, waitForDelete bool) error {
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

// DeleteServer server deletes servers that match nameOrID.
// Warning: If a name is passed in, all servers with the same name, that match the controller ID
// set in the tags, will be deleted
func (o *OpenstackClient) DeleteServer(nameOrID string, waitForDelete bool) error {
	results, err := o.ListServersWithNameOrID(nameOrID)
	if err != nil {
		// errors returned by gophercloud are not errors.Is compatible.
		if _, ok := gErrors.Unwrap(err).(gophercloud.ErrDefault404); ok {
			return nil
		}
		return fmt.Errorf("failed to find server: %w", err)
	}
	for _, srv := range results {
		if err := o.deleteServerByID(srv.ID, true); err != nil {
			// errors returned by gophercloud are not errors.Is compatible.
			if _, ok := gErrors.Unwrap(err).(gophercloud.ErrDefault404); ok {
				continue
			}
			return fmt.Errorf("failed to delete server with ID %s: %w", srv.ID, err)
		}
	}
	return nil
}

// GetFlavor resolves a flavor name or ID to a flavor.
func (o *OpenstackClient) GetFlavor(nameOrId string) (*flavors.Flavor, error) {
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
func (o *OpenstackClient) GetImage(nameOrID string) (*images.Image, error) {
	var result *images.Image
	var err error

	if isUUID(nameOrID) {
		result, err = images.Get(o.image, nameOrID).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to find image: %w", err)
		}
		return result, nil
	}
	opts := images.ListOpts{
		Name: nameOrID,
	}
	// perhaps it's a name. List all images and look for the image by name.
	if err := images.List(o.image, opts).EachPage(func(page pagination.Page) (bool, error) {
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
func (o *OpenstackClient) GetNetwork(nameOrID string) (*networks.Network, error) {
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

func (o *OpenstackClient) StopServer(nameOrID string) error {
	srv, err := o.GetServer(nameOrID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	if srv.Status == "SHUTOFF" {
		return nil
	}

	if err := startstop.Stop(o.compute, srv.ID).ExtractErr(); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	return nil
}

func (o *OpenstackClient) StartServer(nameOrID string) error {
	srv, err := o.GetServer(nameOrID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	if srv.Status == "ACTIVE" {
		return nil
	}

	if err := startstop.Start(o.compute, srv.ID).ExtractErr(); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	return nil
}

func isUUID(data string) bool {
	if _, err := uuid.Parse(data); err == nil {
		return true
	}

	return false
}
