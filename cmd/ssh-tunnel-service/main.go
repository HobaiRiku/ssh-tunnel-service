package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/daemon"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/httpapi"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return serve([]string{})
	}
	command := args[0]
	commandArgs := args[1:]

	switch command {
	case "serve":
		return serve(commandArgs)
	case "skill":
		return skill()
	case "daemon":
		return daemonCmd(commandArgs)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", ":8080", "http listen address")
	autostart := fs.Bool("autostart", false, "launch all autoStart commd on startup")
	if err := fs.Parse(args); err != nil {
		return err
	}

	svc := service.New(service.NewSSHLauncher())
	if *autostart {
		if err := svc.AutoStart(context.Background()); err != nil {
			return err
		}
	}
	api, err := httpapi.New(svc)
	if err != nil {
		return err
	}
	log.Printf("ssh tunnel service listening on %s", *addr)
	return http.ListenAndServe(*addr, api.Handler())
}

func skill() error {
	doc := map[string]any{
		"name":        "ssh-tunnel-operator",
		"description": "AI-ready CLI skill for creating and managing SSH -R/-L tunnel commands",
		"commands": []map[string]string{
			{"command": "serve", "usage": "ssh-tunnel-service serve --addr :8080"},
			{"command": "daemon status", "usage": "ssh-tunnel-service daemon status"},
			{"command": "HTTP API", "usage": "POST /api/remotes, POST /api/commds, POST /api/commds/{id}/launch"},
		},
		"objects": []string{"remote", "commd"},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func daemonCmd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: daemon <install|uninstall|start|stop|status>")
	}
	mgr := daemon.New("ssh-tunnel-service")
	switch strings.ToLower(args[0]) {
	case "install":
		return mgr.Install()
	case "uninstall":
		return mgr.Uninstall()
	case "start":
		return mgr.Start()
	case "stop":
		return mgr.Stop()
	case "status":
		fmt.Println(mgr.Status())
		return nil
	default:
		return fmt.Errorf("unknown daemon command %q", args[0])
	}
}
