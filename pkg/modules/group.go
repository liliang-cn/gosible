package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	
	"github.com/gosinble/gosinble/pkg/types"
)

// GroupModule manages system groups
type GroupModule struct {
	BaseModule
}

// NewGroupModule creates a new group module instance
func NewGroupModule() *GroupModule {
	return &GroupModule{
		BaseModule: BaseModule{
			name: "group",
		},
	}
}

// Run executes the group module
func (m *GroupModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Get arguments
	name, _ := args["name"].(string)
	state, _ := args["state"].(string)
	gid, _ := args["gid"]
	system, _ := args["system"].(bool)
	
	// Default state is present
	if state == "" {
		state = "present"
	}
	
	result := &types.Result{
		Success: true,
		Changed: false,
		Data:    make(map[string]interface{}),
	}
	
	// Check if group exists
	exists, currentGID := m.groupExists(ctx, conn, name)
	
	switch state {
	case "present":
		if exists {
			// Check if GID needs to be updated
			if gid != nil {
				requestedGID, err := m.toInt(gid)
				if err != nil {
					result.Success = false
					result.Error = fmt.Errorf("invalid gid: %v", err)
					return result, nil
				}
				
				if requestedGID != currentGID {
					if err := m.updateGroupGID(ctx, conn, name, requestedGID); err != nil {
						result.Success = false
						result.Error = err
						return result, nil
					}
					result.Changed = true
					result.Message = fmt.Sprintf("Group %s GID updated to %d", name, requestedGID)
				} else {
					result.Message = fmt.Sprintf("Group %s already exists with GID %d", name, currentGID)
				}
			} else {
				result.Message = fmt.Sprintf("Group %s already exists", name)
			}
		} else {
			// Create group
			err := m.createGroup(ctx, conn, name, gid, system)
			if err != nil {
				result.Success = false
				result.Error = err
				return result, nil
			}
			result.Changed = true
			result.Message = fmt.Sprintf("Group %s created", name)
		}
		
		// Get group info after operation
		_, updatedGID := m.groupExists(ctx, conn, name)
		if updatedGID > 0 {
			result.Data["gid"] = strconv.Itoa(updatedGID)
		}
		
		groupInfo := m.getGroupInfo(ctx, conn, name)
		if members, ok := groupInfo["members"]; ok {
			result.Data["members"] = members
		}
		
	case "absent":
		if exists {
			err := m.removeGroup(ctx, conn, name)
			if err != nil {
				result.Success = false
				result.Error = err
				return result, nil
			}
			result.Changed = true
			result.Message = fmt.Sprintf("Group %s removed", name)
		} else {
			result.Message = fmt.Sprintf("Group %s already absent", name)
		}
		
	default:
		result.Success = false
		result.Error = fmt.Errorf("unsupported state: %s", state)
		return result, nil
	}
	
	return result, nil
}

// groupExists checks if a group exists and returns its GID
func (m *GroupModule) groupExists(ctx context.Context, conn types.Connection, name string) (bool, int) {
	cmd := fmt.Sprintf("getent group %s 2>/dev/null", name)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil || !result.Success {
		return false, 0
	}
	
	// Parse output to get GID
	output := strings.TrimSpace(result.Message)
	if output == "" {
		return false, 0
	}
	
	parts := strings.Split(output, ":")
	if len(parts) < 3 {
		return true, 0
	}
	
	gid, _ := strconv.Atoi(parts[2])
	return true, gid
}

// createGroup creates a new group
func (m *GroupModule) createGroup(ctx context.Context, conn types.Connection, name string, gid interface{}, system bool) error {
	cmd := "groupadd"
	
	// Add options
	if system {
		cmd += " -r"
	}
	
	if gid != nil {
		gidInt, err := m.toInt(gid)
		if err != nil {
			return fmt.Errorf("invalid gid: %v", err)
		}
		cmd += fmt.Sprintf(" -g %d", gidInt)
	}
	
	cmd += " " + name
	
	// Create group
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return fmt.Errorf("failed to create group: %v", err)
	}
	
	return nil
}

// updateGroupGID updates a group's GID
func (m *GroupModule) updateGroupGID(ctx context.Context, conn types.Connection, name string, gid int) error {
	cmd := fmt.Sprintf("groupmod -g %d %s", gid, name)
	
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return fmt.Errorf("failed to update group GID: %v", err)
	}
	
	return nil
}

// removeGroup removes a group
func (m *GroupModule) removeGroup(ctx context.Context, conn types.Connection, name string) error {
	cmd := fmt.Sprintf("groupdel %s", name)
	
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return fmt.Errorf("failed to remove group: %v", err)
	}
	
	return nil
}

// getGroupInfo gets information about a group
func (m *GroupModule) getGroupInfo(ctx context.Context, conn types.Connection, name string) map[string]string {
	info := make(map[string]string)
	
	// Get group info from /etc/group
	cmd := fmt.Sprintf("getent group %s 2>/dev/null", name)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err == nil && result.Success {
		parts := strings.Split(strings.TrimSpace(result.Message), ":")
		if len(parts) >= 4 {
			info["gid"] = parts[2]
			info["members"] = parts[3]
		}
	}
	
	return info
}

// toInt converts various types to int
func (m *GroupModule) toInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case float64:
		return int(val), nil
	case string:
		return strconv.Atoi(val)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

// Validate checks if the module arguments are valid
func (m *GroupModule) Validate(args map[string]interface{}) error {
	// Name is required
	name, ok := args["name"]
	if !ok || name == nil || name == "" {
		return types.NewValidationError("name", name, "required field is missing")
	}
	
	// Validate state if provided
	if state, ok := args["state"].(string); ok && state != "" {
		validStates := []string{"present", "absent"}
		valid := false
		for _, s := range validStates {
			if state == s {
				valid = true
				break
			}
		}
		if !valid {
			return types.NewValidationError("state", state, 
				fmt.Sprintf("must be one of: %s", strings.Join(validStates, ", ")))
		}
	}
	
	return nil
}

// Documentation returns the module documentation
func (m *GroupModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "group",
		Description: "Manage groups",
		Parameters: map[string]types.ParamDoc{
			"name": {
				Description: "Name of the group",
				Required:    true,
				Type:        "string",
			},
			"state": {
				Description: "State of the group",
				Required:    false,
				Type:        "string",
				Default:     "present",
				Choices:     []string{"present", "absent"},
			},
			"gid": {
				Description: "Group ID",
				Required:    false,
				Type:        "int",
			},
			"system": {
				Description: "Create system group",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
		},
		Examples: []string{
			"- name: Create group\n  group:\n    name: developers\n    gid: 2001",
			"- name: Create system group\n  group:\n    name: myservice\n    system: true",
			"- name: Remove group\n  group:\n    name: oldgroup\n    state: absent",
		},
		Returns: map[string]string{
			"gid":     "Group ID",
			"members": "Group members",
		},
	}
}