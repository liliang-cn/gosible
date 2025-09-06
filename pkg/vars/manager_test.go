package vars

import (
	"context"
	"fmt"
	"testing"

	"github.com/gosinble/gosinble/pkg/types"
	"github.com/gosinble/gosinble/pkg/connection"
)

func TestNewVarManager(t *testing.T) {
	vm := NewVarManager()
	if vm == nil {
		t.Fatal("NewVarManager returned nil")
	}

	if vm.variables == nil || vm.facts == nil {
		t.Error("VarManager maps not initialized")
	}
}

func TestVarManagerSetAndGetVar(t *testing.T) {
	vm := NewVarManager()

	// Test setting and getting string variable
	vm.SetVar("test_string", "hello world")
	if value, exists := vm.GetVar("test_string"); !exists {
		t.Error("variable should exist")
	} else if value != "hello world" {
		t.Errorf("expected 'hello world', got %v", value)
	}

	// Test setting and getting numeric variable
	vm.SetVar("test_number", 42)
	if value, exists := vm.GetVar("test_number"); !exists {
		t.Error("numeric variable should exist")
	} else if value != 42 {
		t.Errorf("expected 42, got %v", value)
	}

	// Test getting nonexistent variable
	if _, exists := vm.GetVar("nonexistent"); exists {
		t.Error("nonexistent variable should not exist")
	}
}

func TestVarManagerSetVars(t *testing.T) {
	vm := NewVarManager()

	vars := map[string]interface{}{
		"var1": "value1",
		"var2": 123,
		"var3": true,
	}

	vm.SetVars(vars)

	// Check all variables were set
	for key, expectedValue := range vars {
		if value, exists := vm.GetVar(key); !exists {
			t.Errorf("variable %s should exist", key)
		} else if value != expectedValue {
			t.Errorf("variable %s expected %v, got %v", key, expectedValue, value)
		}
	}
}

func TestVarManagerGetVars(t *testing.T) {
	vm := NewVarManager()

	// Set some variables
	vm.SetVar("var1", "value1")
	vm.SetVar("var2", 42)

	// Mock some facts
	vm.mu.Lock()
	vm.facts["fact1"] = "factvalue1"
	vm.facts["fact2"] = 100
	vm.mu.Unlock()

	allVars := vm.GetVars()

	// Check variables are present
	if allVars["var1"] != "value1" {
		t.Errorf("var1 expected 'value1', got %v", allVars["var1"])
	}
	if allVars["var2"] != 42 {
		t.Errorf("var2 expected 42, got %v", allVars["var2"])
	}

	// Check facts are present
	if allVars["fact1"] != "factvalue1" {
		t.Errorf("fact1 expected 'factvalue1', got %v", allVars["fact1"])
	}
	if allVars["fact2"] != 100 {
		t.Errorf("fact2 expected 100, got %v", allVars["fact2"])
	}

	// Variables should override facts with same name
	vm.SetVar("fact1", "overridden")
	allVars = vm.GetVars()
	if allVars["fact1"] != "overridden" {
		t.Errorf("fact1 should be overridden by variable, got %v", allVars["fact1"])
	}
}

