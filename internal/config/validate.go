package config

import "fmt"

// Validate checks the parsed config for required fields and consistency.
func Validate(cfg *Config) error {
ids := map[string]bool{}
for i, r := range cfg.Remotes {
if r.ID == "" {
return fmt.Errorf("remotes[%d]: id is required", i)
}
if ids[r.ID] {
return fmt.Errorf("remotes[%d]: duplicate id %q", i, r.ID)
}
ids[r.ID] = true
if r.Host == "" {
return fmt.Errorf("remotes[%d] (%s): host is required", i, r.ID)
}
if r.Port <= 0 || r.Port > 65535 {
return fmt.Errorf("remotes[%d] (%s): port must be 1-65535", i, r.ID)
}
if r.User == "" {
return fmt.Errorf("remotes[%d] (%s): user is required", i, r.ID)
}
}
tids := map[string]bool{}
for i, t := range cfg.Tunnels {
if t.ID == "" {
return fmt.Errorf("tunnels[%d]: id is required", i)
}
if tids[t.ID] {
return fmt.Errorf("tunnels[%d]: duplicate id %q", i, t.ID)
}
tids[t.ID] = true
if !ids[t.RemoteID] {
return fmt.Errorf("tunnels[%d] (%s): remote_id %q not found", i, t.ID, t.RemoteID)
}
if t.Direction != DirectionLocal && t.Direction != DirectionRemote {
return fmt.Errorf("tunnels[%d] (%s): direction must be -L or -R", i, t.ID)
}
if t.BindPort <= 0 || t.BindPort > 65535 {
return fmt.Errorf("tunnels[%d] (%s): bind_port must be 1-65535", i, t.ID)
}
if t.TargetHost == "" {
return fmt.Errorf("tunnels[%d] (%s): target_host is required", i, t.ID)
}
if t.TargetPort <= 0 || t.TargetPort > 65535 {
return fmt.Errorf("tunnels[%d] (%s): target_port must be 1-65535", i, t.ID)
}
}
return nil
}

// applyDefaults fills in optional fields with their default values.
func applyDefaults(cfg *Config) {
if cfg.App.HTTPListen == "" {
cfg.App.HTTPListen = "127.0.0.1:2222"
}
if cfg.App.LogLevel == "" {
cfg.App.LogLevel = "info"
}
for i := range cfg.Remotes {
if cfg.Remotes[i].Port == 0 {
cfg.Remotes[i].Port = 22
}
}
for i := range cfg.Tunnels {
if cfg.Tunnels[i].BindAddress == "" {
cfg.Tunnels[i].BindAddress = "127.0.0.1"
}
}
}
