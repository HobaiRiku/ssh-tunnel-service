package config

import (
	"net"
	"testing"
)

func TestPickAvailableListenSkipsOccupiedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	taken := ln.Addr().String()
	got := PickAvailableListen(taken)
	if got == taken {
		t.Fatalf("expected a different address than the occupied %q, got the same", taken)
	}
	if !portFree(got) {
		t.Fatalf("PickAvailableListen returned an address that isn't actually free: %q", got)
	}
}

func TestPickAvailableListenKeepsFreeAddress(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	if got := PickAvailableListen(addr); got != addr {
		t.Fatalf("expected PickAvailableListen to keep the free address %q, got %q", addr, got)
	}
}
