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

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/paths"
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
	cases := []struct {
		name   string
		stderr string
		want   string
	}{
		{name: "host key verification", stderr: "Host key verification failed.", want: "host key verification failed"},
		{name: "host key changed", stderr: "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n@    WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!     @\nSomeone could be eavesdropping on you right now (man-in-the-middle attack)!\nOffending ECDSA key in /home/user/.ssh/known_hosts:3", want: "remote host key changed"},
		{name: "password required", stderr: "ubuntu@ssh.example.com: Permission denied (publickey,password).", want: "requires password/keyboard-interactive auth"},
		{name: "keyboard-interactive required", stderr: "ubuntu@ssh.example.com: Permission denied (keyboard-interactive).", want: "requires password/keyboard-interactive auth"},
		{name: "publickey only denied", stderr: "ubuntu@ssh.example.com: Permission denied (publickey).", want: "verify keys, agent access, and remote user permissions"},
		{name: "unresolved hostname", stderr: "ssh: Could not resolve hostname ssh.example.com: Name or service not known", want: "hostname could not be resolved"},
		{name: "connection refused", stderr: "ssh: connect to host ssh.example.com port 22: Connection refused", want: "connection refused"},
		{name: "connection timed out", stderr: "ssh: connect to host ssh.example.com port 22: Connection timed out", want: "network connection failed"},
		{name: "no diagnostic output", stderr: "", want: "ssh exited without diagnostic output"},
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

func TestCommandIncludesManagedTunnelOptions(t *testing.T) {
	cfg := &config.Config{
		App:     config.AppConfig{SSHHostKeyPolicy: config.SSHHostKeyPolicyAcceptNew, SSHKnownHosts: "/tmp/known_hosts"},
		Keys:    []config.SSHKey{{Name: "deploy-key", File: "deploy-key"}},
		Remotes: []config.Remote{{Name: "remote-a", Host: "ssh.example.com", Port: 2222, User: "ubuntu", Key: "deploy-key"}},
		Tunnels: []config.Tunnel{{Name: "tunnel-a", Remote: "remote-a", Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: 15432, TargetHost: "db.internal", TargetPort: 5432, SSHOptions: []string{"-o", "ServerAliveInterval=30"}}},
	}

	rt := NewRuntime()
	reg := New(cfg, paths.Paths{Home: "/tmp/home"}, rt)
	mgr := NewManager(context.Background(), reg, rt, slog.New(slog.NewTextHandler(io.Discard, nil)))

	preview, err := mgr.Command("tunnel-a")
	if err != nil {
		t.Fatalf("Command returned error: %v", err)
	}

	joined := strings.Join(preview.Args, " ")
	for _, want := range []string{
		"-o BatchMode=yes",
		"-o PasswordAuthentication=no",
		"-o KbdInteractiveAuthentication=no",
		"-o NumberOfPasswordPrompts=0",
		"-o ExitOnForwardFailure=yes",
		"-o ServerAliveInterval=15",
		"-o ServerAliveCountMax=3",
		"-o TCPKeepAlive=yes",
		"-o StrictHostKeyChecking=accept-new",
		"-o UserKnownHostsFile=/tmp/known_hosts",
		"-i /tmp/home/keys/deploy-key",
		"-o IdentitiesOnly=yes",
		"-L 127.0.0.1:15432:db.internal:5432",
		"-o ServerAliveInterval=30",
		"-p 2222 ubuntu@ssh.example.com",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected command args to contain %q, got %v", want, preview.Args)
		}
	}
	if !strings.HasPrefix(preview.Command, "ssh ") {
		t.Fatalf("expected preview command to start with ssh, got %q", preview.Command)
	}
}

func TestStopTerminatesGracefullyAndWaitsForExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake ssh shell script is POSIX-only")
	}

	home := t.TempDir()
	marker := filepath.Join(home, "marker")

	binDir := t.TempDir()
	fakeSSH := filepath.Join(binDir, "ssh")
	// The fake ssh records how it was asked to die: a clean SIGTERM appends
	// "term", letting us assert Stop terminates gracefully rather than SIGKILL.
	script := "#!/bin/sh\n" +
		"trap 'echo term >> \"" + marker + "\"; exit 0' TERM\n" +
		"echo ready >> \"" + marker + "\"\n" +
		"while true; do sleep 0.05; done\n"
	if err := os.WriteFile(fakeSSH, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := &config.Config{
		App:     config.AppConfig{SSHHostKeyPolicy: config.SSHHostKeyPolicyInsecure},
		Remotes: []config.Remote{{Name: "remote-a", Host: "ssh.example.com", Port: 22, User: "ubuntu"}},
		Tunnels: []config.Tunnel{{Name: "tunnel-a", Remote: "remote-a", Direction: config.DirectionRemote, BindAddress: "127.0.0.1", BindPort: 9000, TargetHost: "127.0.0.1", TargetPort: 8080}},
	}

	rt := NewRuntime()
	reg := New(cfg, paths.Paths{Home: home}, rt)
	mgr := NewManager(context.Background(), reg, rt, slog.New(slog.NewTextHandler(io.Discard, nil)))
	reg.SetManager(mgr)

	if err := mgr.Start("tunnel-a"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	// Wait until the fake ssh has installed its TERM trap (it appends "ready"),
	// otherwise Stop could race in before the trap exists and SIGTERM would just
	// kill it by default — masking whether termination was graceful.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(marker); err == nil && strings.Contains(string(data), "ready") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := mgr.Stop("tunnel-a"); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	// Stop must wait for the process to actually exit before returning, so the
	// next Start can re-bind the (remote) port without racing the old ssh.
	if mgr.IsRunning("tunnel-a") {
		t.Fatalf("expected no running process after Stop returns")
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if !strings.Contains(string(data), "term") {
		t.Fatalf("expected graceful SIGTERM termination, marker=%q", string(data))
	}

	// A clean restart after the wait must succeed.
	if err := mgr.Restart("tunnel-a", "test"); err != nil {
		t.Fatalf("Restart returned error: %v", err)
	}
	if !mgr.IsRunning("tunnel-a") {
		t.Fatalf("expected tunnel to be running after Restart")
	}
	if err := mgr.Stop("tunnel-a"); err != nil {
		t.Fatalf("final Stop returned error: %v", err)
	}
}

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

	home := t.TempDir()
	keyPath := filepath.Join(home, "keys")
	if err := os.MkdirAll(keyPath, 0o700); err != nil {
		t.Fatalf("mkdir keys: %v", err)
	}
	if err := os.WriteFile(filepath.Join(keyPath, "deploy-key"), []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nkey\n-----END OPENSSH PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatalf("write managed key: %v", err)
	}

	cfg := &config.Config{
		App:     config.AppConfig{SSHHostKeyPolicy: config.SSHHostKeyPolicyInsecure},
		Keys:    []config.SSHKey{{Name: "deploy-key", File: "deploy-key"}},
		Remotes: []config.Remote{{Name: "remote-a", Host: "ssh.example.com", Port: 22, User: "ubuntu", Key: "deploy-key"}},
		Tunnels: []config.Tunnel{{Name: "tunnel-a", Remote: "remote-a", Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: 15432, TargetHost: "127.0.0.1", TargetPort: 5432}},
	}

	rt := NewRuntime()
	reg := New(cfg, paths.Paths{Home: home}, rt)
	mgr := NewManager(context.Background(), reg, rt, slog.New(slog.NewTextHandler(io.Discard, nil)))
	reg.SetManager(mgr)

	if err := mgr.Start("tunnel-a"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

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
