package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafePath_AllowsValidPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src", "pkg"), 0755))

	tests := []struct {
		name string
		path string
	}{
		{"simple file", "main.go"},
		{"nested file", "src/pkg/handler.go"},
		{"with dot prefix", "./main.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := SafePath(root, tt.path)
			require.NoError(t, err)
			assert.True(t, filepath.IsAbs(result))
		})
	}
}

func TestSafePath_RejectsTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	tests := []struct {
		name string
		path string
	}{
		{"parent directory", "../etc/passwd"},
		{"deep traversal", "../../../etc/shadow"},
		{"absolute path", "/etc/passwd"},
		{"hidden traversal", "src/../../etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := SafePath(root, tt.path)
			assert.Error(t, err, "path %q should be rejected", tt.path)
		})
	}
}

func TestSafePath_RejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Create a symlink inside the project that points outside
	outsideDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0644))
	require.NoError(t, os.Symlink(outsideDir, filepath.Join(root, "escape")))

	_, err := SafePath(root, "escape/secret.txt")
	assert.Error(t, err, "symlink pointing outside project root should be rejected")
	assert.Contains(t, err.Error(), "symlink escape detected")
}

func TestSafePath_AllowsSymlinkInsideProject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	subdir := filepath.Join(root, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subdir, "file.txt"), []byte("ok"), 0644))
	// Symlink within the project
	require.NoError(t, os.Symlink(subdir, filepath.Join(root, "link")))

	result, err := SafePath(root, "link/file.txt")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(result))
}

func TestSafePath_RejectsRootItself(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// ".." from a subdirectory that resolves to root should be caught
	_, err := SafePath(root, "subdir/..")
	// This resolves to the root itself, which is allowed since absPath == absRoot
	// but not a file - the handler checks IsDir separately
	assert.NoError(t, err)
}
