package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/services"
	"ssh-tunnel-service/internal/version"
)

// Options holds dependencies for the router.
type Options struct {
	Context  context.Context
	Registry *services.Registry
	Runtime  *services.Runtime
	Manager  *services.Manager
	Logger   *slog.Logger
	APIToken string
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

	r.GET("/api/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/api/bootstrap", func(c *gin.Context) {
		ip := c.ClientIP()
		if ip != "127.0.0.1" && ip != "::1" {
			c.AbortWithStatusJSON(http.StatusForbidden, apiError(errors.New("forbidden")))
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": opts.APIToken})
	})

	api := r.Group("/api")
	api.Use(tokenAuth(opts.APIToken))

	api.GET("/version", func(c *gin.Context) { c.JSON(http.StatusOK, version.Current()) })

	keys := api.Group("/keys")
	keys.GET("", listKeys(opts.Registry))
	keys.POST("", addKey(opts.Registry))
	keys.GET("/:id", getKey(opts.Registry))
	keys.PUT("/:id", updateKey(opts.Registry))
	keys.DELETE("/:id", deleteKey(opts.Registry))

	remotes := api.Group("/remotes")
	remotes.GET("", listRemotes(opts.Registry))
	remotes.POST("", addRemote(opts.Registry))
	remotes.GET("/:id", getRemote(opts.Registry))
	remotes.PUT("/:id", updateRemote(opts.Registry))
	remotes.DELETE("/:id", deleteRemote(opts.Registry))

	tunnels := api.Group("/tunnels")
	tunnels.GET("", listTunnels(opts.Registry))
	tunnels.POST("", addTunnel(opts.Registry))
	tunnels.GET("/:id", getTunnel(opts.Registry))
	tunnels.GET("/:id/command", tunnelCommand(opts.Manager))
	tunnels.PUT("/:id", updateTunnel(opts.Registry))
	tunnels.DELETE("/:id", deleteTunnel(opts.Registry))
	tunnels.POST("/:id/start", startTunnel(opts.Context, opts.Manager))
	tunnels.POST("/:id/stop", stopTunnel(opts.Manager))

	return r
}

func listKeys(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusOK, reg.ListKeys()) }
}

func getKey(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, err := reg.GetKey(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, apiError(err))
			return
		}
		c.JSON(http.StatusOK, key)
	}
}

func addKey(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input services.SSHKeyInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		if err := reg.AddKey(input); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		key, err := reg.GetKey(input.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.JSON(http.StatusCreated, key)
	}
}

func updateKey(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input services.SSHKeyInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		if err := reg.UpdateKey(c.Param("id"), input); err != nil {
			if errors.Is(err, services.ErrNotFound) {
				c.JSON(http.StatusNotFound, apiError(err))
				return
			}
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		key, err := reg.GetKey(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.JSON(http.StatusOK, key)
	}
}

func deleteKey(reg *services.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := reg.DeleteKey(c.Param("id")); err != nil {
			if errors.Is(err, services.ErrNotFound) {
				c.JSON(http.StatusNotFound, apiError(err))
				return
			}
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.Status(http.StatusNoContent)
	}
}

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
			if errors.Is(err, services.ErrNotFound) {
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
			if errors.Is(err, services.ErrNotFound) {
				c.JSON(http.StatusNotFound, apiError(err))
				return
			}
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.Status(http.StatusNoContent)
	}
}

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

func tunnelCommand(mgr *services.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		preview, err := mgr.Command(c.Param("id"))
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				c.JSON(http.StatusNotFound, apiError(err))
				return
			}
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.JSON(http.StatusOK, preview)
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
			if errors.Is(err, services.ErrNotFound) {
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
			if errors.Is(err, services.ErrNotFound) {
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

func apiError(err error) gin.H {
	return gin.H{"error": err.Error()}
}

func accessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		logger.Debug("api",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
		)
	}
}

func tokenAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || auth[len("Bearer "):] != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apiError(errors.New("unauthorized")))
			return
		}
		c.Next()
	}
}
