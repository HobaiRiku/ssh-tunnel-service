package daemon

import (
	"fmt"
	"runtime"
)

type Manager interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Status() string
}

func New(serviceName string) Manager {
	return &platformManager{serviceName: serviceName, os: runtime.GOOS}
}

type platformManager struct {
	serviceName string
	os          string
}

func (m *platformManager) Install() error {
	return fmt.Errorf("daemon install is not automated for %s yet; integrate with system manager manually for service %q", m.os, m.serviceName)
}

func (m *platformManager) Uninstall() error {
	return fmt.Errorf("daemon uninstall is not automated for %s yet; remove service %q manually", m.os, m.serviceName)
}

func (m *platformManager) Start() error {
	return fmt.Errorf("daemon start is not automated for %s yet; start service %q manually", m.os, m.serviceName)
}

func (m *platformManager) Stop() error {
	return fmt.Errorf("daemon stop is not automated for %s yet; stop service %q manually", m.os, m.serviceName)
}

func (m *platformManager) Status() string {
	return fmt.Sprintf("daemon manager ready for %s with service %q", m.os, m.serviceName)
}
