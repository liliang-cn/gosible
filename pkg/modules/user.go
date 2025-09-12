package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	
	"github.com/liliang-cn/gosible/pkg/types"
)

// UserModule manages user accounts
type UserModule struct {
	BaseModule
}

// NewUserModule creates a new user module instance
func NewUserModule() *UserModule {
	return &UserModule{
		BaseModule: BaseModule{
			name: "user",
		},
	}
}

// Run executes the user module
func (m *UserModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	// Get arguments
	name, _ := args["name"].(string)
	state, _ := args["state"].(string)
	uid, _ := args["uid"]
	gid, _ := args["gid"]
	groups, _ := args["groups"].(string)
	appendGroups, _ := args["append"].(bool)
	home, _ := args["home"].(string)
	shell, _ := args["shell"].(string)
	password, _ := args["password"].(string)
	comment, _ := args["comment"].(string)
	createHome, _ := args["create_home"].(bool)
	system, _ := args["system"].(bool)
	remove, _ := args["remove"].(bool)
	force, _ := args["force"].(bool)
	
	// Default state is present
	if state == "" {
		state = "present"
	}
	
	// Default create_home to true for normal users
	if state == "present" && !system {
		if _, ok := args["create_home"]; !ok {
			createHome = true
		}
	}
	
	result := &types.Result{
		Success: true,
		Changed: false,
		Data:    make(map[string]interface{}),
	}
	
	// Check if user exists
	exists := m.userExists(ctx, conn, name)
	
	switch state {
	case "present":
		if exists {
			// Update user if needed
			changed, err := m.updateUser(ctx, conn, name, uid, gid, groups, appendGroups, home, shell, password, comment)
			if err != nil {
				result.Success = false
				result.Error = err
				return result, nil
			}
			result.Changed = changed
			if changed {
				result.Message = fmt.Sprintf("User %s updated", name)
			} else {
				result.Message = fmt.Sprintf("User %s already exists with correct settings", name)
			}
		} else {
			// Create user
			err := m.createUser(ctx, conn, name, uid, gid, groups, home, shell, password, comment, createHome, system)
			if err != nil {
				result.Success = false
				result.Error = err
				return result, nil
			}
			result.Changed = true
			result.Message = fmt.Sprintf("User %s created", name)
		}
		
	case "absent":
		if exists {
			err := m.removeUser(ctx, conn, name, remove, force)
			if err != nil {
				result.Success = false
				result.Error = err
				return result, nil
			}
			result.Changed = true
			result.Message = fmt.Sprintf("User %s removed", name)
		} else {
			result.Message = fmt.Sprintf("User %s already absent", name)
		}
		
	default:
		result.Success = false
		result.Error = fmt.Errorf("unsupported state: %s", state)
		return result, nil
	}
	
	// Get user info if user exists
	if state == "present" {
		userInfo := m.getUserInfo(ctx, conn, name)
		result.Data["uid"] = userInfo["uid"]
		result.Data["gid"] = userInfo["gid"]
		result.Data["home"] = userInfo["home"]
		result.Data["shell"] = userInfo["shell"]
		result.Data["groups"] = userInfo["groups"]
	}
	
	return result, nil
}

// userExists checks if a user exists
func (m *UserModule) userExists(ctx context.Context, conn types.Connection, name string) bool {
	cmd := fmt.Sprintf("id %s >/dev/null 2>&1", name)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	return err == nil && result.Success
}

