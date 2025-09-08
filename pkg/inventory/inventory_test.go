package inventory

import (
	"fmt"
	"strings"
	"testing"

	"github.com/liliang-cn/gosinble/pkg/types"
)

func TestNewStaticInventory(t *testing.T) {
	inv := NewStaticInventory()
	if inv == nil {
		t.Fatal("NewStaticInventory returned nil")
	}

	if inv.hosts == nil || inv.groups == nil {
		t.Error("inventory maps not initialized")
	}
}

func TestNewFromYAML(t *testing.T) {
	yamlData := `
all:
  hosts:
    web1:
      address: 192.168.1.10
      port: 22
      user: ubuntu
      vars:
        env: production
    web2:
      address: 192.168.1.11
      port: 22
      user: ubuntu
  children:
    webservers:
      hosts:
        - web1
        - web2
      vars:
        http_port: 80
    databases:
      hosts:
        - db1
      vars:
        db_port: 5432
`

	inv, err := NewFromYAML([]byte(yamlData))
	if err != nil {
		t.Fatalf("NewFromYAML failed: %v", err)
	}

	// Check hosts
	if len(inv.hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(inv.hosts))
	}

	web1, exists := inv.hosts["web1"]
	if !exists {
		t.Error("web1 host not found")
	}
	if web1.Address != "192.168.1.10" {
		t.Errorf("web1 address expected 192.168.1.10, got %s", web1.Address)
	}

	// Check groups (all, webservers, databases)
	if len(inv.groups) != 3 {
		t.Errorf("expected 3 groups, got %d", len(inv.groups))
	}

	webservers, exists := inv.groups["webservers"]
	if !exists {
		t.Error("webservers group not found")
	}
	if len(webservers.Hosts) != 2 {
		t.Errorf("webservers group expected 2 hosts, got %d", len(webservers.Hosts))
	}
}

func TestAddHost(t *testing.T) {
	inv := NewStaticInventory()

	host := types.Host{
		Name:    "test1",
		Address: "192.168.1.100",
		Port:    22,
		User:    "root",
		Variables: map[string]interface{}{
			"env": "test",
		},
		Groups: []string{"testgroup"},
	}

	err := inv.AddHost(host)
	if err != nil {
		t.Fatalf("AddHost failed: %v", err)
	}

	// Check host was added
	storedHost, exists := inv.hosts["test1"]
	if !exists {
		t.Error("host was not stored")
	}
	if storedHost.Address != "192.168.1.100" {
		t.Errorf("host address expected 192.168.1.100, got %s", storedHost.Address)
	}

	// Check group was created
	group, exists := inv.groups["testgroup"]
	if !exists {
		t.Error("group was not created")
	}
	if len(group.Hosts) != 1 || group.Hosts[0] != "test1" {
		t.Error("host was not added to group")
	}
}

