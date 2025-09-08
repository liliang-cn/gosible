package modules

import (
	"context"
	"io"
	"testing"
	
	"github.com/liliang-cn/gosinble/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockConnection is a mock implementation of types.Connection
type MockConnection struct {
	mock.Mock
}

func (m *MockConnection) Connect(ctx context.Context, info types.ConnectionInfo) error {
	args := m.Called(ctx, info)
	return args.Error(0)
}

func (m *MockConnection) Execute(ctx context.Context, command string, options types.ExecuteOptions) (*types.Result, error) {
	args := m.Called(ctx, command, options)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Result), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockConnection) Copy(ctx context.Context, src io.Reader, dest string, mode int) error {
	args := m.Called(ctx, src, dest, mode)
	return args.Error(0)
}

func (m *MockConnection) Fetch(ctx context.Context, src string) (io.Reader, error) {
	args := m.Called(ctx, src)
	if args.Get(0) != nil {
		return args.Get(0).(io.Reader), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockConnection) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConnection) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestFileModule_Validate(t *testing.T) {
	module := NewFileModule()
	
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
			errMsg:  "required field is missing",
		},
		{
			name: "valid file state",
			args: map[string]interface{}{
				"path":  "/tmp/test",
				"state": "file",
			},
			wantErr: false,
		},
		{
			name: "valid directory state",
			args: map[string]interface{}{
				"path":  "/tmp/test",
				"state": "directory",
			},
			wantErr: false,
		},
		{
			name: "link state without src",
			args: map[string]interface{}{
				"path":  "/tmp/test",
				"state": "link",
			},
			wantErr: true,
			errMsg:  "required when state=link",
		},
		{
			name: "link state with src",
			args: map[string]interface{}{
				"path":  "/tmp/test",
				"state": "link",
				"src":   "/tmp/source",
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			args: map[string]interface{}{
				"path":  "/tmp/test",
				"state": "invalid",
			},
			wantErr: true,
			errMsg:  "must be one of",
		},
		{
			name: "invalid mode",
			args: map[string]interface{}{
				"path": "/tmp/test",
				"mode": "invalid",
			},
			wantErr: true,
			errMsg:  "must be an octal number",
		},
		{
			name: "valid mode",
			args: map[string]interface{}{
				"path": "/tmp/test",
				"mode": "0755",
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := module.Validate(tt.args)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFileModule_Run_CreateDirectory(t *testing.T) {
	module := NewFileModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	// Test creating a new directory
	args := map[string]interface{}{
		"path":  "/tmp/testdir",
		"state": "directory",
		"mode":  "0755",
	}
	
	// Mock: Check if directory exists
	mockConn.On("Execute", ctx, "test -e /tmp/testdir && echo EXISTS || echo NOTEXISTS", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "NOTEXISTS",
	}, nil)
	
	// Mock: Create directory
	mockConn.On("Execute", ctx, "mkdir -p /tmp/testdir", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	// Mock: Set permissions
	mockConn.On("Execute", ctx, "chmod 0755 /tmp/testdir", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "Directory created", result.Message)
	
	mockConn.AssertExpectations(t)
}

func TestFileModule_Run_CreateFile(t *testing.T) {
	module := NewFileModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	// Test creating a new file
	args := map[string]interface{}{
		"path":  "/tmp/testfile",
		"state": "file",
	}
	
	// Mock: Check if file exists
	mockConn.On("Execute", ctx, "test -e /tmp/testfile && echo EXISTS || echo NOTEXISTS", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "NOTEXISTS",
	}, nil)
	
	// Mock: Create file
	mockConn.On("Execute", ctx, "touch /tmp/testfile", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "File created", result.Message)
	
	mockConn.AssertExpectations(t)
}

func TestFileModule_Run_CreateSymlink(t *testing.T) {
	module := NewFileModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	// Test creating a symlink
	args := map[string]interface{}{
		"path":  "/tmp/testlink",
		"src":   "/tmp/source",
		"state": "link",
	}
	
	// Mock: Check if link exists
	mockConn.On("Execute", ctx, "test -e /tmp/testlink && echo EXISTS || echo NOTEXISTS", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "NOTEXISTS",
	}, nil)
	
	// Mock: Create symlink
	mockConn.On("Execute", ctx, "ln -s /tmp/source /tmp/testlink", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "Link created", result.Message)
	
	mockConn.AssertExpectations(t)
}

func TestFileModule_Run_RemoveFile(t *testing.T) {
	module := NewFileModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	// Test removing a file
	args := map[string]interface{}{
		"path":  "/tmp/testfile",
		"state": "absent",
	}
	
	// Mock: Check if file exists
	mockConn.On("Execute", ctx, "test -e /tmp/testfile && echo EXISTS || echo NOTEXISTS", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "EXISTS",
	}, nil)
	
	// Mock: Remove file
	mockConn.On("Execute", ctx, "rm -rf /tmp/testfile", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.True(t, result.Changed)
	assert.Equal(t, "Path removed", result.Message)
	
	mockConn.AssertExpectations(t)
}

func TestFileModule_Run_FileAlreadyExists(t *testing.T) {
	module := NewFileModule()
	ctx := context.Background()
	mockConn := new(MockConnection)
	
	// Test when file already exists
	args := map[string]interface{}{
		"path":  "/tmp/testfile",
		"state": "file",
	}
	
	// Mock: Check if file exists
	mockConn.On("Execute", ctx, "test -e /tmp/testfile && echo EXISTS || echo NOTEXISTS", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "EXISTS",
	}, nil)
	
	// Mock: Check if it's a file
	mockConn.On("Execute", ctx, "test -f /tmp/testfile && echo FILE || echo NOTFILE", 
		types.ExecuteOptions{}).Return(&types.Result{
		Success: true,
		Message: "FILE",
	}, nil)
	
	result, err := module.Run(ctx, mockConn, args)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.False(t, result.Changed)
	assert.Equal(t, "File already exists", result.Message)
	
	mockConn.AssertExpectations(t)
}