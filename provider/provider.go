package provider

import (
	"context"
	"fmt"

	"github.com/cloudbase/garm-provider-openstack/client"
	"github.com/cloudbase/garm-provider-openstack/config"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/providers/external/execution"
)

var _ execution.ExternalProvider = &openstackProvider{}

const (
	controllerIDTagName = "garm-controller-id"
	poolIDTagName       = "garm-pool-id"
	instanceNameTag     = "instance-name"
)

func NewOpenStackProvider(configPath, controllerID string) (execution.ExternalProvider, error) {
	conf, err := config.NewConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	cli, err := client.NewClient(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return &openstackProvider{
		cfg:          conf,
		controllerID: controllerID,
		cli:          cli,
	}, nil
}

type openstackProvider struct {
	cfg          *config.Config
	cli          *client.OpenstackClient
	controllerID string
}

func openstackServerToInstance(srv client.ServerWithExt) params.Instance {
	return params.Instance{
		ProviderID: srv.ID,
		Name:       srv.Name,
	}
}

// CreateInstance creates a new compute instance in the provider.
func (a *openstackProvider) CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) (params.Instance, error) {
	spec, err := NewMachineSpec(bootstrapParams, a.cfg, a.controllerID)
	if err != nil {
		return params.Instance{}, fmt.Errorf("failed to build machine spec: %w", err)
	}
	flavor, err := a.cli.GetFlavor(spec.Flavor)
	if err != nil {
		return params.Instance{}, fmt.Errorf("failed to resolve flavor %s: %w", bootstrapParams.Flavor, err)
	}

	net, err := a.cli.GetNetwork(spec.NetworkID)
	if err != nil {
		return params.Instance{}, fmt.Errorf("failed to resolve network %s: %w", spec.NetworkID, err)
	}

	image, err := a.cli.GetImage(spec.Image)
	if err != nil {
		return params.Instance{}, fmt.Errorf("failed to resolve image info: %w", err)
	}

	srvCreateOpts, err := spec.GetServerCreateOpts(*flavor, *net, *image)
	if err != nil {
		return params.Instance{}, fmt.Errorf("failed to get server create options: %w", err)
	}

	var ret params.Instance

	if !spec.BootFromVolume {
		srv, err := a.cli.CreateServerFromImage(srvCreateOpts)
		if err != nil {
			return params.Instance{}, fmt.Errorf("failed to create server: %w", err)
		}
		ret = openstackServerToInstance(srv)
	} else {
		createOption, err := spec.GetBootFromVolumeOpts(srvCreateOpts)
		if err != nil {
			return params.Instance{}, fmt.Errorf("failed to get boot from volume create options: %w", err)
		}
		srv, err := a.cli.CreateServerFromVolume(createOption, spec.BootstrapParams.Name)
		if err != nil {
			return params.Instance{}, fmt.Errorf("failed to create server: %w", err)
		}
		ret = openstackServerToInstance(srv)
	}
	return ret, nil
}

// Delete instance will delete the instance in a provider.
func (a *openstackProvider) DeleteInstance(ctx context.Context, instance string) error {
	return nil
}

// GetInstance will return details about one instance.
func (a *openstackProvider) GetInstance(ctx context.Context, instance string) (params.Instance, error) {
	return params.Instance{}, nil
}

// ListInstances will list all instances for a provider.
func (a *openstackProvider) ListInstances(ctx context.Context, poolID string) ([]params.Instance, error) {
	return nil, nil
}

// RemoveAllInstances will remove all instances created by this provider.
func (a *openstackProvider) RemoveAllInstances(ctx context.Context) error {
	return nil
}

// Stop shuts down the instance.
func (a *openstackProvider) Stop(ctx context.Context, instance string, force bool) error {
	return nil
}

// Start boots up an instance.
func (a *openstackProvider) Start(ctx context.Context, instance string) error {
	return nil
}
