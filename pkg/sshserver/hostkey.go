package sshserver

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// LoadOrGenerateSigner loads an SSH signer from the provided path, generating a new RSA host key if none exists.
func LoadOrGenerateSigner(path string) (ssh.Signer, error) {
	if path == "" {
		return EphemeralSigner()
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("sshserver: resolve host key path: %w", err)
	}

	if signer, err := loadSigner(absPath); err == nil {
		return signer, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return generateAndStoreSigner(absPath)
}

func loadSigner(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	key, err := ssh.ParseRawPrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("sshserver: parse host key %q: %w", path, err)
	}

	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, fmt.Errorf("sshserver: create signer from %q: %w", path, err)
	}

	return signer, nil
}

func generateAndStoreSigner(path string) (ssh.Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("sshserver: generate host key: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("sshserver: create host key dir %q: %w", dir, err)
	}

	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	if err := os.WriteFile(path, pem.EncodeToMemory(pemBlock), 0o600); err != nil {
		return nil, fmt.Errorf("sshserver: write host key %q: %w", path, err)
	}

	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, fmt.Errorf("sshserver: create signer: %w", err)
	}

	return signer, nil
}
