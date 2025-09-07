package connection

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gosinble/gosinble/pkg/types"
)

// ConnectionPoolConfig holds configuration for connection pooling
type ConnectionPoolConfig struct {
	MaxConnections     int           // Maximum number of connections in pool
	MaxIdleTime        time.Duration // Maximum time a connection can be idle
	ConnectionTimeout  time.Duration // Timeout for establishing connections
	HealthCheckInterval time.Duration // Interval for health checking idle connections
	RetryAttempts      int           // Number of retry attempts for failed connections
	RetryDelay         time.Duration // Delay between retry attempts
}

// DefaultConnectionPoolConfig returns default configuration for connection pooling
func DefaultConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxConnections:      10,
		MaxIdleTime:         5 * time.Minute,
		ConnectionTimeout:   30 * time.Second,
		HealthCheckInterval: 1 * time.Minute,
		RetryAttempts:       3,
		RetryDelay:          1 * time.Second,
	}
}

// PooledConnection wraps a connection with metadata
type PooledConnection struct {
	Connection   types.Connection
	Info         types.ConnectionInfo
	LastUsed     time.Time
	InUse        bool
	HealthCheck  time.Time
	CreatedAt    time.Time
	UseCount     int64
}

// ConnectionPool manages a pool of connections for efficient reuse
type ConnectionPool struct {
	config      ConnectionPoolConfig
	connections map[string][]*PooledConnection // keyed by host:port:user
	mutex       sync.RWMutex
	healthCheck *time.Ticker
	quit        chan bool
}

// NewConnectionPool creates a new connection pool with the given configuration
func NewConnectionPool(config ConnectionPoolConfig) *ConnectionPool {
	pool := &ConnectionPool{
		config:      config,
		connections: make(map[string][]*PooledConnection),
		quit:        make(chan bool),
	}

	// Start background health checker
	pool.healthCheck = time.NewTicker(config.HealthCheckInterval)
	go pool.backgroundHealthCheck()

	return pool
}

// Get retrieves or creates a connection for the given connection info
func (p *ConnectionPool) Get(ctx context.Context, info types.ConnectionInfo) (types.Connection, error) {
	key := p.connectionKey(info)

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Look for available connection in pool
	if conns, exists := p.connections[key]; exists {
		for _, conn := range conns {
			if !conn.InUse && conn.Connection.IsConnected() {
				// Check if connection is too old
				if time.Since(conn.LastUsed) > p.config.MaxIdleTime {
					p.removeConnection(key, conn)
					continue
				}

				// Mark as in use and return
				conn.InUse = true
				conn.LastUsed = time.Now()
				conn.UseCount++
				return conn.Connection, nil
			}
		}
	}

	// No available connection found, create new one
	if p.getTotalConnections() >= p.config.MaxConnections {
		// Try to evict an idle connection
		if !p.evictIdleConnection() {
			return nil, errors.New("connection pool exhausted")
		}
	}

	// Create new connection with retry logic
	var conn types.Connection
	var err error
	
	for attempt := 0; attempt <= p.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(p.config.RetryDelay)
		}

		// Create connection based on type
		if info.IsWindows() {
			conn = NewWinRMConnection()
		} else {
			conn = NewSSHConnection()
		}

		// Set connection timeout
		ctxWithTimeout := ctx
		if p.config.ConnectionTimeout > 0 {
			var cancel context.CancelFunc
			ctxWithTimeout, cancel = context.WithTimeout(ctx, p.config.ConnectionTimeout)
			defer cancel()
		}

		err = conn.Connect(ctxWithTimeout, info)
		if err == nil {
			break
		}

		if attempt < p.config.RetryAttempts {
			// Log retry attempt (in a real implementation, use proper logging)
			fmt.Printf("Connection attempt %d failed for %s: %v, retrying...\n", attempt+1, info.Host, err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to establish connection after %d attempts: %w", p.config.RetryAttempts+1, err)
	}

	// Add to pool
	pooledConn := &PooledConnection{
		Connection:  conn,
		Info:        info,
		LastUsed:    time.Now(),
		InUse:       true,
		HealthCheck: time.Now(),
		CreatedAt:   time.Now(),
		UseCount:    1,
	}

	if _, exists := p.connections[key]; !exists {
		p.connections[key] = make([]*PooledConnection, 0)
	}
	p.connections[key] = append(p.connections[key], pooledConn)

	return conn, nil
}

