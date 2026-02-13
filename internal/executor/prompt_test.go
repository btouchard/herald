package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWritePromptFile_CreatesFileWithContent(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	path, err := WritePromptFile(workDir, "herald-test01", "fix the bug")
	require.NoError(t, err)

	assert.True(t, filepath.IsAbs(path))
	assert.Contains(t, path, "prompt.md")

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "fix the bug", string(content))
}

func TestWritePromptFile_CreatesTaskDirectory(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	_, err := WritePromptFile(workDir, "herald-abc123", "test prompt")
	require.NoError(t, err)

	dir := filepath.Join(workDir, "tasks", "herald-abc123")
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestWritePromptFile_SetsProperPermissions(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	path, err := WritePromptFile(workDir, "herald-perm01", "secret prompt")
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	// File should be readable by owner and group, no world access
	assert.Equal(t, os.FileMode(0640), info.Mode().Perm())
}

func TestWritePromptFile_HandlesLargePrompt(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	largePrompt := string(make([]byte, 100_000)) // 100KB

	path, err := WritePromptFile(workDir, "herald-large1", largePrompt)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Len(t, content, 100_000)
}

func TestWritePromptFile_WhenInvalidWorkDir_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := WritePromptFile("/nonexistent/path/that/does/not/exist", "herald-err01", "prompt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creating prompt directory")
}

func TestCleanupPromptFile_RemovesDirectory(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	_, err := WritePromptFile(workDir, "herald-clean1", "to be cleaned")
	require.NoError(t, err)

	dir := filepath.Join(workDir, "tasks", "herald-clean1")
	_, err = os.Stat(dir)
	require.NoError(t, err)

	CleanupPromptFile(workDir, "herald-clean1")

	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err))
}

func TestCleanupPromptFile_WhenDirDoesNotExist_DoesNotPanic(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		CleanupPromptFile("/tmp/nonexistent", "herald-nope01")
	})
}
