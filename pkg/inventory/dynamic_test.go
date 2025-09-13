package inventory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// MockDynamicSource implements DynamicSource for testing
type MockDynamicSource struct {
	name          string
	inventoryData *DynamicInventoryData
	hostData      map[string]map[string]interface{}
	errorOnGet    bool
	getInventoryFunc func(ctx context.Context) (*DynamicInventoryData, error)
}

func (m *MockDynamicSource) GetInventory(ctx context.Context) (*DynamicInventoryData, error) {
	if m.getInventoryFunc != nil {
		return m.getInventoryFunc(ctx)
	}
	if m.errorOnGet {
		return nil, fmt.Errorf("mock error")
	}
	return m.inventoryData, nil
}

func (m *MockDynamicSource) GetHost(ctx context.Context, hostname string) (map[string]interface{}, error) {
	if m.errorOnGet {
		return nil, fmt.Errorf("mock error")
	}
	if data, ok := m.hostData[hostname]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("host not found")
}

func (m *MockDynamicSource) Name() string {
	return m.name
}

func (m *MockDynamicSource) Type() string {
	return "mock"
}

func TestDynamicInventory_Basic(t *testing.T) {
	mockData := &DynamicInventoryData{
		Groups: map[string]*GroupData{
			"webservers": {
				Hosts: []string{"web1", "web2"},
				Vars: map[string]interface{}{
					"http_port": 80,
				},
			},
			"databases": {
				Hosts: []string{"db1"},
				Vars: map[string]interface{}{
					"db_port": 5432,
				},
			},
		},
		HostVars: map[string]interface{}{
			"web1": map[string]interface{}{
				"ansible_host": "192.168.1.10",
				"ansible_port": 22,
			},
			"web2": map[string]interface{}{
				"ansible_host": "192.168.1.11",
				"ansible_port": 22,
			},
			"db1": map[string]interface{}{
				"ansible_host": "192.168.1.20",
				"ansible_port": 22,
			},
		},
	}
	
	source := &MockDynamicSource{
		name:          "test",
		inventoryData: mockData,
		hostData:      make(map[string]map[string]interface{}),
	}
	
	di := NewDynamicInventory(source, 5*time.Minute)
	
	// Test GetHosts
	hosts, err := di.GetHosts("webservers")
	if err != nil {
		t.Fatalf("Failed to get hosts: %v", err)
	}
	
	if len(hosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(hosts))
	}
	
	// Test GetHost
	host, err := di.GetHost("web1")
	if err != nil {
		t.Fatalf("Failed to get host: %v", err)
	}
	
	if host.Name != "web1" {
		t.Errorf("Expected host name 'web1', got '%s'", host.Name)
	}
	
	if host.Address != "192.168.1.10" {
		t.Errorf("Expected host address '192.168.1.10', got '%s'", host.Address)
	}
	
	// Test GetGroups
	groups, err := di.GetGroups()
	if err != nil {
		t.Fatalf("Failed to get groups: %v", err)
	}
	
	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}
}

