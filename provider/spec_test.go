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

	"github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-openstack/config"
	"github.com/stretchr/testify/assert"
)

func TestJsonSchemaValidation(t *testing.T) {
	tests := []struct {
		name      string
		input     json.RawMessage
		errString string
	}{
		{
			name: "valid",
			input: json.RawMessage(`{
				"boot_from_volume": true,
				"security_groups": ["allow_ssh", "allow_web"],
				"network_id": "542b68dd-4b3d-459d-8531-34d5e779d4d6",
				"storage_backend": "cinder_nvme",
				"boot_disk_size": 150,
				"use_config_drive": false,
				"enable_boot_debug": true,
				"allowed_image_owners": ["123456"],
				"image_visibility": "all"
			}`),
			errString: "",
		},
		{
			name: "invalid input - wrong data type",
			input: json.RawMessage(`{
				"boot_from_volume": "true"
			}`),
			errString: "schema validation failed: [boot_from_volume: Invalid type. Expected: boolean, given: string]",
		},
		{
			name: "invalid input - extra field",
			input: json.RawMessage(`{
				"boot_from_volume": true,
				"extra_field": "extra"
			}`),
			errString: "Additional property extra_field is not allowed",
		},
		{
			name:      "valid input - empty extra specs",
			input:     json.RawMessage(`{}`),
			errString: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jsonSchemaValidation(tt.input)
			if tt.errString == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errString)
			}
		})
	}
}

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
			name: "valid",
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
					"enable_boot_debug": true
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
			},
			errString: "",
		},
		{
			name: "invalid",
			input: params.BootstrapInstance{
				ExtraSpecs: json.RawMessage(`{
					"image_visibility": "invalid",
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
