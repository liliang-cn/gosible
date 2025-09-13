package connection

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/liliang-cn/gosible/pkg/types"
)

// MockConnection implements types.Connection for testing
type MockConnection struct {
	connected   bool
	host        string
	pingErr     error
	executeFunc func(ctx context.Context, command string, options types.ExecuteOptions) (*types.Result, error)
}

func (m *MockConnection) Connect(ctx context.Context, info types.ConnectionInfo) error {
	m.connected = true
	m.host = info.Host
	return nil
}

func (m *MockConnection) Execute(ctx context.Context, command string, options types.ExecuteOptions) (*types.Result, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, command, options)
	}
	return &types.Result{
		Success: true,
		Host:    m.host,
		Data:    map[string]interface{}{"stdout": "mock output"},
	}, nil
}

func (m *MockConnection) ExecuteStream(ctx context.Context, command string, options types.ExecuteOptions) (<-chan types.StreamEvent, error) {
	ch := make(chan types.StreamEvent, 1)
	go func() {
		defer close(ch)
		ch <- types.StreamEvent{
			Type: types.StreamDone,
			Result: &types.Result{
				Success: true,
				Host:    m.host,
				Data:    map[string]interface{}{"stdout": "mock output"},
			},
		}
	}()
	return ch, nil
}

func (m *MockConnection) Copy(ctx context.Context, src io.Reader, dest string, mode int) error {
	return nil
}

func (m *MockConnection) Fetch(ctx context.Context, src string) (io.Reader, error) {
	return nil, nil
}

func (m *MockConnection) Close() error {
	m.connected = false
	return nil
}

func (m *MockConnection) IsConnected() bool {
	return m.connected
}

func (m *MockConnection) Ping() error {
	return m.pingErr
}

func TestConnectionPool_Get(t *testing.T) {
	t.Skip("Skipping test that attempts real network connection")
	
	// This test attempts to connect to a non-existent host which can be slow
	// due to DNS lookups and timeouts. In a real test environment, we would
	// use a mock connection factory instead.
}

func TestConnectionPool_Stats(t *testing.T) {
	config := DefaultConnectionPoolConfig()
	pool := NewConnectionPool(config)
	defer pool.Close()
	
	stats := pool.Stats()
	if stats.TotalConnections != 0 {
		t.Errorf("expected 0 total connections, got %d", stats.TotalConnections)
	}
	
	if stats.ActiveConnections != 0 {
		t.Errorf("expected 0 active connections, got %d", stats.ActiveConnections)
	}
	
	if stats.IdleConnections != 0 {
		t.Errorf("expected 0 idle connections, got %d", stats.IdleConnections)
	}
}

func TestConnectionPool_ConnectionKey(t *testing.T) {
	pool := NewConnectionPool(DefaultConnectionPoolConfig())
	defer pool.Close()
	
	tests := []struct {
		name     string
		info     types.ConnectionInfo
		expected string
	}{
		{
			name: "SSH default port",
			info: types.ConnectionInfo{
				Host: "host1",
				User: "user1",
			},
			expected: "host1:22:user1",
		},
		{
			name: "SSH custom port",
			info: types.ConnectionInfo{
				Host: "host1",
				User: "user1",
				Port: 2222,
			},
			expected: "host1:2222:user1",
		},
		{
			name: "WinRM HTTP default port",
			info: types.ConnectionInfo{
				Type: "winrm",
				Host: "winhost",
				User: "winuser",
			},
			expected: "winhost:5985:winuser",
		},
		{
			name: "WinRM HTTPS default port",
			info: types.ConnectionInfo{
				Type:   "winrm",
				Host:   "winhost",
				User:   "winuser",
				UseSSL: true,
			},
			expected: "winhost:5986:winuser",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := pool.connectionKey(tt.info)
			if key != tt.expected {
				t.Errorf("expected key %q, got %q", tt.expected, key)
			}
		})
	}
}

func TestConnectionPool_Close(t *testing.T) {
	pool := NewConnectionPool(DefaultConnectionPoolConfig())
	
	err := pool.Close()
	if err != nil {
		t.Errorf("unexpected error closing pool: %v", err)
	}
	
	// Closing again should not error
	err = pool.Close()
	if err != nil {
		t.Errorf("unexpected error closing pool twice: %v", err)
	}
}

func TestPooledConnectionManager_NewPooledConnectionManager(t *testing.T) {
	config := DefaultConnectionPoolConfig()
	manager := NewPooledConnectionManager(config)
	
	if manager.config.MaxConnections != config.MaxConnections {
		t.Errorf("expected MaxConnections %d, got %d", config.MaxConnections, manager.config.MaxConnections)
	}
	
	err := manager.Close()
	if err != nil {
		t.Errorf("unexpected error closing manager: %v", err)
	}
}

func TestPooledConnectionManager_NewPooledConnectionManagerWithDefaults(t *testing.T) {
	manager := NewPooledConnectionManagerWithDefaults()
	
	defaultConfig := DefaultConnectionPoolConfig()
	if manager.config.MaxConnections != defaultConfig.MaxConnections {
		t.Errorf("expected MaxConnections %d, got %d", defaultConfig.MaxConnections, manager.config.MaxConnections)
	}
	
	err := manager.Close()
	if err != nil {
		t.Errorf("unexpected error closing manager: %v", err)
	}
}

