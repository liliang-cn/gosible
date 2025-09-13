package roles

import (
	"testing"
)

func TestDependencyResolver_SimpleChain(t *testing.T) {
	dr := NewDependencyResolver()
	
	// Create roles with simple chain: A -> B -> C
	roleA := &Role{Name: "roleA", Dependencies: []RoleDependency{{Role: "roleB"}}}
	roleB := &Role{Name: "roleB", Dependencies: []RoleDependency{{Role: "roleC"}}}
	roleC := &Role{Name: "roleC", Dependencies: []RoleDependency{}}
	
	dr.AddRole(roleA)
	dr.AddRole(roleB)
	dr.AddRole(roleC)
	
	roles, err := dr.Resolve()
	if err != nil {
		t.Fatalf("Failed to resolve dependencies: %v", err)
	}
	
	// Should be executed in order: C, B, A
	if len(roles) != 3 {
		t.Fatalf("Expected 3 roles, got %d", len(roles))
	}
	
	if roles[0].Name != "roleC" {
		t.Errorf("Expected first role to be roleC, got %s", roles[0].Name)
	}
	if roles[1].Name != "roleB" {
		t.Errorf("Expected second role to be roleB, got %s", roles[1].Name)
	}
	if roles[2].Name != "roleA" {
		t.Errorf("Expected third role to be roleA, got %s", roles[2].Name)
	}
}

func TestDependencyResolver_CircularDependency(t *testing.T) {
	dr := NewDependencyResolver()
	
	// Create circular dependency: A -> B -> C -> A
	roleA := &Role{Name: "roleA", Dependencies: []RoleDependency{{Role: "roleB"}}}
	roleB := &Role{Name: "roleB", Dependencies: []RoleDependency{{Role: "roleC"}}}
	roleC := &Role{Name: "roleC", Dependencies: []RoleDependency{{Role: "roleA"}}}
	
	dr.AddRole(roleA)
	dr.AddRole(roleB)
	dr.AddRole(roleC)
	
	_, err := dr.Resolve()
	if err == nil {
		t.Fatal("Expected circular dependency error, got nil")
	}
	
	if err.Error() != "circular dependency detected involving role 'roleA'" {
		t.Errorf("Expected circular dependency error for roleA, got: %v", err)
	}
}

func TestDependencyResolver_MultipleDependencies(t *testing.T) {
	dr := NewDependencyResolver()
	
	// Role A depends on B and C
	roleA := &Role{Name: "roleA", Dependencies: []RoleDependency{
		{Role: "roleB"},
		{Role: "roleC"},
	}}
	roleB := &Role{Name: "roleB", Dependencies: []RoleDependency{}}
	roleC := &Role{Name: "roleC", Dependencies: []RoleDependency{}}
	
	dr.AddRole(roleA)
	dr.AddRole(roleB)
	dr.AddRole(roleC)
	
	roles, err := dr.Resolve()
	if err != nil {
		t.Fatalf("Failed to resolve dependencies: %v", err)
	}
	
	if len(roles) != 3 {
		t.Fatalf("Expected 3 roles, got %d", len(roles))
	}
	
	// A should be last
	if roles[2].Name != "roleA" {
		t.Errorf("Expected last role to be roleA, got %s", roles[2].Name)
	}
	
	// B and C should come before A
	foundB := false
	foundC := false
	for i := 0; i < 2; i++ {
		if roles[i].Name == "roleB" {
			foundB = true
		}
		if roles[i].Name == "roleC" {
			foundC = true
		}
	}
	
	if !foundB || !foundC {
		t.Error("Expected roleB and roleC to be resolved before roleA")
	}
}

func TestDependencyResolver_MissingDependency(t *testing.T) {
	dr := NewDependencyResolver()
	
	// Role A depends on B, but B doesn't exist
	roleA := &Role{Name: "roleA", Dependencies: []RoleDependency{{Role: "roleB"}}}
	
	dr.AddRole(roleA)
	
	_, err := dr.Resolve()
	if err == nil {
		t.Fatal("Expected missing dependency error, got nil")
	}
	
	expectedErr := "dependency 'roleB' of role 'roleA' not found"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got: %v", expectedErr, err)
	}
}

func TestDependencyResolver_NoDependencies(t *testing.T) {
	dr := NewDependencyResolver()
	
	roleA := &Role{Name: "roleA", Dependencies: []RoleDependency{}}
	roleB := &Role{Name: "roleB", Dependencies: []RoleDependency{}}
	roleC := &Role{Name: "roleC", Dependencies: []RoleDependency{}}
	
	dr.AddRole(roleA)
	dr.AddRole(roleB)
	dr.AddRole(roleC)
	
	roles, err := dr.Resolve()
	if err != nil {
		t.Fatalf("Failed to resolve dependencies: %v", err)
	}
	
	if len(roles) != 3 {
		t.Fatalf("Expected 3 roles, got %d", len(roles))
	}
	
	// Check all roles are present
	foundRoles := make(map[string]bool)
	for _, role := range roles {
		foundRoles[role.Name] = true
	}
	
	if !foundRoles["roleA"] || !foundRoles["roleB"] || !foundRoles["roleC"] {
		t.Error("Not all roles were resolved")
	}
}

