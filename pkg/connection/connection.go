// Package connection provides connection plugins for executing commands on remote hosts.
package connection

import (
	"context"
	"fmt"

	"github.com/liliang-cn/gosinble/pkg/types"
)

// ConnectionType defines the type of connection
type ConnectionType string

const (
	ConnectionTypeLocal ConnectionType = "local"
	ConnectionTypeSSH   ConnectionType = "ssh"
)

// ConnectionManager manages connection plugins
type ConnectionManager struct {
	plugins map[ConnectionType]ConnectionFactory
}

// ConnectionFactory creates connection instances
type ConnectionFactory func() types.Connection

// NewConnectionManager creates a new connection manager
func NewConnectionManager() *ConnectionManager {
	manager := &ConnectionManager{
		plugins: make(map[ConnectionType]ConnectionFactory),
	}

	// Register built-in connection plugins
	manager.RegisterPlugin(ConnectionTypeLocal, func() types.Connection {
		return NewLocalConnection()
	})
	manager.RegisterPlugin(ConnectionTypeSSH, func() types.Connection {
		return NewSSHConnection()
	})

	return manager
}

// RegisterPlugin registers a connection plugin
func (cm *ConnectionManager) RegisterPlugin(connType ConnectionType, factory ConnectionFactory) {
	cm.plugins[connType] = factory
}

// CreateConnection creates a connection instance for the given type
func (cm *ConnectionManager) CreateConnection(connType ConnectionType) (types.Connection, error) {
	factory, exists := cm.plugins[connType]
	if !exists {
		return nil, fmt.Errorf("unsupported connection type: %s", connType)
	}

	return factory(), nil
}

// GetConnection creates and connects to a host
func (cm *ConnectionManager) GetConnection(ctx context.Context, info types.ConnectionInfo) (types.Connection, error) {
	connType := ConnectionType(info.Type)
	if connType == "" {
		connType = ConnectionTypeSSH // default
	}

	conn, err := cm.CreateConnection(connType)
	if err != nil {
		return nil, err
	}

	if err := conn.Connect(ctx, info); err != nil {
		return nil, err
	}

	return conn, nil
}

// ListPlugins returns all registered connection plugin types
func (cm *ConnectionManager) ListPlugins() []ConnectionType {
	var types []ConnectionType
	for t := range cm.plugins {
		types = append(types, t)
	}
	return types
}

// DefaultConnectionManager provides a default connection manager instance
var DefaultConnectionManager = NewConnectionManager()