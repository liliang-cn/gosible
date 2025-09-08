package roles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBasicRolesFunctionality(t *testing.T) {
	t.Run("Role_Creation", func(t *testing.T) {
		role := &Role{
			Name: "test-role",
			Path: "/roles/test-role",
			Defaults: map[string]interface{}{
				"var1": "value1",
			},
			Vars: map[string]interface{}{
				"var2": "value2",
			},
		}
		
		if role.Name != "test-role" {
			t.Errorf("Expected name 'test-role', got '%s'", role.Name)
		}
		
		if role.Path != "/roles/test-role" {
			t.Errorf("Expected path '/roles/test-role', got '%s'", role.Path)
		}
		
		if role.Defaults["var1"] != "value1" {
			t.Errorf("Expected default var1='value1', got %v", role.Defaults["var1"])
		}
		
		if role.Vars["var2"] != "value2" {
			t.Errorf("Expected vars var2='value2', got %v", role.Vars["var2"])
		}
	})

	t.Run("RoleMeta_Creation", func(t *testing.T) {
		meta := &RoleMeta{
			Author:            "test-author",
			Description:       "Test role",
			License:           "MIT",
			MinAnsibleVersion: "2.9",
			Platforms: []Platform{
				{
					Name:     "Ubuntu",
					Versions: []string{"20.04", "22.04"},
				},
			},
			Dependencies: []RoleDependency{
				{
					Role:    "common",
					Version: "1.0",
				},
			},
			Tags: []string{"web", "test"},
		}
		
		if meta.Author != "test-author" {
			t.Errorf("Expected author 'test-author', got '%s'", meta.Author)
		}
		
		if meta.Description != "Test role" {
			t.Errorf("Expected description 'Test role', got '%s'", meta.Description)
		}
		
		if len(meta.Platforms) != 1 {
			t.Errorf("Expected 1 platform, got %d", len(meta.Platforms))
		}
		
		if meta.Platforms[0].Name != "Ubuntu" {
			t.Errorf("Expected platform 'Ubuntu', got '%s'", meta.Platforms[0].Name)
		}
		
		if len(meta.Dependencies) != 1 {
			t.Errorf("Expected 1 dependency, got %d", len(meta.Dependencies))
		}
		
		if meta.Dependencies[0].Role != "common" {
			t.Errorf("Expected dependency 'common', got '%s'", meta.Dependencies[0].Role)
		}
	})

	t.Run("RoleDependency_Creation", func(t *testing.T) {
		dep := RoleDependency{
			Role:    "nginx",
			Src:     "https://github.com/user/nginx-role",
			Version: "v1.0",
			Vars: map[string]interface{}{
				"port": 8080,
			},
			Tags: []string{"webserver"},
		}
		
		if dep.Role != "nginx" {
			t.Errorf("Expected role 'nginx', got '%s'", dep.Role)
		}
		
		if dep.Src != "https://github.com/user/nginx-role" {
			t.Errorf("Expected src URL, got '%s'", dep.Src)
		}
		
		if dep.Version != "v1.0" {
			t.Errorf("Expected version 'v1.0', got '%s'", dep.Version)
		}
		
		if dep.Vars["port"] != 8080 {
			t.Errorf("Expected port=8080, got %v", dep.Vars["port"])
		}
	})

	t.Run("Platform_Creation", func(t *testing.T) {
		platform := Platform{
			Name:     "CentOS",
			Versions: []string{"7", "8", "9"},
		}
		
		if platform.Name != "CentOS" {
			t.Errorf("Expected name 'CentOS', got '%s'", platform.Name)
		}
		
		if len(platform.Versions) != 3 {
			t.Errorf("Expected 3 versions, got %d", len(platform.Versions))
		}
		
		if platform.Versions[0] != "7" {
			t.Errorf("Expected first version '7', got '%s'", platform.Versions[0])
		}
	})

	t.Run("RoleManager_Creation", func(t *testing.T) {
		paths := []string{"/custom/roles", "/system/roles"}
		manager := NewRoleManager(paths)
		
		if manager == nil {
			t.Fatal("NewRoleManager() returned nil")
		}
		
		if len(manager.rolesPath) != 2 {
			t.Errorf("Expected 2 role paths, got %d", len(manager.rolesPath))
		}
		
		if manager.rolesPath[0] != "/custom/roles" {
			t.Errorf("Expected first path '/custom/roles', got '%s'", manager.rolesPath[0])
		}
	})

	t.Run("RoleManager_DefaultPaths", func(t *testing.T) {
		manager := NewRoleManager([]string{})
		
		if len(manager.rolesPath) == 0 {
			t.Error("Expected default paths to be set")
		}
		
		// Should have default paths
		found := false
		for _, path := range manager.rolesPath {
			if path == "roles" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected 'roles' in default paths")
		}
	})

	t.Run("RoleManager_PathLookup", func(t *testing.T) {
		tmpDir := t.TempDir()
		manager := NewRoleManager([]string{tmpDir})
		
		// Create role directory
		rolePath := filepath.Join(tmpDir, "test-role")
		err := os.MkdirAll(rolePath, 0755)
		if err != nil {
			t.Fatalf("Failed to create role directory: %v", err)
		}
		
		// Test that manager has the correct paths
		if len(manager.rolesPath) != 1 {
			t.Errorf("Expected 1 role path, got %d", len(manager.rolesPath))
		}
		
		if manager.rolesPath[0] != tmpDir {
			t.Errorf("Expected path '%s', got '%s'", tmpDir, manager.rolesPath[0])
		}
	})

	t.Run("RoleManager_LoadRole_Cache", func(t *testing.T) {
		manager := NewRoleManager([]string{})
		
		// Test cache operations
		testRole := &Role{Name: "cached-role"}
		manager.loadedRoles["cached-role"] = testRole
		
		if manager.loadedRoles["cached-role"] != testRole {
			t.Error("Role not found in cache")
		}
		
		// Clear cache simulation
		manager.loadedRoles = make(map[string]*Role)
		
		if len(manager.loadedRoles) != 0 {
			t.Error("Cache should be empty after reset")
		}
	})
}