// createUser creates a new user
func (m *UserModule) createUser(ctx context.Context, conn types.Connection, name string, uid, gid interface{}, groups, home, shell, password, comment string, createHome, system bool) error {
	cmd := "useradd"
	
	// Add options
	if system {
		cmd += " -r"
	}
	
	if uid != nil {
		uidInt, err := m.toInt(uid)
		if err != nil {
			return fmt.Errorf("invalid uid: %v", err)
		}
		cmd += fmt.Sprintf(" -u %d", uidInt)
	}
	
	if gid != nil {
		gidInt, err := m.toInt(gid)
		if err != nil {
			return fmt.Errorf("invalid gid: %v", err)
		}
		cmd += fmt.Sprintf(" -g %d", gidInt)
	}
	
	if groups != "" {
		cmd += fmt.Sprintf(" -G %s", groups)
	}
	
	if home != "" {
		cmd += fmt.Sprintf(" -d %s", home)
	}
	
	if shell != "" {
		cmd += fmt.Sprintf(" -s %s", shell)
	}
	
	if comment != "" {
		cmd += fmt.Sprintf(" -c '%s'", comment)
	}
	
	if createHome {
		cmd += " -m"
	} else {
		cmd += " -M"
	}
	
	cmd += " " + name
	
	// Create user
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}
	
	// Set password if provided
	if password != "" {
		if err := m.setPassword(ctx, conn, name, password); err != nil {
			return fmt.Errorf("failed to set password: %v", err)
		}
	}
	
	return nil
}

// updateUser updates an existing user
func (m *UserModule) updateUser(ctx context.Context, conn types.Connection, name string, uid, gid interface{}, groups string, appendGroups bool, home, shell, password, comment string) (bool, error) {
	changed := false
	currentInfo := m.getUserInfo(ctx, conn, name)
	
	cmd := "usermod"
	options := []string{}
	
	// Check UID
	if uid != nil {
		uidInt, err := m.toInt(uid)
		if err != nil {
			return false, fmt.Errorf("invalid uid: %v", err)
		}
		currentUID, _ := strconv.Atoi(currentInfo["uid"])
		if uidInt != currentUID {
			options = append(options, fmt.Sprintf("-u %d", uidInt))
			changed = true
		}
	}
	
	// Check GID
	if gid != nil {
		gidInt, err := m.toInt(gid)
		if err != nil {
			return false, fmt.Errorf("invalid gid: %v", err)
		}
		currentGID, _ := strconv.Atoi(currentInfo["gid"])
		if gidInt != currentGID {
			options = append(options, fmt.Sprintf("-g %d", gidInt))
			changed = true
		}
	}
	
	// Check groups
	if groups != "" {
		if appendGroups {
			options = append(options, fmt.Sprintf("-a -G %s", groups))
		} else {
			options = append(options, fmt.Sprintf("-G %s", groups))
		}
		changed = true
	}
	
	// Check home
	if home != "" && home != currentInfo["home"] {
		options = append(options, fmt.Sprintf("-d %s", home))
		changed = true
	}
	
	// Check shell
	if shell != "" && shell != currentInfo["shell"] {
		options = append(options, fmt.Sprintf("-s %s", shell))
		changed = true
	}
	
	// Check comment
	if comment != "" && comment != currentInfo["comment"] {
		options = append(options, fmt.Sprintf("-c '%s'", comment))
		changed = true
	}
	
	// Apply changes if any
	if len(options) > 0 {
		fullCmd := fmt.Sprintf("%s %s %s", cmd, strings.Join(options, " "), name)
		if _, err := conn.Execute(ctx, fullCmd, types.ExecuteOptions{}); err != nil {
			return false, fmt.Errorf("failed to update user: %v", err)
		}
	}
	
	// Update password if provided
	if password != "" {
		if err := m.setPassword(ctx, conn, name, password); err != nil {
			return false, fmt.Errorf("failed to set password: %v", err)
		}
		changed = true
	}
	
	return changed, nil
}

// removeUser removes a user
func (m *UserModule) removeUser(ctx context.Context, conn types.Connection, name string, remove, force bool) error {
	cmd := "userdel"
	
	if remove {
		cmd += " -r"  // Remove home directory
	}
	
	if force {
		cmd += " -f"  // Force removal
	}
	
	cmd += " " + name
	
	if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
		return fmt.Errorf("failed to remove user: %v", err)
	}
	
	return nil
}

