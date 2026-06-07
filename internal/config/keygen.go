package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

// KeyComment is the identifying comment embedded in managed keys and appended to
// the public keys this service materializes, e.g. "ssh-tunnel-service@host".
func KeyComment() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "localhost"
	}
	return "ssh-tunnel-service@" + host
}

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

// PublicKeyAuthorized derives the OpenSSH authorized_keys line (with trailing
// newline) for a private key in OpenSSH PEM form, appending a
// "ssh-tunnel-service@<hostname>" comment so the key is identifiable on the
// servers it is installed on. Used to materialize a `.pub` alongside every
// managed key so users can install it on target servers.
func PublicKeyAuthorized(privatePEM string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privatePEM))
	if err != nil {
		return "", fmt.Errorf("derive public key: %w", err)
	}
	line := strings.TrimRight(string(ssh.MarshalAuthorizedKey(signer.PublicKey())), "\n")
	return line + " " + KeyComment() + "\n", nil
}