// Release returns a connection to the pool
func (p *ConnectionPool) Release(conn types.Connection) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Find the connection in the pool
	for key, conns := range p.connections {
		for _, pooledConn := range conns {
			if pooledConn.Connection == conn {
				pooledConn.InUse = false
				pooledConn.LastUsed = time.Now()
				return
			}
		}
		_ = key // avoid unused variable error
	}
}

// Close closes all connections in the pool and stops background tasks
func (p *ConnectionPool) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Stop background health checker
	if p.healthCheck != nil {
		p.healthCheck.Stop()
	}
	
	// Check if quit channel is already closed
	select {
	case <-p.quit:
		// Already closed
	default:
		close(p.quit)
	}

	// Close all connections
	var lastErr error
	for key, conns := range p.connections {
		for _, conn := range conns {
			if err := conn.Connection.Close(); err != nil {
				lastErr = err
			}
		}
		delete(p.connections, key)
	}

	return lastErr
}

// Stats returns statistics about the connection pool
func (p *ConnectionPool) Stats() PoolStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := PoolStats{
		TotalConnections: p.getTotalConnections(),
		ActiveConnections: 0,
		IdleConnections: 0,
		HostStats: make(map[string]HostStats),
	}

	for key, conns := range p.connections {
		hostStats := HostStats{
			TotalConnections: len(conns),
		}

		for _, conn := range conns {
			if conn.InUse {
				stats.ActiveConnections++
				hostStats.ActiveConnections++
			} else {
				stats.IdleConnections++
				hostStats.IdleConnections++
			}
			hostStats.TotalUseCount += conn.UseCount
		}

		stats.HostStats[key] = hostStats
	}

	return stats
}

// PoolStats holds statistics about the connection pool
type PoolStats struct {
	TotalConnections  int
	ActiveConnections int
	IdleConnections   int
	HostStats         map[string]HostStats
}

// HostStats holds statistics for a specific host
type HostStats struct {
	TotalConnections  int
	ActiveConnections int
	IdleConnections   int
	TotalUseCount     int64
}

// connectionKey generates a unique key for connection pooling
func (p *ConnectionPool) connectionKey(info types.ConnectionInfo) string {
	port := info.Port
	if port == 0 {
		if info.IsWindows() {
			if info.UseSSL {
				port = 5986
			} else {
				port = 5985
			}
		} else {
			port = 22
		}
	}
	return fmt.Sprintf("%s:%d:%s", info.Host, port, info.User)
}

// getTotalConnections returns the total number of connections (assumes mutex is held)
func (p *ConnectionPool) getTotalConnections() int {
	total := 0
	for _, conns := range p.connections {
		total += len(conns)
	}
	return total
}

// evictIdleConnection removes the oldest idle connection to make room
func (p *ConnectionPool) evictIdleConnection() bool {
	var oldestConn *PooledConnection
	var oldestKey string
	var oldestIndex int

	for key, conns := range p.connections {
		for i, conn := range conns {
			if !conn.InUse && (oldestConn == nil || conn.LastUsed.Before(oldestConn.LastUsed)) {
				oldestConn = conn
				oldestKey = key
				oldestIndex = i
			}
		}
	}

	if oldestConn != nil {
		p.removeConnection(oldestKey, oldestConn)
		// Remove from slice
		conns := p.connections[oldestKey]
		p.connections[oldestKey] = append(conns[:oldestIndex], conns[oldestIndex+1:]...)
		if len(p.connections[oldestKey]) == 0 {
			delete(p.connections, oldestKey)
		}
		return true
	}

	return false
}

// removeConnection closes and removes a connection from the pool
func (p *ConnectionPool) removeConnection(key string, conn *PooledConnection) {
	conn.Connection.Close()
}

// backgroundHealthCheck periodically checks the health of idle connections
func (p *ConnectionPool) backgroundHealthCheck() {
	for {
		select {
		case <-p.healthCheck.C:
			p.performHealthCheck()
		case <-p.quit:
			return
		}
	}
}

