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
	"testing"

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
