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

package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"gopkg.in/yaml.v2"
)

// NewConfig returns a new Config
func NewConfig(cfgFile string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(cfgFile, &config); err != nil {
		return nil, fmt.Errorf("error decoding config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}
	return &config, nil
}

type Config struct {
	// Cloud is the name of the cloud that should be used. The cloud
	// must be defined in the supplied clouds.yaml file supplied in the
	// credentials field.
	//
	// This option can NOT be overwritten using extra_specs.
	Cloud string `toml:"cloud"`

	// Credentials holds information needed to connect to a cloud.
	//
	// This option can NOT be overwritten using extra_specs.
	Credentials Credentials `toml:"credentials"`

	// DefaultStorageBackend holds the name of the default storage backend
	// to use. If this is is empty, we will default to whatever is the default
	// in the cloud.
	//
	// This option can be overwritten using extra_specs.
	DefaultStorageBackend string `toml:"default_storage_backend"`

	// DefaultSecurityGroups holds a list of security group IDs that will be
	// added by default to runners.
	//
	// This option can be overwritten using extra_specs.
	DefaultSecurityGroups []string `toml:"default_security_groups"`

	// DefaultNetworkID is the default network ID to use when creating a new runner.
	//
	// This value is mandatory.
	// This value can be overwritten by extra_specs.
	DefaultNetworkID string `toml:"network_id"`

	// BootFromVolume indicates whether or not to boot from a cinder volume.
	//
	// This value can be overwritten using extra_specs.
	BootFromVolume bool `toml:"boot_from_volume"`

	// BootDiskSize is used in when BootFromVolume is set to true. If not explicitly
	// set, we use the root disk size defined in the flavor. If no root disk size is
	// specified in the flavor, we default to 50 GB.
	//
	// This value can be overwritten using extra_specs.
	// This option is ignored if BootFromVolume is set to false.
	BootDiskSize *int64 `toml:"root_disk_size"`

	// UseConfigDrive indicates whether to use config drive or not.
	//
	// This value can be overwritten using extra_specs.
	UseConfigDrive bool `toml:"use_config_drive"`

	// AllowedImageOwners is a list of image owners that are allowed to be used.
	// If this is empty, all images are allowed.
	// If not empty, only images owned by the specified owners are allowed.
	//
	// This value can be overwritten using extra_specs.
	AllowedImageOwners []string `toml:"allowed_image_owners"`

	// DisableUdatesOnBoot indicates whether to install or update packages on boot during cloud-init.
	// If set to true `PackageUpgrade` is set to false and `Packages` is set to an empty list in the cloud-init config.
	//
	// This value can NOT be overwritten using extra_specs.
	DisableUpdatesOnBoot bool `toml:"disable_updates_on_boot"`

	// EnableBootDebug indicates whether to enable debug mode during boot / cloud-init.
	// If set to true `set -x` is added to the cloud-init config.
	// Attention: This will might expose sensitive data in the logs! Do not use in production!
	//
	// This value can be overwritten using extra_specs.
	EnableBootDebug bool `toml:"enable_boot_debug"`
}

func (c *Config) Validate() error {
	if err := c.Credentials.Validate(); err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}
	if !c.Credentials.HasCloud(c.Cloud) {
		return fmt.Errorf("cloud %s is not defined in clouds.yaml", c.Cloud)
	}

	if c.DefaultNetworkID == "" {
		return fmt.Errorf("missing network_id")
	}
	return nil
}

// Credentials holds the paths on disk to the following files:
//   - clouds.yaml
//   - clouds-public.yaml
//   - secure.yaml
//
// Out of the 3, the only one that is mandatory is clouds.yaml.
type Credentials struct {
	// Clouds holds the path to the clouds.yaml file. This field is mandatory
	// and holds information on how to connect to one or more OpenStack clouds.
	Clouds string `toml:"clouds"`
	// PublicClouds is the path on disk to clouds-public.yaml. See:
	// https://docs.openstack.org/python-openstackclient/latest/configuration/index.html#clouds-public-yaml
	PublicClouds string `toml:"public_clouds"`
	// SecureClouds is the path on disk to secure.yaml. This file normally holds secrets
	// for connecting to the cloud. The format is identical to clouds.yaml, with only
	// sensitive fields fileld in. These fields are merged with the values in clouds.yaml.
	// See: https://docs.openstack.org/os-client-config/latest/user/configuration.html#splitting-secrets
	SecureClouds string `toml:"secure_clouds"`
}

func (c Credentials) HasCloud(name string) bool {
	clouds, err := c.LoadCloudsYAML()
	if err != nil {
		return false
	}
	if _, ok := clouds[name]; !ok {
		return false
	}
	return true
}

func (c Credentials) Validate() error {
	if _, err := c.LoadCloudsYAML(); err != nil {
		return fmt.Errorf("failed to load clouds.yaml: %w", err)
	}
	if _, err := c.LoadPublicCloudsYAML(); err != nil {
		return fmt.Errorf("failed to load clouds-public.yaml: %w", err)
	}
	if _, err := c.LoadSecureCloudsYAML(); err != nil {
		return fmt.Errorf("failed to load secure.yaml: %w", err)
	}
	return nil
}

func readFile(filePath string) (map[string]clientconfig.Cloud, error) {
	if filePath == "" {
		return nil, fmt.Errorf("missing clouds config")
	}
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("failed to access clouds config: %w", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read clouds config: %w", err)
	}
	var clouds clientconfig.Clouds
	err = yaml.Unmarshal(content, &clouds)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %w", err)
	}
	return clouds.Clouds, nil
}

func canAccess(filePath string) bool {
	if filePath == "" {
		return false
	}
	if _, err := os.Stat(filePath); err != nil {
		return false
	}
	return true
}

func (o *Credentials) LoadCloudsYAML() (map[string]clientconfig.Cloud, error) {
	return readFile(o.Clouds)
}

func (o *Credentials) LoadSecureCloudsYAML() (map[string]clientconfig.Cloud, error) {
	if !canAccess(o.SecureClouds) {
		return map[string]clientconfig.Cloud{}, nil
	}
	return readFile(o.SecureClouds)
}

func (o *Credentials) LoadPublicCloudsYAML() (map[string]clientconfig.Cloud, error) {
	if !canAccess(o.PublicClouds) {
		return map[string]clientconfig.Cloud{}, nil
	}
	return readFile(o.PublicClouds)
}