func TestPooledConnectionManager_ExecuteOnHost(t *testing.T) {
	t.Skip("Skipping test that attempts real network connection")
	
	// This test attempts to connect to a non-existent host which can be slow.
	// In a real test environment, we would use a mock connection factory.
}

func TestPooledConnectionManager_Stats(t *testing.T) {
	manager := NewPooledConnectionManagerWithDefaults()
	defer manager.Close()
	
	stats := manager.Stats()
	if stats.TotalConnections != 0 {
		t.Errorf("expected 0 total connections, got %d", stats.TotalConnections)
	}
}

func TestGetDefaultPooledConnectionManager(t *testing.T) {
	// Note: This test uses a singleton which persists across tests.
	// We need to clean it up to prevent goroutine leaks.
	manager1 := GetDefaultPooledConnectionManager()
	manager2 := GetDefaultPooledConnectionManager()
	
	// Should be the same instance (singleton)
	if manager1 != manager2 {
		t.Error("expected same instance from GetDefaultPooledConnectionManager")
	}
	
	// Clean up the singleton to prevent goroutine leaks
	if manager1 != nil {
		manager1.Close()
	}
	// Reset the singleton so next test gets a fresh instance
	defaultPooledConnectionManager = nil
	initOnce = sync.Once{}
}

func TestDefaultConnectionPoolConfig(t *testing.T) {
	config := DefaultConnectionPoolConfig()
	
	if config.MaxConnections <= 0 {
		t.Error("expected positive MaxConnections")
	}
	
	if config.MaxIdleTime <= 0 {
		t.Error("expected positive MaxIdleTime")
	}
	
	if config.ConnectionTimeout <= 0 {
		t.Error("expected positive ConnectionTimeout")
	}
	
	if config.HealthCheckInterval <= 0 {
		t.Error("expected positive HealthCheckInterval")
	}
	
	if config.RetryAttempts < 0 {
		t.Error("expected non-negative RetryAttempts")
	}
	
	if config.RetryDelay < 0 {
		t.Error("expected non-negative RetryDelay")
	}
}

func TestPooledConnection(t *testing.T) {
	mockConn := &MockConnection{}
	info := types.ConnectionInfo{Host: "testhost", User: "testuser"}
	
	pooledConn := &PooledConnection{
		Connection:  mockConn,
		Info:        info,
		LastUsed:    time.Now(),
		InUse:       false,
		HealthCheck: time.Now(),
		CreatedAt:   time.Now(),
		UseCount:    0,
	}
	
	if pooledConn.Connection != mockConn {
		t.Error("expected connection to match")
	}
	
	if pooledConn.Info.Host != info.Host {
		t.Error("expected info to match")
	}
	
	if pooledConn.InUse {
		t.Error("expected InUse to be false")
	}
	
	if pooledConn.UseCount != 0 {
		t.Error("expected UseCount to be 0")
	}
}

func TestHostStats(t *testing.T) {
	stats := HostStats{
		TotalConnections:  3,
		ActiveConnections: 1,
		IdleConnections:   2,
		TotalUseCount:     10,
	}
	
	if stats.TotalConnections != 3 {
		t.Errorf("expected TotalConnections 3, got %d", stats.TotalConnections)
	}
	
	if stats.ActiveConnections != 1 {
		t.Errorf("expected ActiveConnections 1, got %d", stats.ActiveConnections)
	}
	
	if stats.IdleConnections != 2 {
		t.Errorf("expected IdleConnections 2, got %d", stats.IdleConnections)
	}
	
	if stats.TotalUseCount != 10 {
		t.Errorf("expected TotalUseCount 10, got %d", stats.TotalUseCount)
	}
}

func TestPoolStats(t *testing.T) {
	stats := PoolStats{
		TotalConnections:  5,
		ActiveConnections: 2,
		IdleConnections:   3,
		HostStats:         make(map[string]HostStats),
	}
	
	stats.HostStats["host1"] = HostStats{
		TotalConnections:  2,
		ActiveConnections: 1,
		IdleConnections:   1,
		TotalUseCount:     5,
	}
	
	if stats.TotalConnections != 5 {
		t.Errorf("expected TotalConnections 5, got %d", stats.TotalConnections)
	}
	
	if stats.ActiveConnections != 2 {
		t.Errorf("expected ActiveConnections 2, got %d", stats.ActiveConnections)
	}
	
	if stats.IdleConnections != 3 {
		t.Errorf("expected IdleConnections 3, got %d", stats.IdleConnections)
	}
	
	hostStats, exists := stats.HostStats["host1"]
	if !exists {
		t.Error("expected host1 stats to exist")
	}
	
	if hostStats.TotalConnections != 2 {
		t.Errorf("expected host1 TotalConnections 2, got %d", hostStats.TotalConnections)
	}
}

// Benchmark tests
func BenchmarkConnectionPool_ConnectionKey(b *testing.B) {
	pool := NewConnectionPool(DefaultConnectionPoolConfig())
	defer pool.Close()
	
	info := types.ConnectionInfo{
		Host: "testhost",
		User: "testuser",
		Port: 22,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.connectionKey(info)
	}
}

func BenchmarkConnectionPool_Stats(b *testing.B) {
	pool := NewConnectionPool(DefaultConnectionPoolConfig())
	defer pool.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Stats()
	}
}