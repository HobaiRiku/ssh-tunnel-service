package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/HobaiRiku/ssh-tunnel-service/internal/config"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/services"
	"github.com/HobaiRiku/ssh-tunnel-service/internal/version"
)

// Options holds dependencies for the router.
type Options struct {
	Context  context.Context
	Registry *services.Registry
	Runtime  *services.Runtime
	Manager  *services.Manager
	Logger   *slog.Logger
}

// NewRouter builds the gin engine with all API routes mounted.
func NewRouter(opts Options) *gin.Engine {
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(accessLog(opts.Logger))

	// Health / version
	r.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/api/version", func(c *gin.Context) { c.JSON(http.StatusOK, version.Current()) })

	// Topology (read-only)
	r.GET("/api/topology", topologyHandler(opts.Registry))

	// Remotes CRUD
	remotes := r.Group("/api/remotes")
	remotes.GET("", listRemotes(opts.Registry))
	remotes.POST("", addRemote(opts.Registry))
	remotes.GET("/:id", getRemote(opts.Registry))
	remotes.PUT("/:id", updateRemote(opts.Registry))
	remotes.DELETE("/:id", deleteRemote(opts.Registry))

	// Tunnels CRUD + actions
	tunnels := r.Group("/api/tunnels")
	tunnels.GET("", listTunnels(opts.Registry))
	tunnels.POST("", addTunnel(opts.Registry))
	tunnels.GET("/:id", getTunnel(opts.Registry))
	tunnels.PUT("/:id", updateTunnel(opts.Registry))
	tunnels.DELETE("/:id", deleteTunnel(opts.Registry))
	tunnels.POST("/:id/start", startTunnel(opts.Context, opts.Manager))
	tunnels.POST("/:id/stop", stopTunnel(opts.Manager))

	return r
}

// ── Remotes ──────────────────────────────────────────────────────────────────

func listRemotes(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusOK, reg.ListRemotes()) }
}

func getRemote(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		remote, err := reg.GetRemote(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, apiError(err))
			return
		}
		c.JSON(http.StatusOK, remote)
	}
}

func addRemote(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input config.Remote
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		if err := reg.AddRemote(input); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.JSON(http.StatusCreated, input)
	}
}

func updateRemote(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input config.Remote
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		if err := reg.UpdateRemote(c.Param("id"), input); err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, apiError(err))
				return
			}
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.JSON(http.StatusOK, input)
	}
}

func deleteRemote(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := reg.DeleteRemote(c.Param("id")); err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, apiError(err))
				return
			}
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ── Tunnels ───────────────────────────────────────────────────────────────────

func listTunnels(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusOK, reg.ListTunnels()) }
}

func getTunnel(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ts, err := reg.GetTunnel(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, apiError(err))
			return
		}
		c.JSON(http.StatusOK, ts)
	}
}

func addTunnel(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input config.Tunnel
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		if err := reg.AddTunnel(input); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.JSON(http.StatusCreated, input)
	}
}

func updateTunnel(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input config.Tunnel
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		if err := reg.UpdateTunnel(c.Param("id"), input); err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, apiError(err))
				return
			}
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.JSON(http.StatusOK, input)
	}
}

func deleteTunnel(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := reg.DeleteTunnel(c.Param("id")); err != nil {
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, apiError(err))
				return
			}
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func startTunnel(ctx context.Context, mgr *services.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := mgr.Start(ctx, c.Param("id")); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "started"})
	}
}

func stopTunnel(mgr *services.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := mgr.Stop(c.Param("id")); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "stopped"})
	}
}

// ── Topology ─────────────────────────────────────────────────────────────────

type topoNode struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Data     any          `json:"data"`
	Position topoPosition `json:"position"`
}

type topoEdge struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Label    string `json:"label"`
	Animated bool   `json:"animated"`
}

type topoPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type remoteNodeData struct {
	Label string `json:"label"`
	Host  string `json:"host"`
	Port  int    `json:"port"`
	User  string `json:"user,omitempty"`
}

type tunnelNodeData struct {
	Label       string `json:"label"`
	Direction   string `json:"direction"`
	BindAddress string `json:"bindAddress"`
	BindPort    int    `json:"bindPort"`
	State       string `json:"state"`
}

type targetNodeData struct {
	Label string `json:"label"`
	Host  string `json:"host"`
	Port  int    `json:"port"`
}

func topologyHandler(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		remotes := reg.ListRemotes()
		tunnels := reg.ListTunnels()

		nodes := make([]topoNode, 0, len(remotes)+len(tunnels)*2)
		edges := make([]topoEdge, 0, len(tunnels)*2)
		for i, r := range remotes {
			y := float64(i * 180)
			nodes = append(nodes, topoNode{
				ID:   "remote-" + r.ID,
				Type: "remote",
				Data: remoteNodeData{
					Label: r.Name,
					Host:  r.Host,
					Port:  r.Port,
					User:  r.User,
				},
				Position: topoPosition{X: 0, Y: y},
			})
		}
		for i, ts := range tunnels {
			tunnelNodeID := "tunnel-" + ts.ID
			targetNodeID := "target-" + ts.ID
			y := float64(i * 180)
			nodes = append(nodes, topoNode{
				ID:   tunnelNodeID,
				Type: "tunnel",
				Data: tunnelNodeData{
					Label:       ts.Name,
					Direction:   string(ts.Direction),
					BindAddress: ts.BindAddress,
					BindPort:    ts.BindPort,
					State:       string(ts.State),
				},
				Position: topoPosition{X: 320, Y: y},
			})
			nodes = append(nodes, topoNode{
				ID:   targetNodeID,
				Type: "target",
				Data: targetNodeData{
					Label: fmt.Sprintf("%s:%d", ts.TargetHost, ts.TargetPort),
					Host:  ts.TargetHost,
					Port:  ts.TargetPort,
				},
				Position: topoPosition{X: 640, Y: y},
			})
			// edge: tunnel → remote
			edges = append(edges, topoEdge{
				ID:       "e-" + ts.ID + "-remote",
				Source:   tunnelNodeID,
				Target:   "remote-" + ts.RemoteID,
				Label:    string(ts.Direction),
				Animated: true,
			})
			// edge: tunnel → target
			edges = append(edges, topoEdge{
				ID:       "e-" + ts.ID + "-target",
				Source:   tunnelNodeID,
				Target:   targetNodeID,
				Label:    fmt.Sprintf(":%d", ts.BindPort),
				Animated: true,
			})
		}

		c.JSON(http.StatusOK, gin.H{"nodes": nodes, "edges": edges})
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func apiError(err error) gin.H {
	return gin.H{"error": err.Error()}
}

func accessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		logger.Info("api",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
		)
	}
}
