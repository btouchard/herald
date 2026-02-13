package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const secretFileName = "secret"

// LoadOrCreateSecret reads the secret from configDir/secret, or generates and
// persists a new 256-bit hex-encoded secret if the file is missing or empty.
func LoadOrCreateSecret(configDir string) (string, error) {
	path := filepath.Join(configDir, secretFileName)

	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		return string(data), nil
	}

	secret, err := generateSecret()
	if err != nil {
		return "", err
	}

	if err := writeSecret(configDir, path, secret); err != nil {
		return "", err
	}

	return secret, nil
}

// RotateSecret generates a new secret, replacing the existing one.
// All existing sessions are invalidated when the secret changes.
func RotateSecret(configDir string) (string, error) {
	path := filepath.Join(configDir, secretFileName)

	secret, err := generateSecret()
	if err != nil {
		return "", err
	}

	if err := writeSecret(configDir, path, secret); err != nil {
		return "", err
	}

	return secret, nil
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func writeSecret(configDir, path, secret string) error {
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(secret), 0600); err != nil {
		return fmt.Errorf("write secret: %w", err)
	}
	return nil
}
