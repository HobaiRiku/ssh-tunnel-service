// Package config defines the typed YAML schema for ~/.ssh-tunnel-service/config.yaml.
package config

// DefaultHTTPListen is the address the management API + Web UI bind to when
// config.yaml does not set app.http_listen. The CLI also falls back to it when
// locating the running service.
const DefaultHTTPListen = "127.0.0.1:2222"

// Config is the root document of config.yaml.
type Config struct {
	App     AppConfig `yaml:"app"     json:"app"`
	Keys    []SSHKey  `yaml:"keys"    json:"keys"`
	Remotes []Remote  `yaml:"remotes" json:"remotes"`
	Tunnels []Tunnel  `yaml:"tunnels" json:"tunnels"`
}

// SSHHostKeyPolicy controls how OpenSSH validates remote host keys.
type SSHHostKeyPolicy string

const (
	SSHHostKeyPolicyAcceptNew SSHHostKeyPolicy = "accept-new"
	SSHHostKeyPolicyStrict    SSHHostKeyPolicy = "strict"
	SSHHostKeyPolicyInsecure  SSHHostKeyPolicy = "insecure"
)

// AppConfig covers the management HTTP API + Web UI.
type AppConfig struct {
	HTTPListen       string           `yaml:"http_listen"          json:"http_listen"`
	LogLevel         string           `yaml:"log_level"            json:"log_level"`
	LogConsole       bool             `yaml:"log_console"          json:"log_console"`
	LogMaxSizeMB     int              `yaml:"log_max_size_mb"      json:"log_max_size_mb"`
	LogMaxBackups    int              `yaml:"log_max_backups"      json:"log_max_backups"`
	LogMaxAgeDays    int              `yaml:"log_max_age_days"     json:"log_max_age_days"`
	LogCompress      bool             `yaml:"log_compress"         json:"log_compress"`
	SSHHostKeyPolicy SSHHostKeyPolicy `yaml:"ssh_host_key_policy"  json:"ssh_host_key_policy"`
	SSHKnownHosts    string           `yaml:"ssh_known_hosts_file" json:"ssh_known_hosts_file,omitempty"`
	// SystemDefaultKey names the managed key used by tunnels whose remote binds
	// no explicit key when the service runs as a system service. It is generated
	// on first system-service start, cannot be deleted while designated, and may
	// be switched to another key or rotated in place. It is unused in per-user
	// (session) runs, where unbound tunnels rely on the system ssh defaults.
	SystemDefaultKey string `yaml:"system_default_key"   json:"system_default_key,omitempty"`
}

// SSHKey is a managed private key stored beneath the runtime home directory.
// The unique key is Name; there is no separate id concept.
type SSHKey struct {
	Name        string `yaml:"name"        json:"name"`
	File        string `yaml:"file"        json:"file"`
	Description string `yaml:"description" json:"description,omitempty"`
	// Public is the derived OpenSSH public key (authorized_keys line). It is
	// never persisted to config.yaml (yaml:"-"); the registry populates it on
	// read from the managed `<file>.pub` so callers can copy it to target hosts.
	Public string `yaml:"-" json:"public_key,omitempty"`
}

// Remote is a reusable SSH target server definition, keyed by its unique Name.
type Remote struct {
	Name        string `yaml:"name"        json:"name"`
	Host        string `yaml:"host"        json:"host"`
	Port        int    `yaml:"port"        json:"port"`
	User        string `yaml:"user"        json:"user"`
	Key         string `yaml:"key,omitempty" json:"key,omitempty"` // references SSHKey.Name
	Description string `yaml:"description" json:"description,omitempty"`
}

// TunnelDirection is -L (local port forward) or -R (remote port forward).
type TunnelDirection string

const (
	DirectionLocal  TunnelDirection = "-L"
	DirectionRemote TunnelDirection = "-R"
)

// Tunnel defines a single ssh -L/-R port-forwarding rule tied to a Remote.
// The unique key is Name; Remote references the owning Remote by its Name.
type Tunnel struct {
	Name        string          `yaml:"name"         json:"name"`
	Remote      string          `yaml:"remote"       json:"remote"` // references Remote.Name
	Direction   TunnelDirection `yaml:"direction"    json:"direction"`
	BindAddress string          `yaml:"bind_address" json:"bind_address"`
	BindPort    int             `yaml:"bind_port"    json:"bind_port"`
	TargetHost  string          `yaml:"target_host"  json:"target_host"`
	TargetPort  int             `yaml:"target_port"  json:"target_port"`
	SSHOptions  []string        `yaml:"ssh_options"  json:"ssh_options,omitempty"`
	AutoStart   bool            `yaml:"auto_start"   json:"auto_start"`
	Description string          `yaml:"description"  json:"description,omitempty"`
}
