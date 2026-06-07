package config

import (
	"net"
	"strconv"
)

// maxPortProbe bounds how many consecutive ports PickAvailableListen tries
// before giving up and returning the preferred address as-is (Validate /
// the HTTP server will then surface the real bind error).
const maxPortProbe = 50

// PickAvailableListen returns preferred if its address is free to bind, or the
// first free "host:port" found by probing consecutive ports above it
// otherwise. This lets a fresh install on a host that already runs another
// instance (e.g. system + per-user) avoid colliding on the default port
// without any user intervention. If preferred can't be parsed or no free port
// is found within the probe range, preferred is returned unchanged.
func PickAvailableListen(preferred string) string {
	host, portStr, err := net.SplitHostPort(preferred)
	if err != nil {
		return preferred
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return preferred
	}
	for i := 0; i < maxPortProbe; i++ {
		candidate := net.JoinHostPort(host, strconv.Itoa(port+i))
		if portFree(candidate) {
			return candidate
		}
	}
	return preferred
}

func portFree(addr string) bool {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
