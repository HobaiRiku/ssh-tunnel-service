package services

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/config"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/paths"
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
	// Fixtures are real OpenSSH stderr output for each failure class.
	cases := []struct {
		name   string
		stderr string
		want   string
	}{
		{
			name:   "host key verification",
			stderr: "Host key verification failed.",
			want:   "host key verification failed",
		},
		{
			name: "host key changed",
			stderr: "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n" +
				"@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @\n" +
				"Someone could be eavesdropping on you right now (man-in-the-middle attack)!\n" +
				"Offending ECDSA key in /home/user/.ssh/known_hosts:3",
			want: "remote host key changed",
		},
		{
			name:   "password required",
			stderr: "ubuntu@ssh.example.com: Permission denied (publickey,password).",
			want:   "requires password/keyboard-interactive auth",
		},
		{
			name:   "keyboard-interactive required",
			stderr: "ubuntu@ssh.example.com: Permission denied (keyboard-interactive).",
			want:   "requires password/keyboard-interactive auth",
		},
		{
			name:   "publickey only denied",
			stderr: "ubuntu@ssh.example.com: Permission denied (publickey).",
			want:   "verify keys, agent access, and remote user permissions",
		},
		{
			name:   "unresolved hostname",
			stderr: "ssh: Could not resolve hostname ssh.example.com: Name or service not known",
			want:   "hostname could not be resolved",
		},
		{
			name:   "connection refused",
			stderr: "ssh: connect to host ssh.example.com port 22: Connection refused",
			want:   "connection refused",
		},
		{
			name:   "connection timed out",
			stderr: "ssh: connect to host ssh.example.com port 22: Connection timed out",
			want:   "network connection failed",
		},
		{
			name:   "no diagnostic output",
			stderr: "",
			want:   "ssh exited without diagnostic output",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := diagnoseSSHFailure(tc.stderr)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("diagnoseSSHFailure(%q) = %q, want substring %q", tc.stderr, got, tc.want)
			}
		})
	}
}

// TestStartMarksPasswordRemoteAsError verifies the end-to-end behaviour the
// service depends on: when a remote only offers interactive (password) auth,
// ssh exits non-interactively and the tunnel is recorded as an error instead
// of hanging. A fake ssh on PATH reproduces OpenSSH's real failure output.
func TestStartMarksPasswordRemoteAsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake ssh shell script is POSIX-only")
	}

	binDir := t.TempDir()
	fakeSSH := filepath.Join(binDir, "ssh")
	script := "#!/bin/sh\n" +
		"echo 'ubuntu@ssh.example.com: Permission denied (publickey,password).' >&2\n" +
		"exit 255\n"
	if err := os.WriteFile(fakeSSH, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := &config.Config{
		App: config.AppConfig{SSHHostKeyPolicy: config.SSHHostKeyPolicyInsecure},
		Remotes: []config.Remote{{
			ID: "remote-a", Host: "ssh.example.com", Port: 22, User: "ubuntu",
		}},
		Tunnels: []config.Tunnel{{
			ID: "tunnel-a", RemoteID: "remote-a", Direction: config.DirectionLocal,
			BindAddress: "127.0.0.1", BindPort: 15432,
			TargetHost: "127.0.0.1", TargetPort: 5432,
		}},
	}

	rt := NewRuntime()
	reg := New(cfg, paths.Paths{}, rt)
	mgr := NewManager(reg, rt, slog.New(slog.NewTextHandler(io.Discard, nil)))
	reg.SetManager(mgr)

	if err := mgr.Start(context.Background(), "tunnel-a"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	// ssh exits almost immediately; poll until the watcher goroutine records
	// the terminal state rather than relying on a fixed sleep.
	deadline := time.Now().Add(3 * time.Second)
	var state TunnelState
	var errMsg string
	for time.Now().Before(deadline) {
		state, _, errMsg = rt.Get("tunnel-a")
		if state != StateRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if state != StateError {
		t.Fatalf("expected tunnel state %q, got %q", StateError, state)
	}
	if !strings.Contains(errMsg, "requires password/keyboard-interactive auth") {
		t.Fatalf("expected password diagnostic, got %q", errMsg)
	}
}
