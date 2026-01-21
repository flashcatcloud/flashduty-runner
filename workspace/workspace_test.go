package workspace

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flashcatcloud/flashduty-runner/permission"
	"github.com/flashcatcloud/flashduty-runner/protocol"
)

func newTestWorkspace(t *testing.T) *Workspace {
	tmpDir := t.TempDir()
	checker := permission.NewChecker(map[string]string{
		"*": "allow", // Allow all for testing
	})
	ws, err := New(tmpDir, checker)
	require.NoError(t, err)
	return ws
}

func TestWorkspace_Read(t *testing.T) {
	ws := newTestWorkspace(t)
	ctx := context.Background()

	// Create test file
	testContent := "Hello, World!"
	testPath := "test.txt"
	err := os.WriteFile(filepath.Join(ws.Root(), testPath), []byte(testContent), 0o644)
	require.NoError(t, err)

	// Read entire file
	result, err := ws.Read(ctx, &protocol.ReadArgs{Path: testPath})
	require.NoError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(result.Content)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(decoded))
	assert.Equal(t, int64(len(testContent)), result.TotalSize)

	// Read with offset and limit
	result, err = ws.Read(ctx, &protocol.ReadArgs{
		Path:   testPath,
		Offset: 7,
		Limit:  5,
	})
	require.NoError(t, err)

	decoded, err = base64.StdEncoding.DecodeString(result.Content)
	require.NoError(t, err)
	assert.Equal(t, "World", string(decoded))
}

func TestWorkspace_Read_PathTraversal(t *testing.T) {
	ws := newTestWorkspace(t)
	ctx := context.Background()

	// Attempt path traversal
	_, err := ws.Read(ctx, &protocol.ReadArgs{Path: "../../../etc/passwd"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside workspace root")
}

func TestWorkspace_Write(t *testing.T) {
	ws := newTestWorkspace(t)
	ctx := context.Background()

	testContent := "Test content"
	testPath := "subdir/test.txt"

	err := ws.Write(ctx, &protocol.WriteArgs{
		Path:    testPath,
		Content: base64.StdEncoding.EncodeToString([]byte(testContent)),
	})
	require.NoError(t, err)

	// Verify file was created
	content, err := os.ReadFile(filepath.Join(ws.Root(), testPath))
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestWorkspace_List(t *testing.T) {
	ws := newTestWorkspace(t)
	ctx := context.Background()

	// Create test structure
	os.MkdirAll(filepath.Join(ws.Root(), "dir1"), 0o755)
	os.MkdirAll(filepath.Join(ws.Root(), "dir2"), 0o755)
	os.WriteFile(filepath.Join(ws.Root(), "file1.txt"), []byte("content"), 0o644)
	os.WriteFile(filepath.Join(ws.Root(), "dir1", "file2.txt"), []byte("content"), 0o644)

	// List non-recursive
	result, err := ws.List(ctx, &protocol.ListArgs{Path: ".", Recursive: false})
	require.NoError(t, err)
	assert.Len(t, result.Entries, 3) // dir1, dir2, file1.txt

	// List recursive
	result, err = ws.List(ctx, &protocol.ListArgs{Path: ".", Recursive: true})
	require.NoError(t, err)
	assert.Len(t, result.Entries, 4) // dir1, dir2, file1.txt, dir1/file2.txt
}

func TestWorkspace_Glob(t *testing.T) {
	ws := newTestWorkspace(t)
	ctx := context.Background()

	// Create test files
	os.WriteFile(filepath.Join(ws.Root(), "file1.txt"), []byte("content"), 0o644)
	os.WriteFile(filepath.Join(ws.Root(), "file2.txt"), []byte("content"), 0o644)
	os.WriteFile(filepath.Join(ws.Root(), "file3.log"), []byte("content"), 0o644)

	result, err := ws.Glob(ctx, &protocol.GlobArgs{Pattern: "*.txt"})
	require.NoError(t, err)
	assert.Len(t, result.Matches, 2)
	assert.Contains(t, result.Matches, "file1.txt")
	assert.Contains(t, result.Matches, "file2.txt")
}

func TestWorkspace_Grep(t *testing.T) {
	ws := newTestWorkspace(t)
	ctx := context.Background()

	// Create test files
	os.WriteFile(filepath.Join(ws.Root(), "file1.txt"), []byte("hello world\nfoo bar\nhello again"), 0o644)
	os.WriteFile(filepath.Join(ws.Root(), "file2.txt"), []byte("no match here"), 0o644)

	result, err := ws.Grep(ctx, &protocol.GrepArgs{Pattern: "hello"})
	require.NoError(t, err)
	assert.Len(t, result.Matches, 2)
}

func TestWorkspace_Bash(t *testing.T) {
	ws := newTestWorkspace(t)
	ctx := context.Background()

	// Simple command
	result, err := ws.Bash(ctx, &protocol.BashArgs{Command: "echo hello"})
	require.NoError(t, err)
	assert.Equal(t, "hello\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)

	// Command with exit code
	result, err = ws.Bash(ctx, &protocol.BashArgs{Command: "exit 42"})
	require.NoError(t, err)
	assert.Equal(t, 42, result.ExitCode)
}

func TestWorkspace_Bash_PermissionDenied(t *testing.T) {
	tmpDir := t.TempDir()
	// Note: rules are sorted alphabetically, so "echo *" comes after "*"
	// This means "echo *" will override "*" for echo commands
	checker := permission.NewChecker(map[string]string{
		"*":      "deny",
		"echo *": "allow",
	})
	ws, err := New(tmpDir, checker)
	require.NoError(t, err)

	ctx := context.Background()

	// Allowed command (matches "echo *")
	result, err := ws.Bash(ctx, &protocol.BashArgs{Command: "echo hello"})
	require.NoError(t, err)
	assert.Equal(t, "hello\n", result.Stdout)

	// Denied command (only matches "*" which is deny)
	_, err = ws.Bash(ctx, &protocol.BashArgs{Command: "rm -rf /"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func TestWorkspace_SafePath(t *testing.T) {
	ws := newTestWorkspace(t)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty path", "", false},
		{"dot path", ".", false},
		{"relative path", "subdir/file.txt", false},
		{"path traversal", "../etc/passwd", true},
		{"path traversal deep", "../../../../../../etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ws.safePath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