// getUserInfo gets information about a user
func (m *UserModule) getUserInfo(ctx context.Context, conn types.Connection, name string) map[string]string {
	info := make(map[string]string)
	
	// Get user info from /etc/passwd
	cmd := fmt.Sprintf("getent passwd %s", name)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err == nil && result.Success {
		parts := strings.Split(strings.TrimSpace(result.Message), ":")
		if len(parts) >= 7 {
			info["uid"] = parts[2]
			info["gid"] = parts[3]
			info["comment"] = parts[4]
			info["home"] = parts[5]
			info["shell"] = parts[6]
		}
	}
	
	// Get groups
	cmd = fmt.Sprintf("groups %s 2>/dev/null | cut -d: -f2", name)
	result, err = conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err == nil && result.Success {
		info["groups"] = strings.TrimSpace(result.Message)
	}
	
	return info
}

// setPassword sets a user's password
func (m *UserModule) setPassword(ctx context.Context, conn types.Connection, name, password string) error {
	// Password should be provided as a hash (e.g., from mkpasswd)
	// If it looks like a plain password, we'll hash it
	if !strings.HasPrefix(password, "$") {
		// Use chpasswd for plain passwords
		cmd := fmt.Sprintf("echo '%s:%s' | chpasswd", name, password)
		if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
			return err
		}
	} else {
		// Use usermod for hashed passwords
		cmd := fmt.Sprintf("usermod -p '%s' %s", password, name)
		if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
			return err
		}
	}
	
	return nil
}

// toInt converts various types to int
func (m *UserModule) toInt(v interface{}) (int, error) {
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
func (m *UserModule) Validate(args map[string]interface{}) error {
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
func (m *UserModule) Documentation() types.ModuleDoc {
	return types.ModuleDoc{
		Name:        "user",
		Description: "Manage user accounts",
		Parameters: map[string]types.ParamDoc{
			"name": {
				Description: "Name of the user",
				Required:    true,
				Type:        "string",
			},
			"state": {
				Description: "State of the user",
				Required:    false,
				Type:        "string",
				Default:     "present",
				Choices:     []string{"present", "absent"},
			},
			"uid": {
				Description: "User ID",
				Required:    false,
				Type:        "int",
			},
			"gid": {
				Description: "Primary group ID",
				Required:    false,
				Type:        "int",
			},
			"groups": {
				Description: "Comma-separated list of groups",
				Required:    false,
				Type:        "string",
			},
			"append": {
				Description: "Append groups to user's existing groups",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"home": {
				Description: "Home directory",
				Required:    false,
				Type:        "string",
			},
			"shell": {
				Description: "Login shell",
				Required:    false,
				Type:        "string",
			},
			"password": {
				Description: "Password (hashed or plain)",
				Required:    false,
				Type:        "string",
			},
			"comment": {
				Description: "GECOS field",
				Required:    false,
				Type:        "string",
			},
			"create_home": {
				Description: "Create home directory",
				Required:    false,
				Type:        "bool",
				Default:     true,
			},
			"system": {
				Description: "Create system user",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"remove": {
				Description: "Remove home directory when removing user",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
			"force": {
				Description: "Force removal of user",
				Required:    false,
				Type:        "bool",
				Default:     false,
			},
		},
		Examples: []string{
			"- name: Create user\n  user:\n    name: john\n    uid: 1001\n    groups: sudo,docker\n    shell: /bin/bash",
			"- name: Remove user\n  user:\n    name: john\n    state: absent\n    remove: true",
			"- name: Create system user\n  user:\n    name: myservice\n    system: true\n    shell: /bin/false",
		},
		Returns: map[string]string{
			"uid":    "User ID",
			"gid":    "Primary group ID",
			"home":   "Home directory",
			"shell":  "Login shell",
			"groups": "User's groups",
		},
	}
}