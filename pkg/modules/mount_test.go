package modules

import (
	"context"
	"testing"

	gotest "github.com/gosinble/gosinble/pkg/testing"
)

func TestMountModule(t *testing.T) {
	t.Run("ModuleProperties", func(t *testing.T) {
		m := NewMountModule()
		if m.Name() != "mount" {
			t.Errorf("Expected module name 'mount', got %s", m.Name())
		}
	})

	t.Run("ValidationTests", func(t *testing.T) {
		m := NewMountModule()
		
		testCases := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name: "ValidMount",
				args: map[string]interface{}{
					"path":   "/mnt/data",
					"src":    "/dev/sdb1",
					"fstype": "ext4",
					"state":  "mounted",
				},
				wantErr: false,
			},
			{
				name: "ValidUnmount",
				args: map[string]interface{}{
					"path":  "/mnt/data",
					"state": "unmounted",
				},
				wantErr: false,
			},
			{
				name: "ValidRemount",
				args: map[string]interface{}{
					"path":  "/mnt/data",
					"state": "remounted",
				},
				wantErr: false,
			},
			{
				name: "ValidAbsent",
				args: map[string]interface{}{
					"path":  "/mnt/data",
					"state": "absent",
				},
				wantErr: false,
			},
			{
				name: "MissingPath",
				args: map[string]interface{}{
					"src":    "/dev/sdb1",
					"fstype": "ext4",
					"state":  "mounted",
				},
				wantErr: true,
			},
			{
				name: "MissingSrcForMount",
				args: map[string]interface{}{
					"path":   "/mnt/data",
					"fstype": "ext4",
					"state":  "mounted",
				},
				wantErr: true,
			},
			{
				name: "InvalidState",
				args: map[string]interface{}{
					"path":  "/mnt/data",
					"state": "invalid",
				},
				wantErr: true,
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := m.Validate(tc.args)
				if (err != nil) != tc.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
				}
			})
		}
	})

	t.Run("MountOperationTests", func(t *testing.T) {
		m := NewMountModule()
		helper := gotest.NewModuleTestHelper(t, m)
		conn := helper.GetConnection()
		ctx := context.Background()
		
		t.Run("MountFilesystem", func(t *testing.T) {
			// Check if already mounted
			conn.ExpectCommand("mount | grep '/mnt/data'", &gotest.CommandResponse{
				ExitCode: 1, // Not mounted
			})
			
			// Create mount point
			conn.ExpectCommand("mkdir -p /mnt/data", &gotest.CommandResponse{
				ExitCode: 0,
			})
			
			// Mount the filesystem
			conn.ExpectCommand("mount -t ext4 /dev/sdb1 /mnt/data", &gotest.CommandResponse{
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"path":   "/mnt/data",
				"src":    "/dev/sdb1",
				"fstype": "ext4",
				"state":  "mounted",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
		
		t.Run("AlreadyMounted", func(t *testing.T) {
			conn.Reset()
			// Check if already mounted
			conn.ExpectCommand("mount | grep '/mnt/data'", &gotest.CommandResponse{
				Stdout:   "/dev/sdb1 on /mnt/data type ext4 (rw,relatime)",
				ExitCode: 0, // Already mounted
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"path":   "/mnt/data",
				"src":    "/dev/sdb1",
				"fstype": "ext4",
				"state":  "mounted",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertNotChanged(result)
			conn.Verify()
		})
		
		t.Run("UnmountFilesystem", func(t *testing.T) {
			conn.Reset()
			// Check if mounted
			conn.ExpectCommand("mount | grep '/mnt/data'", &gotest.CommandResponse{
				Stdout:   "/dev/sdb1 on /mnt/data type ext4 (rw,relatime)",
				ExitCode: 0, // Mounted
			})
			
			// Unmount the filesystem
			conn.ExpectCommand("umount /mnt/data", &gotest.CommandResponse{
				ExitCode: 0,
			})
			
			result, err := m.Run(ctx, conn, map[string]interface{}{
				"path":  "/mnt/data",
				"state": "unmounted",
			})
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			helper.AssertSuccess(result)
			helper.AssertChanged(result)
			conn.Verify()
		})
	})
}