package provider

import (
	"context"
	"fmt"

	"github.com/cloudbase/garm-provider-openstack/config"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/providers/external/execution"
)

var _ execution.ExternalProvider = &openstackProvider{}

const (
	controllerIDTagName = "garm-controller-id"
	poolIDTagName       = "garm-pool-id"
)

func NewOpenStackProvider(configPath, controllerID string) (execution.ExternalProvider, error) {
	conf, err := config.NewConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}
	return &openstackProvider{
		cfg:          conf,
		controllerID: controllerID,
	}, nil
}

type openstackProvider struct {
	cfg          *config.Config
	controllerID string
}

// CreateInstance creates a new compute instance in the provider.
func (a *openstackProvider) CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) (params.Instance, error) {
	return params.Instance{}, nil
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
