package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// IPTablesModule manages iptables firewall rules
type IPTablesModule struct {
	BaseModule
}

// NewIPTablesModule creates a new iptables module instance
func NewIPTablesModule() *IPTablesModule {
	return &IPTablesModule{
		BaseModule: BaseModule{},
	}
}

// Name returns the module name
func (m *IPTablesModule) Name() string {
	return "iptables"
}

// Capabilities returns the module capabilities
func (m *IPTablesModule) Capabilities() *types.ModuleCapability {
	return &types.ModuleCapability{
		CheckMode:    true,
		DiffMode:     false, // iptables doesn't really have diff mode
		Platform:     "linux",
		RequiresRoot: true,
	}
}

// Validate validates the module arguments
func (m *IPTablesModule) Validate(args map[string]interface{}) error {
	// State validation
	state := m.GetStringArg(args, "state", "present")
	validStates := []string{"present", "absent"}
	if !m.isValidChoice(state, validStates) {
		return types.NewValidationError("state", state, fmt.Sprintf("state must be one of: %v", validStates))
	}

	// Action validation (for state=present)
	if state == "present" {
		action := m.GetStringArg(args, "action", "")
		jump := m.GetStringArg(args, "jump", "")
		
		if action == "" && jump == "" {
			return types.NewValidationError("action/jump", nil, "either action or jump is required when state is present")
		}
		
		if action != "" {
			validActions := []string{"append", "insert"}
			if !m.isValidChoice(action, validActions) {
				return types.NewValidationError("action", action, fmt.Sprintf("action must be one of: %v", validActions))
			}
		}
	}

	// Chain validation
	chain := m.GetStringArg(args, "chain", "")
	if chain == "" {
		return types.NewValidationError("chain", nil, "chain is required")
	}

	// Table validation
	table := m.GetStringArg(args, "table", "filter")
	validTables := []string{"filter", "nat", "mangle", "raw", "security"}
	if !m.isValidChoice(table, validTables) {
		return types.NewValidationError("table", table, fmt.Sprintf("table must be one of: %v", validTables))
	}

	// Protocol validation
	protocol := m.GetStringArg(args, "protocol", "")
	if protocol != "" {
		validProtocols := []string{"tcp", "udp", "icmp", "esp", "ah", "sctp", "all"}
		if !m.isValidChoice(protocol, validProtocols) && !strings.HasPrefix(protocol, "!") {
			// Check if it's a numeric protocol
			if _, err := strconv.Atoi(protocol); err != nil {
				return types.NewValidationError("protocol", protocol, fmt.Sprintf("protocol must be one of: %v or a protocol number", validProtocols))
			}
		}
	}

	// IP version validation
	ipVersion := m.GetStringArg(args, "ip_version", "ipv4")
	validIPVersions := []string{"ipv4", "ipv6"}
	if !m.isValidChoice(ipVersion, validIPVersions) {
		return types.NewValidationError("ip_version", ipVersion, fmt.Sprintf("ip_version must be one of: %v", validIPVersions))
	}

	return nil
}

