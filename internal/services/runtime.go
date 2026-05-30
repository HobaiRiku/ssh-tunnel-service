package services

import "sync"

// Runtime holds the live process state for all running tunnels.
type Runtime struct {
	mu      sync.RWMutex
	tunnels map[string]*tunnelEntry
}

type tunnelEntry struct {
	state TunnelState
	pid   int
	err   string
}

// NewRuntime returns an initialized Runtime.
func NewRuntime() *Runtime {
	return &Runtime{tunnels: map[string]*tunnelEntry{}}
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
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.tunnels[id] = &tunnelEntry{state: StateError, err: errMsg}
}

func (rt *Runtime) Get(id string) (state TunnelState, pid int, errMsg string) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	if e, ok := rt.tunnels[id]; ok {
		return e.state, e.pid, e.err
	}
	return StateStopped, 0, ""
}
