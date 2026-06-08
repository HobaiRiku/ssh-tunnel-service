package services

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/paths"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	cfg := &config.Config{
		App:     config.AppConfig{SSHHostKeyPolicy: config.SSHHostKeyPolicyInsecure},
		Remotes: []config.Remote{{Name: "bastion", Host: "h", Port: 22, User: "u"}},
		Tunnels: []config.Tunnel{{Name: "pg", Remote: "bastion", Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: 15432, TargetHost: "db", TargetPort: 5432}},
	}
	// Persist into an isolated temp home so writes never touch the source tree.
	return New(cfg, paths.Paths{Home: t.TempDir()}, NewRuntime())
}

func TestUpdateRemoteRenameCascadesToTunnels(t *testing.T) {
	reg := newTestRegistry(t)
	remote, _ := reg.GetRemote("bastion")
	remote.Name = "prod-bastion"
	if err := reg.UpdateRemote("bastion", remote); err != nil {
		t.Fatalf("UpdateRemote: %v", err)
	}
	tn, err := reg.GetTunnel("pg")
	if err != nil {
		t.Fatalf("GetTunnel: %v", err)
	}
	if tn.Remote != "prod-bastion" {
		t.Fatalf("tunnel remote reference = %q, want cascaded rename %q", tn.Remote, "prod-bastion")
	}
	if _, err := reg.GetRemote("bastion"); err == nil {
		t.Fatal("expected old remote name to be gone after rename")
	}
}

func TestAddTunnelRejectsDuplicateName(t *testing.T) {
	reg := newTestRegistry(t)
	dup := config.Tunnel{Name: "pg", Remote: "bastion", Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: 1, TargetHost: "x", TargetPort: 2}
	if err := reg.AddTunnel(dup); err == nil {
		t.Fatal("expected duplicate tunnel name to be rejected")
	}
}

func TestAddTunnelRejectsInvalidName(t *testing.T) {
	reg := newTestRegistry(t)
	bad := config.Tunnel{Name: "has/slash", Remote: "bastion", Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: 1, TargetHost: "x", TargetPort: 2}
	if err := reg.AddTunnel(bad); err == nil {
		t.Fatal("expected invalid tunnel name to be rejected before persistence")
	}
}

func TestDeleteRemoteRejectedWhenReferenced(t *testing.T) {
	reg := newTestRegistry(t)
	if err := reg.DeleteRemote("bastion"); err == nil {
		t.Fatal("expected delete of referenced remote to be rejected")
	}
}

func TestAddKeyRejectsWhitespaceName(t *testing.T) {
	reg := newTestRegistry(t)
	if err := reg.AddKey(SSHKeyInput{Name: " bad ", PrivateKey: "x"}); err == nil {
		t.Fatal("expected key name with surrounding whitespace to be rejected")
	}
}

// TestAddKeyWritesTrailingNewline guards against regressing the OpenSSH
// "invalid format" bug: TrimSpace on the imported material strips the final
// newline, but the written private key file must keep one after its
// "-----END ... PRIVATE KEY-----" footer or ssh refuses to load it.
func TestAddKeyWritesTrailingNewline(t *testing.T) {
	reg, _ := newEmptyRegistry(t)
	// Simulate an import where the source already lacks a trailing newline.
	priv, err := config.GenerateEd25519PrivateKey("test@host")
	if err != nil {
		t.Fatalf("GenerateEd25519PrivateKey: %v", err)
	}
	if err := reg.AddKey(SSHKeyInput{Name: "imported", PrivateKey: strings.TrimRight(priv, "\n")}); err != nil {
		t.Fatalf("AddKey: %v", err)
	}
	data, err := os.ReadFile(reg.KeyPath("imported"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasSuffix(string(data), "-----END OPENSSH PRIVATE KEY-----\n") {
		t.Fatalf("private key file must end with the footer + newline, got %q", data[max(0, len(data)-40):])
	}
	if _, err := ssh.ParseRawPrivateKey(data); err != nil {
		t.Fatalf("written key must be loadable by ssh: %v", err)
	}
}

func TestUpdateKeyRenameCascadesToRemotes(t *testing.T) {
	cfg := &config.Config{
		App:     config.AppConfig{SSHHostKeyPolicy: config.SSHHostKeyPolicyInsecure},
		Keys:    []config.SSHKey{{Name: "k1", File: "k1"}},
		Remotes: []config.Remote{{Name: "r1", Host: "h", Port: 22, User: "u", Key: "k1"}},
	}
	reg := New(cfg, paths.Paths{Home: t.TempDir()}, NewRuntime())
	// Rename only (no new key material) should keep the stored file and cascade.
	if err := reg.UpdateKey("k1", SSHKeyInput{Name: "k2"}); err != nil {
		t.Fatalf("UpdateKey rename: %v", err)
	}
	remote, err := reg.GetRemote("r1")
	if err != nil {
		t.Fatalf("GetRemote: %v", err)
	}
	if remote.Key != "k2" {
		t.Fatalf("remote key reference = %q, want cascaded rename %q", remote.Key, "k2")
	}
}
