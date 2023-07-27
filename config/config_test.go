package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCredentials_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid visibility",
			config: &Config{
				Cloud: "mycloud",
				Credentials: Credentials{
					Clouds: "../testdata/clouds.yaml",
				},
				DefaultNetworkID:     "network",
				AllowedImageOwners:   []string{"owner1", "owner2"},
				ImageVisibility:      "public",
				DisableUpdatesOnBoot: false,
				EnableBootDebug:      true,
			},
			wantErr: false,
		},
		{
			name: "invalid visibility",
			config: &Config{
				Cloud: "mycloud",
				Credentials: Credentials{
					Clouds: "../testdata/clouds.yaml",
				},
				DefaultNetworkID:     "network",
				AllowedImageOwners:   []string{"owner1", "owner2"},
				ImageVisibility:      "invalid",
				DisableUpdatesOnBoot: false,
				EnableBootDebug:      true,
			},
			wantErr: true,
		},
		{
			name: "missing clouds.yaml",
			config: &Config{
				DefaultNetworkID:     "",
				AllowedImageOwners:   []string{"owner1", "owner2"},
				ImageVisibility:      "invalidvisibility",
				DisableUpdatesOnBoot: true,
				EnableBootDebug:      true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				assert.Error(t, tt.config.Validate())
			} else {
				assert.NoError(t, tt.config.Validate())
			}
		})
	}
}

func TestIsValidVisibility(t *testing.T) {
	tests := []struct {
		visibility string
		want       bool
	}{
		{
			visibility: "public",
			want:       true,
		},
		{
			visibility: "private",
			want:       true,
		},
		{
			visibility: "shared",
			want:       true,
		},
		{
			visibility: "community",
			want:       true,
		},
		{
			visibility: "all",
			want:       true,
		},
		{
			visibility: "invalid",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.visibility, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidVisibility(tt.visibility))
		})
	}
}
