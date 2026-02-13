package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOrCreateSecret_WhenNoFile_CreatesNewSecret(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	secret, err := LoadOrCreateSecret(dir)
	require.NoError(t, err)

	assert.Len(t, secret, 64, "secret should be 64 hex chars (32 bytes)")
	assertHexString(t, secret)

	// Verify file permissions
	info, err := os.Stat(filepath.Join(dir, secretFileName))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestLoadOrCreateSecret_WhenFileExists_ReturnsExisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existing := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	require.NoError(t, os.WriteFile(filepath.Join(dir, secretFileName), []byte(existing), 0600))

	secret, err := LoadOrCreateSecret(dir)
	require.NoError(t, err)
	assert.Equal(t, existing, secret)
}

func TestLoadOrCreateSecret_CalledTwice_ReturnsSameSecret(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	first, err := LoadOrCreateSecret(dir)
	require.NoError(t, err)

	second, err := LoadOrCreateSecret(dir)
	require.NoError(t, err)

	assert.Equal(t, first, second)
}

func TestRotateSecret_GeneratesDifferentSecret(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	original, err := LoadOrCreateSecret(dir)
	require.NoError(t, err)

	rotated, err := RotateSecret(dir)
	require.NoError(t, err)

	assert.NotEqual(t, original, rotated)
	assert.Len(t, rotated, 64)
	assertHexString(t, rotated)
}

func TestRotateSecret_FileHasRestrictivePermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := RotateSecret(dir)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, secretFileName))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestLoadOrCreateSecret_WhenEmptyFile_GeneratesNew(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, secretFileName), []byte(""), 0600))

	secret, err := LoadOrCreateSecret(dir)
	require.NoError(t, err)

	assert.Len(t, secret, 64)
	assertHexString(t, secret)
}

func assertHexString(t *testing.T, s string) {
	t.Helper()
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("non-hex character %q in string %q", c, s)
			return
		}
	}
}
