package modules

import (
	"testing"
)

func TestIptablesModule(t *testing.T) {
	t.Run("ModuleProperties", func(t *testing.T) {
		m := NewIPTablesModule()
		if m.Name() != "iptables" {
			t.Errorf("Expected module name 'iptables', got %s", m.Name())
		}
	})

	t.Run("ValidationTests", func(t *testing.T) {
		m := NewIPTablesModule()
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name: "ValidAllowSSH",
				args: map[string]interface{}{
					"chain":    "INPUT",
					"protocol": "tcp",
					"dport":    "22",
					"jump":     "ACCEPT",
					"state":    "present",
				},
				wantErr: false,
			},
			{
				name: "ValidDropRule",
				args: map[string]interface{}{
					"chain":  "INPUT",
					"source": "192.168.1.100",
					"jump":   "DROP",
					"state":  "present",
				},
				wantErr: false,
			},
			{
				name: "ValidForwardRule",
				args: map[string]interface{}{
					"chain":     "FORWARD",
					"in_iface":  "eth0",
					"out_iface": "eth1",
					"jump":      "ACCEPT",
					"state":     "present",
				},
				wantErr: false,
			},
			{
				name: "MissingChain",
				args: map[string]interface{}{
					"protocol": "tcp",
					"dport":    "80",
					"jump":     "ACCEPT",
					"state":    "present",
				},
				wantErr: true,
			},
			{
				name: "ValidCustomChain",
				args: map[string]interface{}{
					"chain": "CUSTOM_CHAIN",
					"jump":  "ACCEPT",
					"state": "present",
				},
				wantErr: false,
			},
			{
				name: "ValidCustomJump",
				args: map[string]interface{}{
					"chain": "INPUT",
					"jump":  "CUSTOM_TARGET",
					"state": "present",
				},
				wantErr: false,
			},
			{
				name: "InvalidState",
				args: map[string]interface{}{
					"chain": "INPUT",
					"jump":  "ACCEPT",
					"state": "invalid",
				},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := m.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
}