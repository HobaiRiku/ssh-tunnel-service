package services

import "sync"

// Runtime holds the live process state for all running tunnels, plus their
// traffic meters (see traffic.go).
type Runtime struct {
	mu      sync.RWMutex
	tunnels map[string]*tunnelEntry
	traffic *trafficBook
}

type tunnelEntry struct {
	state   TunnelState
	pid     int
	err     string
	failure Failure
}

// NewRuntime returns an initialized Runtime.
func NewRuntime() *Runtime {
	return &Runtime{tunnels: map[string]*tunnelEntry{}, traffic: newTrafficBook()}
}

func (rt *Runtime) SetRunning(id string, pid int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.tunnels[id] = &tunnelEntry{state: StateRunning, pid: pid}
}

func (rt *Runtime) SetStopped(id string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.tunnels[id] = &tunnelEntry{state: StateStopped}
}

func (rt *Runtime) SetError(id, errMsg string) {
	rt.SetFailure(id, Failure{Message: errMsg})
}

// SetFailure marks a tunnel as failed with a classified cause.
func (rt *Runtime) SetFailure(id string, f Failure) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.tunnels[id] = &tunnelEntry{state: StateError, err: f.Message, failure: f}
}

// LastFailure returns the classified cause of a tunnel in the error state, or
// the zero Failure otherwise.
func (rt *Runtime) LastFailure(id string) Failure {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	if e, ok := rt.tunnels[id]; ok && e.state == StateError {
		return e.failure
	}
	return Failure{}
}

func (rt *Runtime) Get(id string) (state TunnelState, pid int, errMsg string) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	if e, ok := rt.tunnels[id]; ok {
		return e.state, e.pid, e.err
	}
	return StateStopped, 0, ""
}
