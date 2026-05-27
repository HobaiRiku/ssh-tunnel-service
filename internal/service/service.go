package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/domain"
)

type Launcher interface {
	Launch(ctx context.Context, id string, args []string) error
	Stop(id string) error
}

type InMemoryService struct {
	mu       sync.RWMutex
	remotes  map[string]domain.Remote
	commds   map[string]domain.Commd
	launcher Launcher
}

func New(launcher Launcher) *InMemoryService {
	return &InMemoryService{
		remotes:  map[string]domain.Remote{},
		commds:   map[string]domain.Commd{},
		launcher: launcher,
	}
}

func (s *InMemoryService) AddRemote(remote domain.Remote) error {
	if err := remote.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.remotes[remote.ID]; exists {
		return fmt.Errorf("remote %q already exists", remote.ID)
	}
	s.remotes[remote.ID] = remote
	return nil
}

func (s *InMemoryService) ListRemotes() []domain.Remote {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.Remote, 0, len(s.remotes))
	for _, remote := range s.remotes {
		result = append(result, remote)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func (s *InMemoryService) AddCommd(commd domain.Commd) error {
	if err := commd.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.remotes[commd.RemoteID]; !ok {
		return fmt.Errorf("remote %q not found", commd.RemoteID)
	}
	if _, exists := s.commds[commd.ID]; exists {
		return fmt.Errorf("commd %q already exists", commd.ID)
	}
	s.commds[commd.ID] = commd
	return nil
}

func (s *InMemoryService) ListCommds() []domain.Commd {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.Commd, 0, len(s.commds))
	for _, commd := range s.commds {
		result = append(result, commd)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func (s *InMemoryService) BuildSSHArgs(commdID string) ([]string, error) {
	s.mu.RLock()
	commd, ok := s.commds[commdID]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("commd %q not found", commdID)
	}
	remote, ok := s.remotes[commd.RemoteID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("remote %q not found", commd.RemoteID)
	}

	forward := fmt.Sprintf("%s:%d:%s:%d", commd.BindAddress, commd.BindPort, commd.TargetHost, commd.TargetPort)
	args := []string{"-N", string(commd.Direction), forward}
	args = append(args, commd.SSHOptions...)
	args = append(args, "-p", fmt.Sprintf("%d", remote.Port), fmt.Sprintf("%s@%s", remote.User, remote.Host))
	return args, nil
}

func (s *InMemoryService) LaunchCommd(ctx context.Context, commdID string) error {
	if s.launcher == nil {
		return fmt.Errorf("launcher is not configured")
	}
	args, err := s.BuildSSHArgs(commdID)
	if err != nil {
		return err
	}
	return s.launcher.Launch(ctx, commdID, args)
}

func (s *InMemoryService) StopCommd(commdID string) error {
	if s.launcher == nil {
		return fmt.Errorf("launcher is not configured")
	}
	return s.launcher.Stop(commdID)
}

func (s *InMemoryService) AutoStart(ctx context.Context) error {
	for _, commd := range s.ListCommds() {
		if !commd.AutoStart {
			continue
		}
		if err := s.LaunchCommd(ctx, commd.ID); err != nil {
			return fmt.Errorf("autostart %s: %w", commd.ID, err)
		}
	}
	return nil
}

func (s *InMemoryService) TopologyMermaid() string {
	remotes := s.ListRemotes()
	commds := s.ListCommds()
	var b strings.Builder
	b.WriteString("graph LR\n")
	for _, remote := range remotes {
		fmt.Fprintf(&b, "remote_%s[\"%s\\n%s@%s:%d\"]\n", remote.ID, escape(remote.Name), escape(remote.User), escape(remote.Host), remote.Port)
	}
	for _, commd := range commds {
		arrow := "-->|-L|"
		if commd.Direction == domain.DirectionRemote {
			arrow = "-->|-R|"
		}
		fmt.Fprintf(&b, "commd_%s((\"%s\"))\n", commd.ID, escape(commd.Name))
		fmt.Fprintf(&b, "commd_%s %s remote_%s\n", commd.ID, arrow, commd.RemoteID)
		fmt.Fprintf(&b, "target_%s[\"%s:%d\"]\n", commd.ID, escape(commd.TargetHost), commd.TargetPort)
		b.WriteString("commd_" + commd.ID + " --> target_" + commd.ID + "\n")
	}
	return b.String()
}

func escape(v string) string {
	return strings.NewReplacer("\"", "'", "\n", " ").Replace(v)
}
