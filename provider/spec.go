package provider

import (
	"encoding/json"
	"fmt"

	"github.com/cloudbase/garm-provider-openstack/config"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util"
	"github.com/google/go-github/v48/github"
)

var (
	defaultBootDiskSize int64 = 50
)

type extraSpecs struct {
	StorageBackend     string   `json:"storage_backend,omitempty"`
	SecurityGroups     []string `json:"security_groups,omitempty"`
	NetworkID          string   `json:"network_id"`
	FloatingIPNetwork  string   `json:"floating_ip_network"`
	AllocateFloatingIP *bool    `json:"allocate_floating_ip,omitempty"`
	BootFromVolume     *bool    `json:"boot_from_volume,omitempty"`
	BootDiskSize       *int64   `json:"boot_disk_size,omitempty"`
	UseConfigDrive     *bool    `json:"use_config_drive"`
}

func extraSpecsFromBootstrapData(data params.BootstrapInstance) (extraSpecs, error) {
	if len(data.ExtraSpecs) == 0 {
		return extraSpecs{}, nil
	}

	var spec extraSpecs
	if err := json.Unmarshal(data.ExtraSpecs, &spec); err != nil {
		return extraSpecs{}, fmt.Errorf("failed to unmarshal extra_specs")
	}

	return spec, nil
}

func getTags(controllerID, poolID string) []string {
	return []string{
		fmt.Sprintf("%s=%s", poolIDTagName, poolID),
		fmt.Sprintf("%s=%s", controllerIDTagName, controllerID),
	}
}

func NewMachineSpec(data params.BootstrapInstance, cfg *config.Config, controllerID string) (*machineSpec, error) {
	if cfg == nil {
		return nil, fmt.Errorf("invalid config")
	}

	tools, err := util.GetTools(data.OSType, data.OSArch, data.Tools)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools: %s", err)
	}

	extraSpec, err := extraSpecsFromBootstrapData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to get extra specs: %w", err)
	}

	bootDiskSize := defaultBootDiskSize
	if cfg.BootDiskSize != nil {
		bootDiskSize = *cfg.BootDiskSize
	}

	tags := getTags(controllerID, data.PoolID)

	spec := &machineSpec{
		StorageBackend:     cfg.DefaultStorageBackend,
		SecurityGroups:     cfg.DefaultSecurityGroups,
		NetworkID:          cfg.DefaultNetworkID,
		AllocateFloatingIP: cfg.AllocateFloatingIP,
		FloatingIPNetwork:  cfg.FloatingIPNetwork,
		BootFromVolume:     cfg.BootFromVolume,
		BootDiskSize:       bootDiskSize,
		UseConfigDrive:     cfg.UseConfigDrive,
		Flavor:             data.Flavor,
		Tools:              tools,
		Tags:               tags,
		BootstrapParams:    data,
	}
	spec.MergeExtraSpecs(extraSpec)

	return spec, nil
}

type machineSpec struct {
	StorageBackend     string
	SecurityGroups     []string
	NetworkID          string
	FloatingIPNetwork  string
	AllocateFloatingIP bool
	BootFromVolume     bool
	BootDiskSize       int64
	UseConfigDrive     bool
	Flavor             string
	Tools              github.RunnerApplicationDownload
	Tags               []string
	Properties         map[string]string
	BootstrapParams    params.BootstrapInstance
}

func (m *machineSpec) MergeExtraSpecs(spec extraSpecs) {
	if spec.StorageBackend != "" {
		m.StorageBackend = spec.StorageBackend
	}

	if spec.AllocateFloatingIP != nil {
		m.AllocateFloatingIP = *spec.AllocateFloatingIP
	}

	if spec.BootDiskSize != nil {
		m.BootDiskSize = *spec.BootDiskSize
	}

	if spec.BootFromVolume != nil {
		m.BootFromVolume = *spec.BootFromVolume
	}

	if spec.FloatingIPNetwork != "" {
		m.FloatingIPNetwork = spec.FloatingIPNetwork
	}

	if spec.NetworkID != "" {
		m.NetworkID = spec.NetworkID
	}

	if len(spec.SecurityGroups) > 0 {
		m.SecurityGroups = spec.SecurityGroups
	}

	if spec.UseConfigDrive != nil {
		m.UseConfigDrive = *spec.UseConfigDrive
	}
}
