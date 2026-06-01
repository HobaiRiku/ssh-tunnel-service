package services

import (
	"testing"

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
