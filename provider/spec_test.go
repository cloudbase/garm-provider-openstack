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
	"testing"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	"github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-openstack/config"
	"github.com/stretchr/testify/assert"
)

func Test_machineSpec_MergeExtraSpecs(t *testing.T) {
	tests := []struct {
		name                      string
		configImageVisibility     string
		extraSpecsImageVisibility string
		wantVisibility            string
	}{
		{
			name:                      "only config",
			configImageVisibility:     "public",
			extraSpecsImageVisibility: "",
			wantVisibility:            "public",
		},
		{
			name:                      "only extra_specs",
			configImageVisibility:     "",
			extraSpecsImageVisibility: "all",
			wantVisibility:            "all",
		},
		{
			name:                      "defaults",
			configImageVisibility:     "",
			extraSpecsImageVisibility: "",
			wantVisibility:            "",
		},
		{
			name:                      "overwrite",
			configImageVisibility:     "shared",
			extraSpecsImageVisibility: "community",
			wantVisibility:            "community",
		},
		{
			name:                      "invalid extra_specs, won't overwrite config",
			configImageVisibility:     "shared",
			extraSpecsImageVisibility: "invalid",
			wantVisibility:            "shared",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &machineSpec{
				ImageVisibility: tt.configImageVisibility,
			}
			extraSpecs := extraSpecs{
				ImageVisibility: tt.extraSpecsImageVisibility,
			}
			m.MergeExtraSpecs(extraSpecs)
			assert.Equal(t, tt.wantVisibility, m.ImageVisibility)
		})
	}
}

