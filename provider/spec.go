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
	"encoding/json"
	"fmt"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	"github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/bootfromvolume"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/xeipuuv/gojsonschema"

	"github.com/cloudbase/garm-provider-openstack/config"
)

var defaultBootDiskSize int64 = 50

type ToolFetchFunc func(osType params.OSType, osArch params.OSArch, tools []params.RunnerApplicationDownload) (params.RunnerApplicationDownload, error)

type GetCloudConfigFunc func(bootstrapParams params.BootstrapInstance, tools params.RunnerApplicationDownload, runnerName string) (string, error)

var (
	DefaultToolFetch      ToolFetchFunc      = util.GetTools
	DefaultGetCloudconfig GetCloudConfigFunc = cloudconfig.GetCloudConfig
)

const jsonSchema string = `

{
    "$schema": "http://cloudbase.it/garm-provider-openstack/schemas/extra_specs#",
    "type": "object",
    "description": "Schema defining supported extra specs for the Garm OpenStack Provider",
    "properties": {
        "security_groups": {
            "type": "array",
            "items": {
                "type": "string"
            }
        },
        "network_id": {
            "type": "string",
            "description": "The tenant network to which runners will be connected to."
        },
        "storage_backend": {
            "type": "string",
            "description": "The cinder backend to use when creating volumes."
        },
        "boot_from_volume": {
            "type": "boolean",
            "description": "Whether to boot from volume or not. Use this option if the root disk size defined by the flavor is not enough."
        },
        "boot_disk_size": {
            "type": "integer",
            "description": "The size of the root disk in GB. Default is 50 GB."
        },
        "use_config_drive": {
            "type": "boolean",
            "description": "Use config drive."
        },
        "enable_boot_debug": {
            "type": "boolean",
            "description": "Enable cloud-init debug mode. Adds 'set -x' into the cloud-init script."
        },
        "allowed_image_owners": {
            "type": "array",
            "items": {
                "type": "string"
            },
            "description": "A list of image owners to allow when creating the instance. If not specified, all images will be allowed." 
        },
		"image_visibility": {
			"type": "string",
			"description": "The visibility of the image to use."
		},
        "disable_updates": {
            "type": "boolean",
            "description": "Disable automatic updates on the VM."
        },
        "extra_packages": {
            "type": "array",
            "description": "Extra packages to install on the VM.",
            "items": {
                "type": "string"
            }
        },
        "runner_install_template": {
            "type": "string",
            "description": "This option can be used to override the default runner install template. If used, the caller is responsible for the correctness of the template as well as the suitability of the template for the target OS. Use the extra_context extra spec if your template has variables in it that need to be expanded."
        },
        "extra_context": {
            "type": "object",
            "description": "Extra context that will be passed to the runner_install_template.",
            "additionalProperties": {
                "type": "string"
            }
        },
        "pre_install_scripts": {
            "type": "object",
            "description": "A map of pre-install scripts that will be run before the runner install script. These will run as root and can be used to prep a generic image before we attempt to install the runner. The key of the map is the name of the script as it will be written to disk. The value is a byte array with the contents of the script.",
            "additionalProperties": {
                "type": "string"
            }
        }
    },
	"additionalProperties": false
}
`

type extraSpecs struct {
	SecurityGroups     []string `json:"security_groups,omitempty"`
	AllowedImageOwners []string `json:"allowed_image_owners,omitempty"`
	ImageVisibility    string   `json:"image_visibility,omitempty"`
	NetworkID          string   `json:"network_id"`
	StorageBackend     string   `json:"storage_backend,omitempty"`
	BootFromVolume     *bool    `json:"boot_from_volume,omitempty"`
	BootDiskSize       *int64   `json:"boot_disk_size,omitempty"`
	UseConfigDrive     *bool    `json:"use_config_drive"`
	EnableBootDebug    *bool    `json:"enable_boot_debug"`
	DisableUpdates     *bool    `json:"disable_updates"`
	ExtraPackages      []string `json:"extra_packages"`
}