// Run executes the iptables module
func (m *IPTablesModule) Run(ctx context.Context, conn types.Connection, args map[string]interface{}) (*types.Result, error) {
	startTime := time.Now()
	hostname := m.GetHostFromConnection(conn)
	checkMode := m.CheckMode(args)

	// Parse arguments
	table := m.GetStringArg(args, "table", "filter")
	chain := m.GetStringArg(args, "chain", "")
	protocol := m.GetStringArg(args, "protocol", "")
	source := m.GetStringArg(args, "source", "")
	destination := m.GetStringArg(args, "destination", "")
	jump := m.GetStringArg(args, "jump", "")
	goto_ := m.GetStringArg(args, "goto", "")
	inInterface := m.GetStringArg(args, "in_interface", "")
	outInterface := m.GetStringArg(args, "out_interface", "")
	fragment := m.GetBoolArg(args, "fragment", false)
	setCounters := m.GetStringArg(args, "set_counters", "")
	sourcePort := m.GetStringArg(args, "source_port", "")
	destinationPort := m.GetStringArg(args, "destination_port", "")
	toSource := m.GetStringArg(args, "to_source", "")
	toDestination := m.GetStringArg(args, "to_destination", "")
	toPorts := m.GetStringArg(args, "to_ports", "")
	comment := m.GetStringArg(args, "comment", "")
	state := m.GetStringArg(args, "state", "present")
	action := m.GetStringArg(args, "action", "append")
	ruleNum, _ := m.GetIntArg(args, "rule_num", 0)
	ipVersion := m.GetStringArg(args, "ip_version", "ipv4")
	limit := m.GetStringArg(args, "limit", "")
	limitBurst := m.GetStringArg(args, "limit_burst", "")
	uid := m.GetStringArg(args, "uid_owner", "")
	gid := m.GetStringArg(args, "gid_owner", "")
	syn := m.GetBoolArg(args, "syn", false)
	ctstate := m.GetStringArg(args, "ctstate", "")
	icmpType := m.GetStringArg(args, "icmp_type", "")
	reject := m.GetStringArg(args, "reject_with", "")
	logPrefix := m.GetStringArg(args, "log_prefix", "")
	logLevel := m.GetStringArg(args, "log_level", "")
	tcpFlags := m.GetStringArg(args, "tcp_flags", "")
	flush := m.GetBoolArg(args, "flush", false)
	policy := m.GetStringArg(args, "policy", "")

	// Determine iptables command based on IP version
	iptablesCmd := "iptables"
	if ipVersion == "ipv6" {
		iptablesCmd = "ip6tables"
	}

	// Build the iptables rule
	rule := m.buildRule(table, chain, protocol, source, destination, jump, goto_,
		inInterface, outInterface, fragment, setCounters, sourcePort, destinationPort,
		toSource, toDestination, toPorts, comment, limit, limitBurst, uid, gid,
		syn, ctstate, icmpType, reject, logPrefix, logLevel, tcpFlags)

	// Initialize result
	result := m.CreateSuccessResult(hostname, false, "", map[string]interface{}{
		"table":      table,
		"chain":      chain,
		"rule":       rule,
		"state":      state,
		"ip_version": ipVersion,
	})

	changed := false
	var message string

	// Handle flush operation
	if flush {
		if checkMode {
			result.Changed = true
			result.Message = fmt.Sprintf("Would flush chain %s in table %s", chain, table)
			return result, nil
		}

		cmd := fmt.Sprintf("%s -t %s -F %s", iptablesCmd, table, chain)
		if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
			return nil, fmt.Errorf("failed to flush chain: %w", err)
		}
		
		result.Changed = true
		result.Message = fmt.Sprintf("Flushed chain %s in table %s", chain, table)
		return result, nil
	}

	// Handle policy change
	if policy != "" {
		currentPolicy, err := m.getChainPolicy(ctx, conn, iptablesCmd, table, chain)
		if err != nil {
			return nil, fmt.Errorf("failed to get current policy: %w", err)
		}

		if currentPolicy != strings.ToUpper(policy) {
			changed = true
			message = fmt.Sprintf("Changed policy of %s from %s to %s", chain, currentPolicy, strings.ToUpper(policy))
			
			if !checkMode {
				cmd := fmt.Sprintf("%s -t %s -P %s %s", iptablesCmd, table, chain, strings.ToUpper(policy))
				if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
					return nil, fmt.Errorf("failed to set policy: %w", err)
				}
			}
		}
		
		result.Changed = changed
		if changed {
			result.Message = message
		} else {
			result.Message = fmt.Sprintf("Policy already set to %s", strings.ToUpper(policy))
		}
		return result, nil
	}

	// Check if rule exists
	ruleExists, err := m.ruleExists(ctx, conn, iptablesCmd, table, chain, rule)
	if err != nil {
		return nil, fmt.Errorf("failed to check if rule exists: %w", err)
	}

	if state == "present" {
		if !ruleExists {
			changed = true
			message = fmt.Sprintf("Added iptables rule to %s chain", chain)
			
			if !checkMode {
				var cmd string
				if action == "insert" && ruleNum > 0 {
					cmd = fmt.Sprintf("%s -t %s -I %s %d %s", iptablesCmd, table, chain, ruleNum, rule)
				} else if action == "insert" {
					cmd = fmt.Sprintf("%s -t %s -I %s %s", iptablesCmd, table, chain, rule)
				} else {
					cmd = fmt.Sprintf("%s -t %s -A %s %s", iptablesCmd, table, chain, rule)
				}
				
				if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
					return nil, fmt.Errorf("failed to add rule: %w", err)
				}
			}
		} else {
			message = "Rule already exists"
		}
	} else { // state == "absent"
		if ruleExists {
			changed = true
			message = fmt.Sprintf("Removed iptables rule from %s chain", chain)
			
			if !checkMode {
				cmd := fmt.Sprintf("%s -t %s -D %s %s", iptablesCmd, table, chain, rule)
				if _, err := conn.Execute(ctx, cmd, types.ExecuteOptions{}); err != nil {
					return nil, fmt.Errorf("failed to remove rule: %w", err)
				}
			}
		} else {
			message = "Rule does not exist"
		}
	}

	result.Changed = changed
	result.Message = message

	// Set timing information
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Helper methods