func TestGetHosts(t *testing.T) {
	inv := NewStaticInventory()

	// Add test hosts
	hosts := []types.Host{
		{Name: "web1", Address: "192.168.1.10", Groups: []string{"webservers"}},
		{Name: "web2", Address: "192.168.1.11", Groups: []string{"webservers"}},
		{Name: "db1", Address: "192.168.1.20", Groups: []string{"databases"}},
	}

	for _, host := range hosts {
		inv.AddHost(host)
	}

	tests := []struct {
		pattern  string
		expected int
		desc     string
	}{
		{"*", 3, "all hosts"},
		{"", 3, "empty pattern (all hosts)"},
		{"web*", 2, "web hosts by wildcard"},
		{"webservers", 2, "hosts in webservers group"},
		{"databases", 1, "hosts in databases group"},
		{"nonexistent", 0, "nonexistent pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := inv.GetHosts(tt.pattern)
			if err != nil {
				t.Fatalf("GetHosts failed: %v", err)
			}
			if len(result) != tt.expected {
				t.Errorf("expected %d hosts, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestGetHost(t *testing.T) {
	inv := NewStaticInventory()

	host := types.Host{
		Name:    "test1",
		Address: "192.168.1.100",
		Port:    22,
	}

	inv.AddHost(host)

	// Test get by name
	retrieved, err := inv.GetHost("test1")
	if err != nil {
		t.Fatalf("GetHost by name failed: %v", err)
	}
	if retrieved.Name != "test1" {
		t.Errorf("expected name test1, got %s", retrieved.Name)
	}

	// Test get by address
	retrieved, err = inv.GetHost("192.168.1.100")
	if err != nil {
		t.Fatalf("GetHost by address failed: %v", err)
	}
	if retrieved.Name != "test1" {
		t.Errorf("expected name test1, got %s", retrieved.Name)
	}

	// Test nonexistent host
	_, err = inv.GetHost("nonexistent")
	if err != types.ErrHostNotFound {
		t.Errorf("expected ErrHostNotFound, got %v", err)
	}
}

func TestGetHostVars(t *testing.T) {
	inv := NewStaticInventory()

	// Create group with variables
	group := types.Group{
		Name: "webservers",
		Variables: map[string]interface{}{
			"http_port": 80,
			"env":       "production",
		},
	}
	inv.AddGroup(group)

	// Create host with variables
	host := types.Host{
		Name:    "web1",
		Address: "192.168.1.10",
		Port:    22,
		User:    "ubuntu",
		Variables: map[string]interface{}{
			"env":         "staging", // This should override group var
			"server_role": "frontend",
		},
		Groups: []string{"webservers"},
	}
	inv.AddHost(host)

	vars, err := inv.GetHostVars("web1")
	if err != nil {
		t.Fatalf("GetHostVars failed: %v", err)
	}

	// Check built-in variables
	if vars["inventory_hostname"] != "web1" {
		t.Errorf("inventory_hostname expected web1, got %v", vars["inventory_hostname"])
	}
	if vars["ansible_host"] != "192.168.1.10" {
		t.Errorf("ansible_host expected 192.168.1.10, got %v", vars["ansible_host"])
	}
	if vars["ansible_port"] != 22 {
		t.Errorf("ansible_port expected 22, got %v", vars["ansible_port"])
	}
	if vars["ansible_user"] != "ubuntu" {
		t.Errorf("ansible_user expected ubuntu, got %v", vars["ansible_user"])
	}

	// Check group variables
	if vars["http_port"] != 80 {
		t.Errorf("http_port expected 80, got %v", vars["http_port"])
	}

	// Check host variables override group variables
	if vars["env"] != "staging" {
		t.Errorf("env expected staging (host override), got %v", vars["env"])
	}

	// Check host-specific variables
	if vars["server_role"] != "frontend" {
		t.Errorf("server_role expected frontend, got %v", vars["server_role"])
	}
}

func TestExpandPattern(t *testing.T) {
	inv := NewStaticInventory()

	tests := []struct {
		pattern  string
		expected []string
		desc     string
	}{
		{
			"web[1:3].example.com",
			[]string{"web1.example.com", "web2.example.com", "web3.example.com"},
			"range pattern",
		},
		{
			"db[01:03].local",
			[]string{"db01.local", "db02.local", "db03.local"},
			"zero-padded range pattern",
		},
		{
			"server{1,3,5}.test",
			[]string{"server1.test", "server3.test", "server5.test"},
			"list pattern",
		},
		{
			"single.host",
			[]string{"single.host"},
			"single host pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := inv.ExpandPattern(tt.pattern)
			if err != nil {
				t.Fatalf("ExpandPattern failed: %v", err)
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d results, got %d", len(tt.expected), len(result))
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("result[%d] expected %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

func TestToYAMLAndSaveToFile(t *testing.T) {
	inv := NewStaticInventory()

	// Add test data
	host := types.Host{
		Name:    "web1",
		Address: "192.168.1.10",
		Port:    22,
		Variables: map[string]interface{}{
			"env": "test",
		},
	}
	inv.AddHost(host)

	group := types.Group{
		Name:  "webservers",
		Hosts: []string{"web1"},
		Variables: map[string]interface{}{
			"http_port": 80,
		},
	}
	inv.AddGroup(group)

	// Test YAML export
	yamlData, err := inv.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML failed: %v", err)
	}

	yamlStr := string(yamlData)
	if !strings.Contains(yamlStr, "web1") {
		t.Error("YAML does not contain web1")
	}
	if !strings.Contains(yamlStr, "webservers") {
		t.Error("YAML does not contain webservers")
	}

	// Test parsing the exported YAML
	parsedInv, err := NewFromYAML(yamlData)
	if err != nil {
		t.Fatalf("Failed to parse exported YAML: %v", err)
	}

	if len(parsedInv.hosts) != 1 {
		t.Errorf("parsed inventory expected 1 host, got %d", len(parsedInv.hosts))
	}
	if len(parsedInv.groups) != 2 {
		t.Errorf("parsed inventory expected 2 groups (webservers + all), got %d", len(parsedInv.groups))
	}
}

func TestRemoveHost(t *testing.T) {
	inv := NewStaticInventory()

	// Add host and group
	host := types.Host{
		Name:   "web1",
		Groups: []string{"webservers"},
	}
	inv.AddHost(host)

	// Verify host exists
	if _, err := inv.GetHost("web1"); err != nil {
		t.Fatalf("Host should exist before removal: %v", err)
	}

	// Remove host
	err := inv.RemoveHost("web1")
	if err != nil {
		t.Fatalf("RemoveHost failed: %v", err)
	}

	// Verify host is gone
	if _, err := inv.GetHost("web1"); err != types.ErrHostNotFound {
		t.Error("Host should not exist after removal")
	}

	// Verify host was removed from group
	group, _ := inv.GetGroup("webservers")
	if len(group.Hosts) != 0 {
		t.Error("Host should be removed from group")
	}
}

func TestRemoveGroup(t *testing.T) {
	inv := NewStaticInventory()

	// Add host and group
	host := types.Host{
		Name:   "web1",
		Groups: []string{"webservers"},
	}
	inv.AddHost(host)

	// Verify group exists
	if _, err := inv.GetGroup("webservers"); err != nil {
		t.Fatalf("Group should exist before removal: %v", err)
	}

	// Remove group
	err := inv.RemoveGroup("webservers")
	if err != nil {
		t.Fatalf("RemoveGroup failed: %v", err)
	}

	// Verify group is gone
	if _, err := inv.GetGroup("webservers"); err != types.ErrGroupNotFound {
		t.Error("Group should not exist after removal")
	}

	// Verify group was removed from host
	updatedHost, _ := inv.GetHost("web1")
	if len(updatedHost.Groups) != 0 {
		t.Error("Group should be removed from host")
	}
}

func BenchmarkGetHosts(b *testing.B) {
	inv := NewStaticInventory()

	// Add many hosts for benchmarking
	for i := 0; i < 1000; i++ {
		host := types.Host{
			Name:    fmt.Sprintf("host%d", i),
			Address: fmt.Sprintf("192.168.1.%d", i%254+1),
			Groups:  []string{"testgroup"},
		}
		inv.AddHost(host)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := inv.GetHosts("testgroup")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetHostVars(b *testing.B) {
	inv := NewStaticInventory()

	// Create group with many variables
	group := types.Group{
		Name: "testgroup",
		Variables: make(map[string]interface{}),
	}
	for i := 0; i < 100; i++ {
		group.Variables[fmt.Sprintf("var%d", i)] = fmt.Sprintf("value%d", i)
	}
	inv.AddGroup(group)

	// Create host
	host := types.Host{
		Name:      "testhost",
		Groups:    []string{"testgroup"},
		Variables: make(map[string]interface{}),
	}
	for i := 0; i < 50; i++ {
		host.Variables[fmt.Sprintf("hostvar%d", i)] = fmt.Sprintf("hostvalue%d", i)
	}
	inv.AddHost(host)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := inv.GetHostVars("testhost")
		if err != nil {
			b.Fatal(err)
		}
	}
}