func TestDependencyResolver_GetExecutionOrder(t *testing.T) {
	dr := NewDependencyResolver()
	
	roleA := &Role{Name: "roleA", Dependencies: []RoleDependency{{Role: "roleB"}}}
	roleB := &Role{Name: "roleB", Dependencies: []RoleDependency{}}
	
	dr.AddRole(roleA)
	dr.AddRole(roleB)
	
	_, err := dr.Resolve()
	if err != nil {
		t.Fatalf("Failed to resolve dependencies: %v", err)
	}
	
	order := dr.GetExecutionOrder()
	if len(order) != 2 {
		t.Fatalf("Expected 2 roles in execution order, got %d", len(order))
	}
	
	if order[0] != "roleB" {
		t.Errorf("Expected first in execution order to be roleB, got %s", order[0])
	}
	if order[1] != "roleA" {
		t.Errorf("Expected second in execution order to be roleA, got %s", order[1])
	}
}

func TestDependencyResolver_GetDependencyGraph(t *testing.T) {
	dr := NewDependencyResolver()
	
	roleA := &Role{Name: "roleA", Dependencies: []RoleDependency{
		{Role: "roleB"},
		{Role: "roleC"},
	}}
	roleB := &Role{Name: "roleB", Dependencies: []RoleDependency{{Role: "roleC"}}}
	roleC := &Role{Name: "roleC", Dependencies: []RoleDependency{}}
	
	dr.AddRole(roleA)
	dr.AddRole(roleB)
	dr.AddRole(roleC)
	
	graph := dr.GetDependencyGraph()
	
	// Check roleA dependencies
	if len(graph["roleA"]) != 2 {
		t.Errorf("Expected roleA to have 2 dependencies, got %d", len(graph["roleA"]))
	}
	
	// Check roleB dependencies
	if len(graph["roleB"]) != 1 {
		t.Errorf("Expected roleB to have 1 dependency, got %d", len(graph["roleB"]))
	}
	
	// Check roleC dependencies
	if len(graph["roleC"]) != 0 {
		t.Errorf("Expected roleC to have 0 dependencies, got %d", len(graph["roleC"]))
	}
}

func TestDependencyResolver_GetDependents(t *testing.T) {
	dr := NewDependencyResolver()
	
	roleA := &Role{Name: "roleA", Dependencies: []RoleDependency{{Role: "roleC"}}}
	roleB := &Role{Name: "roleB", Dependencies: []RoleDependency{{Role: "roleC"}}}
	roleC := &Role{Name: "roleC", Dependencies: []RoleDependency{}}
	
	dr.AddRole(roleA)
	dr.AddRole(roleB)
	dr.AddRole(roleC)
	
	dependents := dr.GetDependents("roleC")
	
	if len(dependents) != 2 {
		t.Fatalf("Expected 2 dependents for roleC, got %d", len(dependents))
	}
	
	foundA := false
	foundB := false
	for _, dep := range dependents {
		if dep == "roleA" {
			foundA = true
		}
		if dep == "roleB" {
			foundB = true
		}
	}
	
	if !foundA || !foundB {
		t.Error("Expected roleA and roleB to be dependents of roleC")
	}
}

func TestDependencyResolver_ComplexDependencyTree(t *testing.T) {
	dr := NewDependencyResolver()
	
	// Complex tree:
	//     A
	//    / \
	//   B   C
	//  / \ /
	// D   E
	
	roleA := &Role{Name: "roleA", Dependencies: []RoleDependency{
		{Role: "roleB"},
		{Role: "roleC"},
	}}
	roleB := &Role{Name: "roleB", Dependencies: []RoleDependency{
		{Role: "roleD"},
		{Role: "roleE"},
	}}
	roleC := &Role{Name: "roleC", Dependencies: []RoleDependency{
		{Role: "roleE"},
	}}
	roleD := &Role{Name: "roleD", Dependencies: []RoleDependency{}}
	roleE := &Role{Name: "roleE", Dependencies: []RoleDependency{}}
	
	dr.AddRole(roleA)
	dr.AddRole(roleB)
	dr.AddRole(roleC)
	dr.AddRole(roleD)
	dr.AddRole(roleE)
	
	roles, err := dr.Resolve()
	if err != nil {
		t.Fatalf("Failed to resolve dependencies: %v", err)
	}
	
	if len(roles) != 5 {
		t.Fatalf("Expected 5 roles, got %d", len(roles))
	}
	
	// Check ordering constraints
	orderMap := make(map[string]int)
	for i, role := range roles {
		orderMap[role.Name] = i
	}
	
	// D and E should come before B
	if orderMap["roleD"] >= orderMap["roleB"] {
		t.Error("roleD should be resolved before roleB")
	}
	if orderMap["roleE"] >= orderMap["roleB"] {
		t.Error("roleE should be resolved before roleB")
	}
	
	// E should come before C
	if orderMap["roleE"] >= orderMap["roleC"] {
		t.Error("roleE should be resolved before roleC")
	}
	
	// B and C should come before A
	if orderMap["roleB"] >= orderMap["roleA"] {
		t.Error("roleB should be resolved before roleA")
	}
	if orderMap["roleC"] >= orderMap["roleA"] {
		t.Error("roleC should be resolved before roleA")
	}
}