func (m *IPTablesModule) buildRule(table, chain, protocol, source, destination, jump, goto_,
	inInterface, outInterface string, fragment bool, setCounters, sourcePort, destinationPort,
	toSource, toDestination, toPorts, comment, limit, limitBurst, uid, gid string,
	syn bool, ctstate, icmpType, reject, logPrefix, logLevel, tcpFlags string) string {
	
	var parts []string

	// Protocol
	if protocol != "" {
		parts = append(parts, "-p", protocol)
	}

	// Source
	if source != "" {
		parts = append(parts, "-s", source)
	}

	// Destination
	if destination != "" {
		parts = append(parts, "-d", destination)
	}

	// Interfaces
	if inInterface != "" {
		parts = append(parts, "-i", inInterface)
	}
	if outInterface != "" {
		parts = append(parts, "-o", outInterface)
	}

	// Fragment
	if fragment {
		parts = append(parts, "-f")
	}

	// Ports (TCP/UDP specific)
	if sourcePort != "" && (protocol == "tcp" || protocol == "udp") {
		parts = append(parts, "--sport", sourcePort)
	}
	if destinationPort != "" && (protocol == "tcp" || protocol == "udp") {
		parts = append(parts, "--dport", destinationPort)
	}

	// TCP flags
	if syn && protocol == "tcp" {
		parts = append(parts, "--syn")
	}
	if tcpFlags != "" && protocol == "tcp" {
		flagParts := strings.Split(tcpFlags, " ")
		if len(flagParts) == 2 {
			parts = append(parts, "--tcp-flags", flagParts[0], flagParts[1])
		}
	}

	// ICMP type
	if icmpType != "" && protocol == "icmp" {
		parts = append(parts, "--icmp-type", icmpType)
	}

	// Connection state
	if ctstate != "" {
		parts = append(parts, "-m", "conntrack", "--ctstate", ctstate)
	}

	// Limit
	if limit != "" {
		parts = append(parts, "-m", "limit", "--limit", limit)
		if limitBurst != "" {
			parts = append(parts, "--limit-burst", limitBurst)
		}
	}

	// Owner
	if uid != "" || gid != "" {
		parts = append(parts, "-m", "owner")
		if uid != "" {
			parts = append(parts, "--uid-owner", uid)
		}
		if gid != "" {
			parts = append(parts, "--gid-owner", gid)
		}
	}

	// Comment
	if comment != "" {
		parts = append(parts, "-m", "comment", "--comment", fmt.Sprintf("\"%s\"", comment))
	}

	// NAT specific
	if toSource != "" {
		parts = append(parts, "--to-source", toSource)
	}
	if toDestination != "" {
		parts = append(parts, "--to-destination", toDestination)
	}
	if toPorts != "" {
		parts = append(parts, "--to-ports", toPorts)
	}

	// Logging
	if logPrefix != "" {
		parts = append(parts, "--log-prefix", fmt.Sprintf("\"%s\"", logPrefix))
	}
	if logLevel != "" {
		parts = append(parts, "--log-level", logLevel)
	}

	// Reject with
	if reject != "" {
		parts = append(parts, "--reject-with", reject)
	}

	// Jump/Goto
	if jump != "" {
		parts = append(parts, "-j", jump)
	} else if goto_ != "" {
		parts = append(parts, "-g", goto_)
	}

	return strings.Join(parts, " ")
}

func (m *IPTablesModule) ruleExists(ctx context.Context, conn types.Connection, iptablesCmd, table, chain, rule string) (bool, error) {
	// Use -C (check) to see if rule exists
	cmd := fmt.Sprintf("%s -t %s -C %s %s 2>/dev/null", iptablesCmd, table, chain, rule)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		// Command returns non-zero if rule doesn't exist
		if result != nil && !result.Success {
			return false, nil
		}
		return false, err
	}
	return result.Success, nil
}

func (m *IPTablesModule) getChainPolicy(ctx context.Context, conn types.Connection, iptablesCmd, table, chain string) (string, error) {
	// List the chain and extract policy
	cmd := fmt.Sprintf("%s -t %s -L %s -n | head -1", iptablesCmd, table, chain)
	result, err := conn.Execute(ctx, cmd, types.ExecuteOptions{})
	if err != nil {
		return "", err
	}

	// Parse output like "Chain INPUT (policy ACCEPT)"
	output := ""
	if stdout, ok := result.Data["stdout"].(string); ok {
		output = stdout
	}
	if strings.Contains(output, "policy") {
		parts := strings.Split(output, "policy ")
		if len(parts) > 1 {
			policyPart := strings.TrimSpace(parts[1])
			policyPart = strings.TrimSuffix(policyPart, ")")
			return policyPart, nil
		}
	}

	return "", fmt.Errorf("could not determine chain policy")
}

func (m *IPTablesModule) isValidChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}