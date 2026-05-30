// Package config defines the typed YAML schema for ~/.ssh-tunnel-service/config.yaml.
package config

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
}

// SSHKey is a managed private key stored beneath the runtime home directory.
type SSHKey struct {
	ID          string `yaml:"id"          json:"id"`
	Name        string `yaml:"name"        json:"name"`
	File        string `yaml:"file"        json:"file"`
	Description string `yaml:"description" json:"description,omitempty"`
}

// Remote is a reusable SSH target server definition.
type Remote struct {
	ID          string `yaml:"id"          json:"id"`
	Name        string `yaml:"name"        json:"name"`
	Host        string `yaml:"host"        json:"host"`
	Port        int    `yaml:"port"        json:"port"`
	User        string `yaml:"user"        json:"user"`
	KeyID       string `yaml:"key_id"      json:"key_id,omitempty"`
	Description string `yaml:"description" json:"description,omitempty"`
}

// TunnelDirection is -L (local port forward) or -R (remote port forward).
type TunnelDirection string

const (
	DirectionLocal  TunnelDirection = "-L"
	DirectionRemote TunnelDirection = "-R"
)

// Tunnel defines a single ssh -L/-R port-forwarding rule tied to a Remote.
type Tunnel struct {
	ID          string          `yaml:"id"           json:"id"`
	Name        string          `yaml:"name"         json:"name"`
	RemoteID    string          `yaml:"remote_id"    json:"remote_id"`
	Direction   TunnelDirection `yaml:"direction"    json:"direction"`
	BindAddress string          `yaml:"bind_address" json:"bind_address"`
	BindPort    int             `yaml:"bind_port"    json:"bind_port"`
	TargetHost  string          `yaml:"target_host"  json:"target_host"`
	TargetPort  int             `yaml:"target_port"  json:"target_port"`
	SSHOptions  []string        `yaml:"ssh_options"  json:"ssh_options,omitempty"`
	AutoStart   bool            `yaml:"auto_start"   json:"auto_start"`
	Description string          `yaml:"description"  json:"description,omitempty"`
}
