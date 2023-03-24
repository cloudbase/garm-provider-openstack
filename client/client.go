package client

import (
	"fmt"

	"github.com/cloudbase/garm-provider-openstack/config"
	"github.com/gophercloud/gophercloud"
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
	// Enables filter by tags and metadata property in VM list.
	compute.Microversion = "2.52"

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

type openstackClient struct {
	compute *gophercloud.ServiceClient
	image   *gophercloud.ServiceClient
	network *gophercloud.ServiceClient
	volume  *gophercloud.ServiceClient
}

// CreateVolume creates a new volume.
func (o *openstackClient) CreateVolume() {}

// DeleteVolume removes a volume.
func (o *openstackClient) DeleteVolume() {}

// CreateServer creates a new server.
func (o *openstackClient) CreateServer() {}

// DeleteServer server creates a new server.
func (o *openstackClient) DeleteServer() {}

// GetFlavor resolves a flavor name or ID to a flavor.
func (o *openstackClient) GetFlavor() {}

// GetImage gets details of an image passed in by ID.
func (o *openstackClient) GetImage() {}

// GetNetwork returns network details
func (o *openstackClient) GetNetwork() {}
