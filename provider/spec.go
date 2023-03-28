package provider

import (
	"encoding/json"
	"fmt"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util"
	"github.com/google/go-github/v48/github"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/bootfromvolume"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"

	"github.com/cloudbase/garm-provider-openstack/config"
)

var (
	defaultBootDiskSize int64 = 50
)

type extraSpecs struct {
	SecurityGroups []string `json:"security_groups,omitempty"`
	NetworkID      string   `json:"network_id"`
	StorageBackend string   `json:"storage_backend,omitempty"`
	BootFromVolume *bool    `json:"boot_from_volume,omitempty"`
	BootDiskSize   *int64   `json:"boot_disk_size,omitempty"`
	UseConfigDrive *bool    `json:"use_config_drive"`
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

func getTags(controllerID, poolID, name string) []string {
	return []string{
		fmt.Sprintf("%s=%s", poolIDTagName, poolID),
		fmt.Sprintf("%s=%s", controllerIDTagName, controllerID),
	}
}

func getProperties(data params.BootstrapInstance, controllerID string) map[string]string {
	ret := map[string]string{
		"os_arch":           string(data.OSArch),
		"os_type":           string(data.OSType),
		poolIDTagName:       data.PoolID,
		controllerIDTagName: controllerID,
	}

	return ret
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

	spec := &machineSpec{
		StorageBackend:  cfg.DefaultStorageBackend,
		SecurityGroups:  cfg.DefaultSecurityGroups,
		NetworkID:       cfg.DefaultNetworkID,
		BootFromVolume:  cfg.BootFromVolume,
		BootDiskSize:    bootDiskSize,
		UseConfigDrive:  cfg.UseConfigDrive,
		Flavor:          data.Flavor,
		Image:           data.Image,
		Tools:           tools,
		Tags:            getTags(controllerID, data.PoolID, data.Name),
		BootstrapParams: data,
		Properties:      getProperties(data, controllerID),
	}
	spec.MergeExtraSpecs(extraSpec)

	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate spec: %w", err)
	}

	return spec, nil
}

type machineSpec struct {
	StorageBackend  string
	SecurityGroups  []string
	NetworkID       string
	BootFromVolume  bool
	BootDiskSize    int64
	UseConfigDrive  bool
	Flavor          string
	Image           string
	Tools           github.RunnerApplicationDownload
	Tags            []string
	Properties      map[string]string
	BootstrapParams params.BootstrapInstance
}

func (m *machineSpec) Validate() error {
	if m.NetworkID == "" {
		return fmt.Errorf("missing network ID")
	}

	if m.BootFromVolume {
		if m.BootDiskSize == 0 {
			return fmt.Errorf("boot from volume is enabled, and boot disk size is 0")
		}
	}

	if m.Flavor == "" {
		return fmt.Errorf("missing flavor")
	}

	if m.Image == "" {
		return fmt.Errorf("missing image")
	}

	if len(m.Tags) == 0 {
		return fmt.Errorf("missing tags; at least the controller ID and pool ID must be set")
	}

	if m.Tools.DownloadURL == nil {
		return fmt.Errorf("missing tools")
	}

	if m.BootstrapParams.Name == "" {
		return fmt.Errorf("missing bootstrap params")
	}
	return nil
}

// SetSpecFromImage looks for aditional info in the image metadata that can be set
// on a machine for later retrieval.
func (m *machineSpec) SetSpecFromImage(img images.Image) {
	if os_name, ok := img.Properties["os_distro"]; ok {
		val, ok := os_name.(string)
		if ok {
			m.Properties["os_name"] = val
		}
	}

	if os_version, ok := img.Properties["os_version"]; ok {
		val, ok := os_version.(string)
		if ok {
			m.Properties["os_version"] = val
		}
	}
}

func (m *machineSpec) MergeExtraSpecs(spec extraSpecs) {
	if spec.StorageBackend != "" {
		m.StorageBackend = spec.StorageBackend
	}

	if spec.BootDiskSize != nil {
		m.BootDiskSize = *spec.BootDiskSize
	}

	if spec.BootFromVolume != nil {
		m.BootFromVolume = *spec.BootFromVolume
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

func (m *machineSpec) ComposeUserData() ([]byte, error) {
	switch m.BootstrapParams.OSType {
	case params.Linux, params.Windows:
		udata, err := util.GetCloudConfig(m.BootstrapParams, m.Tools, m.BootstrapParams.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to generate userdata: %w", err)
		}
		return []byte(udata), nil
	}
	return nil, fmt.Errorf("unsupported OS type for cloud config: %s", m.BootstrapParams.OSType)
}

func (m *machineSpec) GetServerCreateOpts(flavor flavors.Flavor, net networks.Network, img images.Image) (servers.CreateOpts, error) {
	udata, err := m.ComposeUserData()
	if err != nil {
		return servers.CreateOpts{}, fmt.Errorf("failed to get user data: %w", err)
	}
	return servers.CreateOpts{
		Name:           m.BootstrapParams.Name,
		ImageRef:       img.ID,
		FlavorRef:      flavor.ID,
		SecurityGroups: m.SecurityGroups,
		Networks: []servers.Network{
			{
				UUID: net.ID,
			},
		},
		Metadata:    m.Properties,
		ConfigDrive: &m.UseConfigDrive,
		Tags:        m.Tags,
		UserData:    udata,
	}, nil
}

func (m *machineSpec) GetBootFromVolumeOpts(srvOpts servers.CreateOpts) (bootfromvolume.CreateOptsExt, error) {
	rootDisk := bootfromvolume.BlockDevice{
		DeleteOnTermination: true,
		DestinationType:     bootfromvolume.DestinationVolume,
		SourceType:          bootfromvolume.SourceImage,
		UUID:                srvOpts.ImageRef,
		VolumeSize:          int(m.BootDiskSize),
	}
	if m.StorageBackend != "" {
		rootDisk.VolumeType = m.StorageBackend
	}
	blockDevices := []bootfromvolume.BlockDevice{
		rootDisk,
	}
	return bootfromvolume.CreateOptsExt{
		CreateOptsBuilder: srvOpts,
		BlockDevice:       blockDevices,
	}, nil
}