func TestDynamicInventory_Cache(t *testing.T) {
	callCount := 0
	mockData := &DynamicInventoryData{
		Groups: map[string]*GroupData{
			"all": {
				Hosts: []string{"host1"},
			},
		},
		HostVars: map[string]interface{}{
			"host1": map[string]interface{}{
				"ansible_host": "127.0.0.1",
			},
		},
	}
	
	source := &MockDynamicSource{
		name:          "test",
		inventoryData: mockData,
		hostData:      make(map[string]map[string]interface{}),
	}
	
	// Track calls using function override
	source.getInventoryFunc = func(ctx context.Context) (*DynamicInventoryData, error) {
		callCount++
		return mockData, nil
	}
	
	di := NewDynamicInventory(source, 100*time.Millisecond)
	
	// First call should fetch from source
	_, err := di.GetHosts("all")
	if err != nil {
		t.Fatalf("Failed to get hosts: %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("Expected 1 call to GetInventory, got %d", callCount)
	}
	
	// Second call should use cache
	_, err = di.GetHosts("all")
	if err != nil {
		t.Fatalf("Failed to get hosts: %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("Expected 1 call to GetInventory (cached), got %d", callCount)
	}
	
	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)
	
	// Third call should fetch from source again
	_, err = di.GetHosts("all")
	if err != nil {
		t.Fatalf("Failed to get hosts: %v", err)
	}
	
	if callCount != 2 {
		t.Errorf("Expected 2 calls to GetInventory (cache expired), got %d", callCount)
	}
}

func TestDynamicInventory_HostVariables(t *testing.T) {
	mockData := &DynamicInventoryData{
		Groups: map[string]*GroupData{},
		HostVars: map[string]interface{}{
			"testhost": map[string]interface{}{
				"ansible_host": "10.0.0.1",
				"ansible_user": "admin",
				"ansible_port": float64(2222), // JSON numbers come as float64
				"custom_var":   "value",
			},
		},
	}
	
	source := &MockDynamicSource{
		name:          "test",
		inventoryData: mockData,
		hostData: map[string]map[string]interface{}{
			"testhost": {
				"from_source": "direct",
			},
		},
	}
	
	di := NewDynamicInventory(source, 5*time.Minute)
	
	// Test GetHostVars - should try source first
	vars, err := di.GetHostVars("testhost")
	if err != nil {
		t.Fatalf("Failed to get host vars: %v", err)
	}
	
	if vars["from_source"] != "direct" {
		t.Errorf("Expected to get vars from source first")
	}
	
	// Test with non-existent host in source
	vars, err = di.GetHostVars("testhost2")
	if err != nil {
		// This is expected since host doesn't exist
		return
	}
}

func TestDynamicInventory_GroupVariables(t *testing.T) {
	mockData := &DynamicInventoryData{
		Groups: map[string]*GroupData{
			"webservers": {
				Hosts: []string{"web1", "web2"},
				Vars: map[string]interface{}{
					"http_port":    80,
					"server_type":  "nginx",
					"enable_https": true,
				},
			},
		},
		HostVars: map[string]interface{}{},
	}
	
	source := &MockDynamicSource{
		name:          "test",
		inventoryData: mockData,
		hostData:      make(map[string]map[string]interface{}),
	}
	
	di := NewDynamicInventory(source, 5*time.Minute)
	
	vars, err := di.GetGroupVars("webservers")
	if err != nil {
		t.Fatalf("Failed to get group vars: %v", err)
	}
	
	if vars["http_port"] != 80 {
		t.Errorf("Expected http_port to be 80, got %v", vars["http_port"])
	}
	
	if vars["server_type"] != "nginx" {
		t.Errorf("Expected server_type to be 'nginx', got %v", vars["server_type"])
	}
	
	if vars["enable_https"] != true {
		t.Errorf("Expected enable_https to be true, got %v", vars["enable_https"])
	}
}

func TestDynamicInventory_ComplexGroups(t *testing.T) {
	mockData := &DynamicInventoryData{
		Groups: map[string]*GroupData{
			"all": {
				Children: []string{"webservers", "databases"},
				Vars: map[string]interface{}{
					"datacenter": "us-west",
				},
			},
			"webservers": {
				Hosts: []string{"web1", "web2"},
				Vars: map[string]interface{}{
					"http_port": 80,
				},
			},
			"databases": {
				Hosts: []string{"db1", "db2"},
				Children: []string{"mysql", "postgres"},
				Vars: map[string]interface{}{
					"backup_enabled": true,
				},
			},
			"mysql": {
				Hosts: []string{"db1"},
				Vars: map[string]interface{}{
					"port": 3306,
				},
			},
			"postgres": {
				Hosts: []string{"db2"},
				Vars: map[string]interface{}{
					"port": 5432,
				},
			},
		},
		HostVars: map[string]interface{}{
			"web1": map[string]interface{}{"ansible_host": "10.0.1.1"},
			"web2": map[string]interface{}{"ansible_host": "10.0.1.2"},
			"db1":  map[string]interface{}{"ansible_host": "10.0.2.1"},
			"db2":  map[string]interface{}{"ansible_host": "10.0.2.2"},
		},
	}
	
	source := &MockDynamicSource{
		name:          "test",
		inventoryData: mockData,
		hostData:      make(map[string]map[string]interface{}),
	}
	
	di := NewDynamicInventory(source, 5*time.Minute)
	
	// Test getting all groups
	groups, err := di.GetGroups()
	if err != nil {
		t.Fatalf("Failed to get groups: %v", err)
	}
	
	if len(groups) != 5 {
		t.Errorf("Expected 5 groups, got %d", len(groups))
	}
	
	// Test specific group with children
	group, err := di.GetGroup("databases")
	if err != nil {
		t.Fatalf("Failed to get databases group: %v", err)
	}
	
	if len(group.Children) != 2 {
		t.Errorf("Expected 2 children for databases group, got %d", len(group.Children))
	}
	
	if len(group.Hosts) != 2 {
		t.Errorf("Expected 2 hosts in databases group, got %d", len(group.Hosts))
	}
}

func TestDynamicInventory_AddMethods(t *testing.T) {
	mockData := &DynamicInventoryData{
		Groups:   map[string]*GroupData{},
		HostVars: map[string]interface{}{},
	}
	
	source := &MockDynamicSource{
		name:          "test",
		inventoryData: mockData,
		hostData:      make(map[string]map[string]interface{}),
	}
	
	di := NewDynamicInventory(source, 5*time.Minute)
	
	// Test AddHost - should fail
	err := di.AddHost(types.Host{Name: "test"})
	if err == nil {
		t.Error("Expected error when adding host to dynamic inventory")
	}
	
	if err.Error() != "cannot add host to dynamic inventory" {
		t.Errorf("Unexpected error message: %v", err)
	}
	
	// Test AddGroup - should fail
	err = di.AddGroup(types.Group{Name: "test"})
	if err == nil {
		t.Error("Expected error when adding group to dynamic inventory")
	}
	
	if err.Error() != "cannot add group to dynamic inventory" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestDynamicInventory_RefreshError(t *testing.T) {
	source := &MockDynamicSource{
		name:          "test",
		inventoryData: nil,
		errorOnGet:    true,
		hostData:      make(map[string]map[string]interface{}),
	}
	
	di := NewDynamicInventory(source, 5*time.Minute)
	
	_, err := di.GetHosts("all")
	if err == nil {
		t.Error("Expected error when source fails")
	}
	
	expectedErr := "failed to fetch inventory from test: mock error"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%v'", expectedErr, err)
	}
}

func TestInventoryCache_IsValid(t *testing.T) {
	cache := &InventoryCache{
		data:       &DynamicInventoryData{},
		expiration: time.Now().Add(1 * time.Minute),
		ttl:        1 * time.Minute,
	}
	
	if !cache.IsValid() {
		t.Error("Expected cache to be valid")
	}
	
	// Expired cache
	cache.expiration = time.Now().Add(-1 * time.Minute)
	if cache.IsValid() {
		t.Error("Expected cache to be invalid")
	}
	
	// Nil data
	cache.data = nil
	cache.expiration = time.Now().Add(1 * time.Minute)
	if cache.IsValid() {
		t.Error("Expected cache with nil data to be invalid")
	}
}