func jsonSchemaValidation(schema json.RawMessage) error {
	schemaLoader := gojsonschema.NewStringLoader(jsonSchema)
	extraSpecsLoader := gojsonschema.NewBytesLoader(schema)
	result, err := gojsonschema.Validate(schemaLoader, extraSpecsLoader)
	if err != nil {
		return fmt.Errorf("failed to validate schema: %w", err)
	}
	if !result.Valid() {
		return fmt.Errorf("schema validation failed: %s", result.Errors())
	}
	return nil
}

func extraSpecsFromBootstrapData(data params.BootstrapInstance) (extraSpecs, error) {
	if len(data.ExtraSpecs) == 0 {
		return extraSpecs{}, nil
	}

	var spec extraSpecs
	if err := jsonSchemaValidation(data.ExtraSpecs); err != nil {
		return extraSpecs{}, fmt.Errorf("failed to validate extra specs: %w", err)
	}
	if err := json.Unmarshal(data.ExtraSpecs, &spec); err != nil {
		return extraSpecs{}, fmt.Errorf("failed to unmarshal extra_specs: %w", err)
	}

	return spec, nil
}

func getTags(controllerID, poolID string) []string {
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

	tools, err := DefaultToolFetch(data.OSType, data.OSArch, data.Tools)
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

	if cfg.DisableUpdatesOnBoot {
		data.UserDataOptions.DisableUpdatesOnBoot = true
	}

	if cfg.EnableBootDebug {
		data.UserDataOptions.EnableBootDebug = true
	}

	spec := &machineSpec{
		StorageBackend:     cfg.DefaultStorageBackend,
		SecurityGroups:     cfg.DefaultSecurityGroups,
		AllowedImageOwners: cfg.AllowedImageOwners,
		ImageVisibility:    cfg.ImageVisibility,
		NetworkID:          cfg.DefaultNetworkID,
		BootFromVolume:     cfg.BootFromVolume,
		BootDiskSize:       bootDiskSize,
		UseConfigDrive:     cfg.UseConfigDrive,
		Flavor:             data.Flavor,
		Image:              data.Image,
		Tools:              tools,
		Tags:               getTags(controllerID, data.PoolID),
		BootstrapParams:    data,
		Properties:         getProperties(data, controllerID),
		ExtraPackages:      extraSpec.ExtraPackages,
	}
	spec.MergeExtraSpecs(extraSpec)

	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate spec: %w", err)
	}

	return spec, nil
}

type machineSpec struct {
	StorageBackend     string
	SecurityGroups     []string
	AllowedImageOwners []string
	ImageVisibility    string
	NetworkID          string
	BootFromVolume     bool
	BootDiskSize       int64
	UseConfigDrive     bool
	Flavor             string
	Image              string
	DisableUpdates     bool
	ExtraPackages      []string
	Tools              params.RunnerApplicationDownload
	Tags               []string
	Properties         map[string]string
	BootstrapParams    params.BootstrapInstance
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

	if spec.AllowedImageOwners != nil {
		m.AllowedImageOwners = spec.AllowedImageOwners
	}

	if spec.EnableBootDebug != nil {
		m.BootstrapParams.UserDataOptions.EnableBootDebug = *spec.EnableBootDebug
	}

	if spec.DisableUpdates != nil {
		m.DisableUpdates = *spec.DisableUpdates
	}

	// an empty visibility in the extra specs should not override the
	// the config's visibility
	if config.IsValidVisibility(spec.ImageVisibility) {
		m.ImageVisibility = spec.ImageVisibility
	}
}

func (m *machineSpec) ComposeUserData() ([]byte, error) {
	bootstrapParams := m.BootstrapParams
	bootstrapParams.UserDataOptions.DisableUpdatesOnBoot = m.DisableUpdates
	bootstrapParams.UserDataOptions.ExtraPackages = m.ExtraPackages
	bootstrapParams.UserDataOptions.EnableBootDebug = m.BootstrapParams.UserDataOptions.EnableBootDebug
	switch m.BootstrapParams.OSType {
	case params.Linux, params.Windows:
		udata, err := cloudconfig.GetCloudConfig(bootstrapParams, m.Tools, bootstrapParams.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to generate userdata: %w", err)
		}
		return []byte(udata), nil
	}
	return nil, fmt.Errorf("unsupported OS type for cloud config: %s", bootstrapParams.OSType)
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

func Ptr[T any](v T) *T {
	return &v
}
