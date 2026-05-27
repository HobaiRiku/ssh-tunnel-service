package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/domain"
)

type launcherStub struct {
	launched map[string][]string
	stopped  map[string]bool
}

func (l *launcherStub) Launch(_ context.Context, id string, args []string) error {
	if l.launched == nil {
		l.launched = map[string][]string{}
	}
	l.launched[id] = args
	return nil
}

func (l *launcherStub) Stop(id string) error {
	if l.stopped == nil {
		l.stopped = map[string]bool{}
	}
	if id == "missing" {
		return errors.New("missing")
	}
	l.stopped[id] = true
	return nil
}

func TestBuildSSHArgs(t *testing.T) {
	svc := New(nil)
	if err := svc.AddRemote(domain.Remote{ID: "r1", Name: "prod", Host: "remote.example", Port: 2222, User: "ubuntu"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddCommd(domain.Commd{
		ID: "c1", Name: "api", RemoteID: "r1", Direction: domain.DirectionLocal,
		BindAddress: "127.0.0.1", BindPort: 15432, TargetHost: "db", TargetPort: 5432,
		SSHOptions: []string{"-o", "ServerAliveInterval=30"},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.BuildSSHArgs("c1")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-N", "-L", "127.0.0.1:15432:db:5432", "-o", "ServerAliveInterval=30", "-p", "2222", "ubuntu@remote.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ssh args mismatch\nwant: %v\n got: %v", want, got)
	}
}

func TestAutoStartLaunchesOnlyAutoStartCommd(t *testing.T) {
	launcher := &launcherStub{}
	svc := New(launcher)
	if err := svc.AddRemote(domain.Remote{ID: "r1", Name: "prod", Host: "remote.example", Port: 22, User: "ubuntu"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddCommd(domain.Commd{ID: "c1", Name: "auto", RemoteID: "r1", Direction: domain.DirectionRemote, BindAddress: "0.0.0.0", BindPort: 8080, TargetHost: "127.0.0.1", TargetPort: 80, AutoStart: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddCommd(domain.Commd{ID: "c2", Name: "manual", RemoteID: "r1", Direction: domain.DirectionLocal, BindAddress: "127.0.0.1", BindPort: 3306, TargetHost: "db", TargetPort: 3306}); err != nil {
		t.Fatal(err)
	}

	if err := svc.AutoStart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := launcher.launched["c1"]; !ok {
		t.Fatalf("expected c1 launched")
	}
	if _, ok := launcher.launched["c2"]; ok {
		t.Fatalf("expected c2 not launched")
	}
}