func TestVarManagerGatherFacts(t *testing.T) {
	vm := NewVarManager()
	conn := connection.NewLocalConnection()
	ctx := context.Background()

	// Connect first
	info := types.ConnectionInfo{Type: "local", Host: "localhost"}
	if err := conn.Connect(ctx, info); err != nil {
		t.Fatalf("connection failed: %v", err)
	}
	defer conn.Close()

	facts, err := vm.GatherFacts(ctx, conn)
	if err != nil {
		t.Fatalf("GatherFacts failed: %v", err)
	}

	if len(facts) == 0 {
		t.Error("facts should not be empty")
	}

	// Check for some expected facts
	expectedFactPrefixes := []string{
		"ansible_hostname",
		"ansible_system",
		"ansible_kernel",
		"ansible_architecture",
	}

	for _, prefix := range expectedFactPrefixes {
		found := false
		for key := range facts {
			if key == prefix {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected fact with prefix %s not found", prefix)
		}
	}

	// Check that facts were stored in the manager
	if value, exists := vm.GetVar("ansible_hostname"); !exists {
		t.Error("ansible_hostname should be accessible via GetVar")
	} else if value == "" {
		t.Error("ansible_hostname should not be empty")
	}
}

func TestVarManagerMergeVars(t *testing.T) {
	vm := NewVarManager()

	base := map[string]interface{}{
		"var1": "base_value1",
		"var2": "base_value2",
		"nested": map[string]interface{}{
			"key1": "base_nested1",
			"key2": "base_nested2",
		},
	}

	override := map[string]interface{}{
		"var1": "override_value1", // Should override
		"var3": "override_value3", // Should be added
		"nested": map[string]interface{}{
			"key1": "override_nested1", // Should override
			"key3": "override_nested3", // Should be added
		},
	}

	result := vm.MergeVars(base, override)

	// Check overridden values
	if result["var1"] != "override_value1" {
		t.Errorf("var1 should be overridden, got %v", result["var1"])
	}

	// Check preserved values
	if result["var2"] != "base_value2" {
		t.Errorf("var2 should be preserved, got %v", result["var2"])
	}

	// Check added values
	if result["var3"] != "override_value3" {
		t.Errorf("var3 should be added, got %v", result["var3"])
	}

	// Check nested merging
	nested, ok := result["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("nested should be a map")
	}

	if nested["key1"] != "override_nested1" {
		t.Errorf("nested key1 should be overridden, got %v", nested["key1"])
	}

	if nested["key2"] != "base_nested2" {
		t.Errorf("nested key2 should be preserved, got %v", nested["key2"])
	}

	if nested["key3"] != "override_nested3" {
		t.Errorf("nested key3 should be added, got %v", nested["key3"])
	}
}

func TestVarManagerParseOSRelease(t *testing.T) {
	vm := NewVarManager()

	osReleaseContent := `NAME="Ubuntu"
VERSION="20.04.3 LTS (Focal Fossa)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 20.04.3 LTS"
VERSION_ID="20.04"
HOME_URL="https://www.ubuntu.com/"
VERSION_CODENAME=focal`

	result := vm.parseOSRelease(osReleaseContent)

	if result["ansible_distribution"] != "Ubuntu" {
		t.Errorf("expected Ubuntu, got %v", result["ansible_distribution"])
	}

	if result["ansible_distribution_version"] != "20.04" {
		t.Errorf("expected 20.04, got %v", result["ansible_distribution_version"])
	}

	if result["ansible_distribution_release"] != "focal" {
		t.Errorf("expected focal, got %v", result["ansible_distribution_release"])
	}

	if result["ansible_os_family"] != "ubuntu" {
		t.Errorf("expected ubuntu, got %v", result["ansible_os_family"])
	}
}

func TestVarManagerParseDefaultRoute(t *testing.T) {
	vm := NewVarManager()

	routeInfo := "1.1.1.1 via 192.168.1.1 dev eth0 src 192.168.1.100 uid 1000"
	result := vm.parseDefaultRoute(routeInfo)

	if result["address"] != "192.168.1.100" {
		t.Errorf("expected address 192.168.1.100, got %v", result["address"])
	}

	if result["interface"] != "eth0" {
		t.Errorf("expected interface eth0, got %v", result["interface"])
	}

	if result["gateway"] != "192.168.1.1" {
		t.Errorf("expected gateway 192.168.1.1, got %v", result["gateway"])
	}
}

func TestVarManagerParseMounts(t *testing.T) {
	vm := NewVarManager()

	dfOutput := `Filesystem     1K-blocks     Used Available Use% Mounted on
/dev/sda1      102687672 12345678  87654321  13% /
tmpfs           1234567       0   1234567   0% /dev/shm
/dev/sda2      10485760  5242880   5242880  50% /home`

	mounts := vm.parseMounts(dfOutput)

	if len(mounts) != 3 {
		t.Errorf("expected 3 mounts, got %d", len(mounts))
		return
	}

	// Check first mount
	mount0 := mounts[0]
	if mount0["device"] != "/dev/sda1" {
		t.Errorf("expected device /dev/sda1, got %v", mount0["device"])
	}
	if mount0["mount"] != "/" {
		t.Errorf("expected mount /, got %v", mount0["mount"])
	}

	// Check that percentage is calculated
	if _, exists := mount0["size_percent"]; !exists {
		t.Error("size_percent should be calculated")
	}
}

func TestVarManagerParseIdOutput(t *testing.T) {
	vm := NewVarManager()

	idOutput := "uid=1000(testuser) gid=1000(testuser) groups=1000(testuser),4(adm),24(cdrom)"
	result := vm.parseIdOutput(idOutput)

	if result["ansible_user_uid"] != 1000 {
		t.Errorf("expected UID 1000, got %v", result["ansible_user_uid"])
	}

	if result["ansible_user_gid"] != 1000 {
		t.Errorf("expected GID 1000, got %v", result["ansible_user_gid"])
	}
}

func TestVarManagerParseIPv4FromAddr(t *testing.T) {
	vm := NewVarManager()

	addrOutput := `2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether 52:54:00:12:34:56 brd ff:ff:ff:ff:ff:ff
    inet 192.168.1.100/24 brd 192.168.1.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::5054:ff:fe12:3456/64 scope link
       valid_lft forever preferred_lft forever`

	result := vm.parseIPv4FromAddr(addrOutput)

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if result["address"] != "192.168.1.100" {
		t.Errorf("expected address 192.168.1.100, got %v", result["address"])
	}

	if result["netmask"] != "255.255.255.0" {
		t.Errorf("expected netmask 255.255.255.0, got %v", result["netmask"])
	}
}

func TestVarManagerParseMACFromAddr(t *testing.T) {
	vm := NewVarManager()

	addrOutput := `2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether 52:54:00:12:34:56 brd ff:ff:ff:ff:ff:ff
    inet 192.168.1.100/24 brd 192.168.1.255 scope global eth0`

	mac := vm.parseMACFromAddr(addrOutput)

	if mac != "52:54:00:12:34:56" {
		t.Errorf("expected MAC 52:54:00:12:34:56, got %s", mac)
	}
}

func TestVarManagerConcurrency(t *testing.T) {
	vm := NewVarManager()

	// Test concurrent access to variables
	done := make(chan bool, 10)

	// Start multiple goroutines setting variables
	for i := 0; i < 5; i++ {
		go func(id int) {
			vm.SetVar(fmt.Sprintf("var%d", id), fmt.Sprintf("value%d", id))
			done <- true
		}(i)
	}

	// Start multiple goroutines reading variables
	for i := 0; i < 5; i++ {
		go func() {
			_ = vm.GetVars()
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all variables were set
	vars := vm.GetVars()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("var%d", i)
		expectedValue := fmt.Sprintf("value%d", i)
		if vars[key] != expectedValue {
			t.Errorf("variable %s expected %s, got %v", key, expectedValue, vars[key])
		}
	}
}

// Benchmark tests
func BenchmarkVarManagerSetVar(b *testing.B) {
	vm := NewVarManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vm.SetVar("benchmark_var", "benchmark_value")
	}
}

func BenchmarkVarManagerGetVar(b *testing.B) {
	vm := NewVarManager()
	vm.SetVar("benchmark_var", "benchmark_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vm.GetVar("benchmark_var")
	}
}

func BenchmarkVarManagerGetVars(b *testing.B) {
	vm := NewVarManager()

	// Set up some variables and facts
	for i := 0; i < 100; i++ {
		vm.SetVar(fmt.Sprintf("var%d", i), fmt.Sprintf("value%d", i))
	}

	vm.mu.Lock()
	for i := 0; i < 100; i++ {
		vm.facts[fmt.Sprintf("fact%d", i)] = fmt.Sprintf("factvalue%d", i)
	}
	vm.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vm.GetVars()
	}
}