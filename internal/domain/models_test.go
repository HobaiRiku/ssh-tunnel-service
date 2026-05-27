package domain

import "testing"

func TestRemoteValidate(t *testing.T) {
	r := Remote{ID: "r1", Name: "home", Host: "example.com", Port: 22, User: "root"}
	if err := r.Validate(); err != nil {
		t.Fatalf("expected valid remote, got error: %v", err)
	}

	r.Port = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected invalid remote port error")
	}
}

func TestCommdValidate(t *testing.T) {
	c := Commd{
		ID:          "c1",
		Name:        "forward web",
		RemoteID:    "r1",
		Direction:   DirectionRemote,
		BindAddress: "0.0.0.0",
		BindPort:    8080,
		TargetHost:  "127.0.0.1",
		TargetPort:  80,
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid commd, got error: %v", err)
	}

	c.Direction = "-X"
	if err := c.Validate(); err == nil {
		t.Fatalf("expected invalid direction error")
	}
}