func TestExtraSpecsFromBootstrapParams(t *testing.T) {
	tests := []struct {
		name      string
		input     params.BootstrapInstance
		wantSpec  extraSpecs
		errString string
	}{
		{
			name: "full specs",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"security_groups": ["allow_ssh", "allow_web"],
					"allowed_image_owners": ["123456"],
					"image_visibility": "all",
					"network_id": "542b68dd-4b3d-459d-8531-34d5e779d4d6",
					"storage_backend": "cinder_nvme",
					"boot_from_volume": true,
					"boot_disk_size": 150,
					"use_config_drive": false,
					"enable_boot_debug": true,
					"disable_updates": true,
					"extra_packages": ["package1", "package2"],
					"runner_install_template": "IyEvYmluL2Jhc2gKZWNobyBJbnN0YWxsaW5nIHJ1bm5lci4uLg==",
					"pre_install_scripts": {"setup.sh": "IyEvYmluL2Jhc2gKZWNobyBTZXR1cCBzY3JpcHQuLi4="},
					"extra_context": {"key": "value"}
				}`),
			},
			wantSpec: extraSpecs{
				SecurityGroups:     []string{"allow_ssh", "allow_web"},
				AllowedImageOwners: []string{"123456"},
				ImageVisibility:    "all",
				NetworkID:          "542b68dd-4b3d-459d-8531-34d5e779d4d6",
				StorageBackend:     "cinder_nvme",
				BootFromVolume:     Ptr(true),
				BootDiskSize:       Ptr(int64(150)),
				UseConfigDrive:     Ptr(false),
				EnableBootDebug:    Ptr(true),
				DisableUpdates:     Ptr(true),
				ExtraPackages:      []string{"package1", "package2"},
				CloudConfigSpec: cloudconfig.CloudConfigSpec{
					RunnerInstallTemplate: []byte("#!/bin/bash\necho Installing runner..."),
					PreInstallScripts: map[string][]byte{
						"setup.sh": []byte("#!/bin/bash\necho Setup script..."),
					},
					ExtraContext: map[string]string{"key": "value"},
				},
			},
			errString: "",
		},
		{
			name: "specs just with security groups",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"security_groups": ["allow_ssh", "allow_web"]
				}`),
			},
			wantSpec: extraSpecs{
				SecurityGroups: []string{"allow_ssh", "allow_web"},
			},
			errString: "",
		},
		{
			name: "specs just with allowed image owners",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"allowed_image_owners": ["123456"]
				}`),
			},
			wantSpec: extraSpecs{
				AllowedImageOwners: []string{"123456"},
			},
			errString: "",
		},
		{
			name: "specs just with image visibility",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"image_visibility": "all"
				}`),
			},
			wantSpec: extraSpecs{
				ImageVisibility: "all",
			},
			errString: "",
		},
		{
			name: "specs just with network ID",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"network_id": "542b68dd-4b3d-459d-8531-34d5e779d4d6"
				}`),
			},
			wantSpec: extraSpecs{
				NetworkID: "542b68dd-4b3d-459d-8531-34d5e779d4d6",
			},
			errString: "",
		},
		{
			name: "specs just with storage backend",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"storage_backend": "cinder_nvme"
				}`),
			},
			wantSpec: extraSpecs{
				StorageBackend: "cinder_nvme",
			},
			errString: "",
		},
		{
			name: "specs just with boot from volume",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"boot_from_volume": true
				}`),
			},
			wantSpec: extraSpecs{
				BootFromVolume: Ptr(true),
			},
			errString: "",
		},
		{
			name: "specs just with boot disk size",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"boot_disk_size": 150
				}`),
			},
			wantSpec: extraSpecs{
				BootDiskSize: Ptr(int64(150)),
			},
			errString: "",
		},
		{
			name: "specs just with use config drive",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"use_config_drive": false
				}`),
			},
			wantSpec: extraSpecs{
				UseConfigDrive: Ptr(false),
			},
			errString: "",
		},
		{
			name: "specs just with enable boot debug",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"enable_boot_debug": true
				}`),
			},
			wantSpec: extraSpecs{
				EnableBootDebug: Ptr(true),
			},
			errString: "",
		},
		{
			name: "specs just with disable updates",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"disable_updates": true
				}`),
			},
			wantSpec: extraSpecs{
				DisableUpdates: Ptr(true),
			},
			errString: "",
		},
		{
			name: "specs just with extra packages",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"extra_packages": ["package1", "package2"]
				}`),
			},
			wantSpec: extraSpecs{
				ExtraPackages: []string{"package1", "package2"},
			},
			errString: "",
		},
		{
			name: "specs just with runner install template",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"runner_install_template": "IyEvYmluL2Jhc2gKZWNobyBJbnN0YWxsaW5nIHJ1bm5lci4uLg=="
				}`),
			},
			wantSpec: extraSpecs{
				CloudConfigSpec: cloudconfig.CloudConfigSpec{
					RunnerInstallTemplate: []byte("#!/bin/bash\necho Installing runner..."),
				},
			},
			errString: "",
		},
		{
			name: "specs just with pre install scripts",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"pre_install_scripts": {"setup.sh": "IyEvYmluL2Jhc2gKZWNobyBTZXR1cCBzY3JpcHQuLi4="}
				}`),
			},
			wantSpec: extraSpecs{
				CloudConfigSpec: cloudconfig.CloudConfigSpec{
					PreInstallScripts: map[string][]byte{
						"setup.sh": []byte("#!/bin/bash\necho Setup script..."),
					},
				},
			},
			errString: "",
		},
		{
			name: "specs just with extra context",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"extra_context": {"key": "value"}
				}`),
			},
			wantSpec: extraSpecs{
				CloudConfigSpec: cloudconfig.CloudConfigSpec{
					ExtraContext: map[string]string{"key": "value"},
				},
			},
			errString: "",
		},
		{
			name: "empty specs",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{}`),
			},
			wantSpec:  extraSpecs{},
			errString: "",
		},
		{
			name: "invalid json",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"image_visibility":
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "failed to validate extra specs",
		},
		{
			name: "invalid input for security groups - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"security_groups": "allow_ssh"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "security_groups: Invalid type. Expected: array, given: string",
		},
		{
			name: "invalid input for allowed image owners - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"allowed_image_owners": "123456"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "allowed_image_owners: Invalid type. Expected: array, given: string",
		},
		{
			name: "invalid input for image visibility - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"image_visibility": 123456
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "image_visibility: Invalid type. Expected: string, given: integer",
		},
		{
			name: "invalid input for network ID - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"network_id": 123456
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "network_id: Invalid type. Expected: string, given: integer",
		},
		{
			name: "invalid input for storage backend - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"storage_backend": 123456
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "storage_backend: Invalid type. Expected: string, given: integer",
		},
		{
			name: "invalid input for boot from volume - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"boot_from_volume": "true"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "boot_from_volume: Invalid type. Expected: boolean, given: string",
		},
		{
			name: "invalid input for boot disk size - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"boot_disk_size": "150"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "boot_disk_size: Invalid type. Expected: integer, given: string",
		},
		{
			name: "invalid input for use config drive - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"use_config_drive": "false"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "use_config_drive: Invalid type. Expected: boolean, given: string",
		},
		{
			name: "invalid input for enable boot debug - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"enable_boot_debug": "true"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "enable_boot_debug: Invalid type. Expected: boolean, given: string",
		},
		{
			name: "invalid input for disable updates - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"disable_updates": "true"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "disable_updates: Invalid type. Expected: boolean, given: string",
		},
		{
			name: "invalid input for extra packages - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"extra_packages": "package1"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "extra_packages: Invalid type. Expected: array, given: string",
		},
		{
			name: "invalid input for runner install template - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"runner_install_template": 123456
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "runner_install_template: Invalid type. Expected: string, given: integer",
		},
		{
			name: "invalid input for pre install scripts - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"pre_install_scripts": "setup.sh"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "pre_install_scripts: Invalid type. Expected: object, given: string",
		},
		{
			name: "invalid input for extra context - wrong data type",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"extra_context": "key"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "extra_context: Invalid type. Expected: object, given: string",
		},
		{
			name: "invalid input - additional property",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"invalid": "property"
				}`),
			},
			wantSpec:  extraSpecs{},
			errString: "failed to validate extra specs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := extraSpecsFromBootstrapData(tt.input)
			assert.Equal(t, tt.wantSpec, spec)
			if tt.errString != "" {
				assert.ErrorContains(t, err, tt.errString)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewMachineSpec(t *testing.T) {
	config := &config.Config{
		Cloud: "mycloud",
		Credentials: config.Credentials{
			Clouds: "../testdata/clouds.yaml",
		},
		DefaultNetworkID:     "network",
		AllowedImageOwners:   []string{"owner1", "owner2"},
		ImageVisibility:      "public",
		DisableUpdatesOnBoot: false,
		EnableBootDebug:      true,
	}
	data := params.BootstrapInstance{
		Name:          "test-instance",
		InstanceToken: "test-token",
		OSArch:        params.Amd64,
		OSType:        params.Linux,
		Flavor:        "m1.small",
		Image:         "ubuntu-20.04",
		Tools: []params.RunnerApplicationDownload{
			{
				OS:                Ptr("linux"),
				Architecture:      Ptr("x64"),
				DownloadURL:       Ptr("http://test.com"),
				Filename:          Ptr("runner.tar.gz"),
				SHA256Checksum:    Ptr("sha256:1123"),
				TempDownloadToken: Ptr("test-token"),
			},
		},
		ExtraSpecs: json.RawMessage(`{
			"security_groups": ["allow_ssh", "allow_web"],
			"allowed_image_owners": ["123456"],
			"image_visibility": "all",
			"network_id": "542b68dd-4b3d-459d-8531-34d5e779d4d6",
			"storage_backend": "cinder_nvme",
			"boot_from_volume": true,
			"boot_disk_size": 150,
			"use_config_drive": false,
			"enable_boot_debug": false
		}`),
		PoolID: "test-pool",
	}
	DefaultToolFetch = func(osType params.OSType, osArch params.OSArch, tools []params.RunnerApplicationDownload) (params.RunnerApplicationDownload, error) {
		return data.Tools[0], nil
	}
	expectedOutput := &machineSpec{
		StorageBackend:     "cinder_nvme",
		SecurityGroups:     []string{"allow_ssh", "allow_web"},
		AllowedImageOwners: []string{"123456"},
		ImageVisibility:    "all",
		NetworkID:          "542b68dd-4b3d-459d-8531-34d5e779d4d6",
		BootFromVolume:     true,
		BootDiskSize:       int64(150),
		UseConfigDrive:     false,
		Flavor:             "m1.small",
		Image:              "ubuntu-20.04",
		Tools:              data.Tools[0],
		Tags:               []string{"garm-pool-id=test-pool", "garm-controller-id=controllerID"},
		BootstrapParams:    data,
		Properties: map[string]string{
			"os_arch":           "amd64",
			"os_type":           "linux",
			poolIDTagName:       "test-pool",
			controllerIDTagName: "controllerID",
		},
	}

	spec, err := NewMachineSpec(data, config, "controllerID")
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, spec)
}

func TestMachineSpecValidate(t *testing.T) {
	tests := []struct {
		name      string
		spec      *machineSpec
		errString string
	}{
		{
			name: "valid",
			spec: &machineSpec{
				StorageBackend:     "cinder_nvme",
				SecurityGroups:     []string{"allow_ssh", "allow_web"},
				AllowedImageOwners: []string{"123456"},
				ImageVisibility:    "all",
				NetworkID:          "542b68dd-4b3d-459d-8531-34d5e779d4d6",
				BootFromVolume:     true,
				BootDiskSize:       int64(150),
				UseConfigDrive:     false,
				Flavor:             "m1.small",
				Image:              "ubuntu-20.04",
				Tools: params.RunnerApplicationDownload{
					OS:                Ptr("linux"),
					Architecture:      Ptr("x64"),
					DownloadURL:       Ptr("http://test.com"),
					Filename:          Ptr("runner.tar.gz"),
					SHA256Checksum:    Ptr("sha256:1123"),
					TempDownloadToken: Ptr("test-token"),
				},
				Tags: []string{"garm-pool-id=test-pool", "garm-controller-id=controllerID"},
				BootstrapParams: params.BootstrapInstance{
					Name:          "test-instance",
					InstanceToken: "test-token",
					OSArch:        params.Amd64,
					OSType:        params.Linux,
					Flavor:        "m1.small",
					Image:         "ubuntu-20.04",
					Tools: []params.RunnerApplicationDownload{
						{
							OS:                Ptr("linux"),
							Architecture:      Ptr("x64"),
							DownloadURL:       Ptr("http://test.com"),
							Filename:          Ptr("runner.tar.gz"),
							SHA256Checksum:    Ptr("sha256:1123"),
							TempDownloadToken: Ptr("test-token"),
						},
					},
					ExtraSpecs: json.RawMessage(`{
						"security_groups": ["allow_ssh", "allow_web"],
						"allowed_image_owners": ["123456"],
						"image_visibility": "all",
						"network_id": "542b68dd-4b3d-459d-8531-34d5e779d4d6",
						"storage_backend": "cinder_nvme",
						"boot_from_volume": true,
						"boot_disk_size": 150,
						"use_config_drive": false,
						"enable_boot_debug": false
					}`),
					PoolID: "test-pool",
				},
				Properties: map[string]string{
					"os_arch":           "amd64",
					"os_type":           "linux",
					poolIDTagName:       "test-pool",
					controllerIDTagName: "controllerID",
				},
			},
			errString: "",
		},
		{
			name: "invalid NetworkID",
			spec: &machineSpec{
				NetworkID: "",
			},
			errString: "missing network ID",
		},
		{
			name: "boot from volume without boot disk size",
			spec: &machineSpec{
				NetworkID:      "542b68dd-4b3d-459d-8531-34d5e779d4d6",
				BootFromVolume: true,
				BootDiskSize:   int64(0),
			},
			errString: "boot from volume is enabled, and boot disk size is 0",
		},
		{
			name: "missing flavor",
			spec: &machineSpec{
				NetworkID:      "542b68dd-4b3d-459d-8531-34d5e779d4d6",
				BootFromVolume: false,
				Flavor:         "",
			},
			errString: "missing flavor",
		},
		{
			name: "missing image",
			spec: &machineSpec{
				NetworkID:      "542b68dd-4b3d-459d-8531-34d5e779d4d6",
				BootFromVolume: false,
				Flavor:         "m1.small",
				Image:          "",
			},
			errString: "missing image",
		},
		{
			name: "missing tags",
			spec: &machineSpec{
				NetworkID:      "542b68dd-4b3d-459d-8531-34d5e779d4d6",
				BootFromVolume: false,
				Flavor:         "m1.small",
				Image:          "ubuntu-20.04",
				Tags:           nil,
			},
			errString: "missing tags; at least the controller ID and pool ID must be set",
		},
		{
			name: "missing tools",
			spec: &machineSpec{
				NetworkID:      "542b68dd-4b3d-459d-8531-34d5e779d4d6",
				BootFromVolume: false,
				Flavor:         "m1.small",
				Image:          "ubuntu-20.04",
				Tags:           []string{"garm-pool-id=test-pool", "garm-controller-id=controllerID"},
				Tools:          params.RunnerApplicationDownload{},
			},
			errString: "missing tools",
		},
		{
			name: "missing bootstrap params",
			spec: &machineSpec{
				NetworkID:      "542b68dd-4b3d-459d-8531-34d5e779d4d6",
				BootFromVolume: false,
				Flavor:         "m1.small",
				Image:          "ubuntu-20.04",
				Tags:           []string{"garm-pool-id=test-pool", "garm-controller-id=controllerID"},
				Tools: params.RunnerApplicationDownload{
					OS:                Ptr("linux"),
					Architecture:      Ptr("x64"),
					DownloadURL:       Ptr("http://test.com"),
					Filename:          Ptr("runner.tar.gz"),
					SHA256Checksum:    Ptr("sha256:1123"),
					TempDownloadToken: Ptr("test-token"),
				},
				BootstrapParams: params.BootstrapInstance{},
			},
			errString: "missing bootstrap params",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if tt.errString == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errString)
			}
		})
	}
}
