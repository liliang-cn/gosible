package modules

import (
	"context"
	"testing"

	gotest "github.com/gosinble/gosinble/pkg/testing"
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
		
		testCases := []struct {
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
					"chain":       "FORWARD",
					"in_iface":    "eth0",
					"out_iface":   "eth1",
					"jump":        "ACCEPT",
					"state":       "present",
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
				name: "InvalidChain",
				args: map[string]interface{}{
					"chain": "INVALID",
					"jump":  "ACCEPT",
					"state": "present",
				},
				wantErr: true,
			},
			{
				name: "InvalidJump",
				args: map[string]interface{}{
					"chain": "INPUT",
					"jump":  "INVALID",
					"state": "present",
				},
				wantErr: true,
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
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := m.Validate(tc.args)
				if (err != nil) != tc.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("IptablesRuleTests", func(t *testing.T) {
		m := NewIPTablesModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("AddAllowSSHRule", func(t *testing.T) {
			// Check if rule exists
			conn.ExpectCommand("iptables -C INPUT -p tcp --dport 22 -j ACCEPT", &gotest.CommandResponse{
				ExitCode: 1, // Rule doesn't exist
			})
			
			// Add rule
			conn.ExpectCommand("iptables -A INPUT -p tcp --dport 22 -j ACCEPT", &gotest.CommandResponse{
				Stdout:   "",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"chain":    "INPUT",
				"protocol": "tcp",
				"dport":    "22",
				"jump":     "ACCEPT",
				"state":    "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("RuleAlreadyExists", func(t *testing.T) {
			conn.Reset()
			// Check if rule exists
			conn.ExpectCommand("iptables -C INPUT -p tcp --dport 22 -j ACCEPT", &gotest.CommandResponse{
				ExitCode: 0, // Rule exists
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"chain":    "INPUT",
				"protocol": "tcp",
				"dport":    "22",
				"jump":     "ACCEPT",
				"state":    "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertNotChanged(result)
			conn.Verify()
		})
		
		t.Run("AddDropRule", func(t *testing.T) {
			conn.Reset()
			// Check if rule exists
			conn.ExpectCommand("iptables -C INPUT -s 192.168.1.100 -j DROP", &gotest.CommandResponse{
				ExitCode: 1,
			})
			
			// Add rule
			conn.ExpectCommand("iptables -A INPUT -s 192.168.1.100 -j DROP", &gotest.CommandResponse{
				Stdout:   "",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"chain":  "INPUT",
				"source": "192.168.1.100",
				"jump":   "DROP",
				"state":  "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
	})

	t.Run("RemoveRuleTests", func(t *testing.T) {
		m := NewIPTablesModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("RemoveExistingRule", func(t *testing.T) {
			// Check if rule exists
			conn.ExpectCommand("iptables -C INPUT -p tcp --dport 80 -j ACCEPT", &gotest.CommandResponse{
				ExitCode: 0, // Rule exists
			})
			
			// Delete rule
			conn.ExpectCommand("iptables -D INPUT -p tcp --dport 80 -j ACCEPT", &gotest.CommandResponse{
				Stdout:   "",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"chain":    "INPUT",
				"protocol": "tcp",
				"dport":    "80",
				"jump":     "ACCEPT",
				"state":    "absent",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("RuleDoesNotExist", func(t *testing.T) {
			conn.Reset()
			// Check if rule exists
			conn.ExpectCommand("iptables -C INPUT -p tcp --dport 443 -j ACCEPT", &gotest.CommandResponse{
				ExitCode: 1, // Rule doesn't exist
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"chain":    "INPUT",
				"protocol": "tcp",
				"dport":    "443",
				"jump":     "ACCEPT",
				"state":    "absent",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertNotChanged(result)
			conn.Verify()
		})
	})

	t.Run("ComplexRuleTests", func(t *testing.T) {
		m := NewIPTablesModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("ForwardRule", func(t *testing.T) {
			// Check if rule exists
			conn.ExpectCommand("iptables -C FORWARD -i eth0 -o eth1 -j ACCEPT", &gotest.CommandResponse{
				ExitCode: 1,
			})
			
			// Add rule
			conn.ExpectCommand("iptables -A FORWARD -i eth0 -o eth1 -j ACCEPT", &gotest.CommandResponse{
				Stdout:   "",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"chain":     "FORWARD",
				"in_iface":  "eth0",
				"out_iface": "eth1",
				"jump":      "ACCEPT",
				"state":     "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("NATRule", func(t *testing.T) {
			conn.Reset()
			// Check if rule exists
			conn.ExpectCommand("iptables -t nat -C POSTROUTING -o eth0 -j MASQUERADE", &gotest.CommandResponse{
				ExitCode: 1,
			})
			
			// Add rule
			conn.ExpectCommand("iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE", &gotest.CommandResponse{
				Stdout:   "",
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"table":     "nat",
				"chain":     "POSTROUTING",
				"out_iface": "eth0",
				"jump":      "MASQUERADE",
				"state":     "present",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
	})

	t.Run("CheckModeTests", func(t *testing.T) {
		m := NewIPTablesModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		// Check if rule exists
		conn.ExpectCommand("iptables -C INPUT -p tcp --dport 22 -j ACCEPT", &gotest.CommandResponse{
			ExitCode: 1, // Rule doesn't exist
		})
		
		result, err := m.Run(ctx, conn, map[string]interface{}{
			"chain":       "INPUT",
			"protocol":    "tcp",
			"dport":       "22",
			"jump":        "ACCEPT",
			"state":       "present",
			"_check_mode": true,
		})
		
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		helper.AssertSuccess(result)
		helper.AssertChanged(result)
		helper.AssertSimulated(result)
		
		// Should only check, not add the rule
		if len(conn.GetCallOrder()) != 1 {
			t.Error("Should only check rule existence in check mode")
		}
	})

	t.Run("ErrorHandlingTests", func(t *testing.T) {
		m := NewIPTablesModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("AddRuleFailed", func(t *testing.T) {
			conn.Reset()
			// Check if rule exists
			conn.ExpectCommand("iptables -C INPUT -p tcp --dport 22 -j ACCEPT", &gotest.CommandResponse{
				ExitCode: 1,
			})
			
			// Add rule fails
			conn.ExpectCommand("iptables -A INPUT -p tcp --dport 22 -j ACCEPT", &gotest.CommandResponse{
				Stderr:   "iptables: Permission denied",
				ExitCode: 1,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"chain":    "INPUT",
				"protocol": "tcp",
				"dport":    "22",
				"jump":     "ACCEPT",
				"state":    "present",
			})
			
			if err == nil {
				t.Error("Expected error but got none")
			}
			if result != nil {
				helper.AssertFailure(result)
			}
		})
	})
}