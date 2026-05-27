package domain

import (
	"fmt"
	"strings"
)

type TunnelDirection string

const (
	DirectionRemote TunnelDirection = "-R"
	DirectionLocal  TunnelDirection = "-L"
)

type Remote struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Description string `json:"description,omitempty"`
}

func (r Remote) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("remote id is required")
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("remote name is required")
	}
	if strings.TrimSpace(r.Host) == "" {
		return fmt.Errorf("remote host is required")
	}
	if r.Port <= 0 || r.Port > 65535 {
		return fmt.Errorf("remote port must be between 1 and 65535")
	}
	if strings.TrimSpace(r.User) == "" {
		return fmt.Errorf("remote user is required")
	}
	return nil
}

type Commd struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	RemoteID    string          `json:"remoteId"`
	Direction   TunnelDirection `json:"direction"`
	BindAddress string          `json:"bindAddress"`
	BindPort    int             `json:"bindPort"`
	TargetHost  string          `json:"targetHost"`
	TargetPort  int             `json:"targetPort"`
	SSHOptions  []string        `json:"sshOptions,omitempty"`
	AutoStart   bool            `json:"autoStart"`
}

func (c Commd) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("commd id is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("commd name is required")
	}
	if strings.TrimSpace(c.RemoteID) == "" {
		return fmt.Errorf("commd remoteId is required")
	}
	if c.Direction != DirectionRemote && c.Direction != DirectionLocal {
		return fmt.Errorf("commd direction must be -R or -L")
	}
	if strings.TrimSpace(c.BindAddress) == "" {
		return fmt.Errorf("commd bindAddress is required")
	}
	if c.BindPort <= 0 || c.BindPort > 65535 {
		return fmt.Errorf("commd bindPort must be between 1 and 65535")
	}
	if strings.TrimSpace(c.TargetHost) == "" {
		return fmt.Errorf("commd targetHost is required")
	}
	if c.TargetPort <= 0 || c.TargetPort > 65535 {
		return fmt.Errorf("commd targetPort must be between 1 and 65535")
	}
	return nil
}
