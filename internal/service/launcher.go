package service

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
)

type SSHLauncher struct {
	mu   sync.Mutex
	cmds map[string]*exec.Cmd
}

func NewSSHLauncher() *SSHLauncher {
	return &SSHLauncher{cmds: map[string]*exec.Cmd{}}
}

func (l *SSHLauncher) Launch(ctx context.Context, id string, args []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.cmds[id]; exists {
		return fmt.Errorf("commd %q already running", id)
	}
	cmd := exec.CommandContext(ctx, "ssh", args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	l.cmds[id] = cmd
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("ssh command %s exited with error: %v", id, err)
		}
		l.mu.Lock()
		if current, ok := l.cmds[id]; ok && current == cmd {
			delete(l.cmds, id)
		}
		l.mu.Unlock()
	}()
	return nil
}

func (l *SSHLauncher) Stop(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	cmd, exists := l.cmds[id]
	if !exists {
		return fmt.Errorf("commd %q is not running", id)
	}
	if cmd.Process == nil {
		delete(l.cmds, id)
		return nil
	}
	if err := cmd.Process.Kill(); err != nil {
		return err
	}
	delete(l.cmds, id)
	return nil
}
