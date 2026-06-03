package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// GenerateEd25519PrivateKey creates a fresh ed25519 keypair and returns the
// private key serialized in the OpenSSH PEM format (the same format the key
// store accepts via ssh.ParseRawPrivateKey). comment is embedded in the key.
func GenerateEd25519PrivateKey(comment string) (string, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generate ed25519 key: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return "", fmt.Errorf("marshal ed25519 key: %w", err)
	}
	return string(pem.EncodeToMemory(block)), nil
}