// performHealthCheck checks the health of all idle connections
func (p *ConnectionPool) performHealthCheck() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	now := time.Now()
	for key, conns := range p.connections {
		for i := len(conns) - 1; i >= 0; i-- {
			conn := conns[i]
			
			// Skip connections that are in use or recently checked
			if conn.InUse || now.Sub(conn.HealthCheck) < p.config.HealthCheckInterval {
				continue
			}

			// Check if connection is too old
			if now.Sub(conn.LastUsed) > p.config.MaxIdleTime {
				p.removeConnection(key, conn)
				// Remove from slice
				p.connections[key] = append(conns[:i], conns[i+1:]...)
				continue
			}

			// Ping the connection to check health
			if pinger, ok := conn.Connection.(interface{ Ping() error }); ok {
				if err := pinger.Ping(); err != nil {
					p.removeConnection(key, conn)
					// Remove from slice
					p.connections[key] = append(conns[:i], conns[i+1:]...)
					continue
				}
			}

			conn.HealthCheck = now
		}

		// Clean up empty connection lists
		if len(p.connections[key]) == 0 {
			delete(p.connections, key)
		}
	}
}

// PooledConnectionManager provides high-level connection management with pooling
type PooledConnectionManager struct {
	pool   *ConnectionPool
	config ConnectionPoolConfig
}

// NewPooledConnectionManager creates a new connection manager with connection pooling
func NewPooledConnectionManager(config ConnectionPoolConfig) *PooledConnectionManager {
	return &PooledConnectionManager{
		pool:   NewConnectionPool(config),
		config: config,
	}
}

// NewPooledConnectionManagerWithDefaults creates a connection manager with default settings
func NewPooledConnectionManagerWithDefaults() *PooledConnectionManager {
	return NewPooledConnectionManager(DefaultConnectionPoolConfig())
}

// Connect gets a connection from the pool for the specified host
func (cm *PooledConnectionManager) Connect(ctx context.Context, info types.ConnectionInfo) (types.Connection, error) {
	return cm.pool.Get(ctx, info)
}

// Release returns a connection to the pool
func (cm *PooledConnectionManager) Release(conn types.Connection) {
	cm.pool.Release(conn)
}

// ExecuteOnHost executes a command on a specific host, handling connection automatically
func (cm *PooledConnectionManager) ExecuteOnHost(ctx context.Context, info types.ConnectionInfo, command string, options types.ExecuteOptions) (*types.Result, error) {
	conn, err := cm.Connect(ctx, info)
	if err != nil {
		return nil, err
	}
	defer cm.Release(conn)

	return conn.Execute(ctx, command, options)
}

// CopyToHost copies a file to a specific host, handling connection automatically
func (cm *PooledConnectionManager) CopyToHost(ctx context.Context, info types.ConnectionInfo, src io.Reader, dest string, mode int) error {
	conn, err := cm.Connect(ctx, info)
	if err != nil {
		return err
	}
	defer cm.Release(conn)

	return conn.Copy(ctx, src, dest, mode)
}

// FetchFromHost fetches a file from a specific host, handling connection automatically
func (cm *PooledConnectionManager) FetchFromHost(ctx context.Context, info types.ConnectionInfo, src string) (io.Reader, error) {
	conn, err := cm.Connect(ctx, info)
	if err != nil {
		return nil, err
	}
	defer cm.Release(conn)

	return conn.Fetch(ctx, src)
}

// Close closes the connection manager and all pooled connections
func (cm *PooledConnectionManager) Close() error {
	return cm.pool.Close()
}

// Stats returns connection pool statistics
func (cm *PooledConnectionManager) Stats() PoolStats {
	return cm.pool.Stats()
}

// Global connection manager instance
var defaultPooledConnectionManager *PooledConnectionManager
var initOnce sync.Once

// GetDefaultPooledConnectionManager returns the global connection manager instance
func GetDefaultPooledConnectionManager() *PooledConnectionManager {
	initOnce.Do(func() {
		defaultPooledConnectionManager = NewPooledConnectionManagerWithDefaults()
	})
	return defaultPooledConnectionManager
}