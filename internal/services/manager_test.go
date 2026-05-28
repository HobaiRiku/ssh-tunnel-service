package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/config"
)

func TestSSHHostKeyArgsAcceptNew(t *testing.T) {
	args := sshHostKeyArgs(config.AppConfig{
		SSHHostKeyPolicy: config.SSHHostKeyPolicyAcceptNew,
		SSHKnownHosts:    "/tmp/known_hosts",
	})

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "StrictHostKeyChecking=accept-new") {
		t.Fatalf("expected accept-new policy args, got %v", args)
	}
	if !strings.Contains(joined, "UserKnownHostsFile=/tmp/known_hosts") {
		t.Fatalf("expected managed known_hosts path, got %v", args)
	}
}

func TestSSHHostKeyArgsInsecureUsesDevNull(t *testing.T) {
	args := sshHostKeyArgs(config.AppConfig{
		SSHHostKeyPolicy: config.SSHHostKeyPolicyInsecure,
		SSHKnownHosts:    "/tmp/known_hosts",
	})

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "StrictHostKeyChecking=no") {
		t.Fatalf("expected insecure policy args, got %v", args)
	}
	if !strings.Contains(joined, "UserKnownHostsFile="+os.DevNull) {
		t.Fatalf("expected devnull known_hosts override, got %v", args)
	}
}

func TestEnsureKnownHostsFileCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "known_hosts")

	if err := ensureKnownHostsFile(path, 0o600); err != nil {
		t.Fatalf("ensureKnownHostsFile returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected known_hosts file to exist: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %o", info.Mode().Perm())
	}
}

func TestDiagnoseSSHFailure(t *testing.T) {
	got := diagnoseSSHFailure("Host key verification failed.")
	if !strings.Contains(got, "host key verification failed") {
		t.Fatalf("unexpected diagnosis: %q", got)
	}
}
