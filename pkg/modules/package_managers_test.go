package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test Homebrew Module
func TestHomebrewModuleValidation(t *testing.T) {
	module := NewHomebrewModule()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid package install",
			args: map[string]interface{}{
				"name":  "wget",
				"state": "present",
			},
			wantErr: false,
		},
		{
			name: "valid multiple packages",
			args: map[string]interface{}{
				"names": []interface{}{"git", "curl"},
				"state": "present",
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			args: map[string]interface{}{
				"name":  "wget",
				"state": "invalid",
			},
			wantErr: true,
		},
		{
			name:    "no action specified",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "valid update homebrew",
			args: map[string]interface{}{
				"update_homebrew": true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test APT Module
func TestAptModuleValidation(t *testing.T) {
	module := NewAptModule()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid package install",
			args: map[string]interface{}{
				"name":  "nginx",
				"state": "present",
			},
			wantErr: false,
		},
		{
			name: "valid update cache",
			args: map[string]interface{}{
				"update_cache": true,
			},
			wantErr: false,
		},
		{
			name: "valid upgrade",
			args: map[string]interface{}{
				"upgrade": "dist",
			},
			wantErr: false,
		},
		{
			name: "invalid upgrade option",
			args: map[string]interface{}{
				"upgrade": "invalid",
			},
			wantErr: true,
		},
		{
			name: "build-dep state",
			args: map[string]interface{}{
				"name":  "python3",
				"state": "build-dep",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test YUM Module
func TestYumModuleValidation(t *testing.T) {
	module := NewYumModule()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid package install",
			args: map[string]interface{}{
				"name":  "httpd",
				"state": "present",
			},
			wantErr: false,
		},
		{
			name: "installed state (alias for present)",
			args: map[string]interface{}{
				"name":  "httpd",
				"state": "installed",
			},
			wantErr: false,
		},
		{
			name: "security updates",
			args: map[string]interface{}{
				"security": true,
			},
			wantErr: false,
		},
		{
			name: "enable repo",
			args: map[string]interface{}{
				"name":       "package",
				"enablerepo": "epel",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test DNF Module
func TestDnfModuleValidation(t *testing.T) {
	module := NewDnfModule()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid package install",
			args: map[string]interface{}{
				"name":  "nginx",
				"state": "present",
			},
			wantErr: false,
		},
		{
			name: "allowerasing option",
			args: map[string]interface{}{
				"name":         "package",
				"allowerasing": true,
			},
			wantErr: false,
		},
		{
			name: "nobest option",
			args: map[string]interface{}{
				"name":   "package",
				"nobest": true,
			},
			wantErr: false,
		},
		{
			name: "autoremove",
			args: map[string]interface{}{
				"autoremove": true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test contains helper function
func TestContainsHelper(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		str   string
		want  bool
	}{
		{
			name:  "string exists",
			slice: []string{"present", "absent", "latest"},
			str:   "present",
			want:  true,
		},
		{
			name:  "string not exists",
			slice: []string{"present", "absent", "latest"},
			str:   "invalid",
			want:  false,
		},
		{
			name:  "empty slice",
			slice: []string{},
			str:   "anything",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.str)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test package manager command building
func TestHomebrewModule_BuildCommand(t *testing.T) {
	module := NewHomebrewModule()

	tests := []struct {
		name           string
		pkg            string
		state          string
		cask           bool
		installOptions string
		want           string
	}{
		{
			name:  "install package",
			pkg:   "wget",
			state: "present",
			want:  "brew install wget",
		},
		{
			name:  "install cask",
			pkg:   "docker",
			state: "present",
			cask:  true,
			want:  "brew install --cask docker",
		},
		{
			name:  "remove package",
			pkg:   "wget",
			state: "absent",
			want:  "brew uninstall wget",
		},
		{
			name:  "upgrade package",
			pkg:   "wget",
			state: "latest",
			want:  "brew upgrade wget",
		},
		{
			name:  "link package",
			pkg:   "wget",
			state: "linked",
			want:  "brew link wget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := module.buildCommand(tt.pkg, tt.state, tt.cask, tt.installOptions)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAptModule_BuildCommand(t *testing.T) {
	module := NewAptModule()

	tests := []struct {
		name   string
		pkg    string
		state  string
		aptCmd string
		want   string
	}{
		{
			name:   "install package",
			pkg:    "nginx",
			state:  "present",
			aptCmd: "apt",
			want:   "DEBIAN_FRONTEND=noninteractive apt install -y nginx",
		},
		{
			name:   "remove package",
			pkg:    "nginx",
			state:  "absent",
			aptCmd: "apt",
			want:   "DEBIAN_FRONTEND=noninteractive apt remove -y nginx",
		},
		{
			name:   "build-dep",
			pkg:    "python3",
			state:  "build-dep",
			aptCmd: "apt-get",
			want:   "DEBIAN_FRONTEND=noninteractive apt-get build-dep -y python3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := module.buildAptCommand(tt.pkg, tt.state, tt.aptCmd)
			assert.Equal(t, tt.want, got)
		})
	}
}