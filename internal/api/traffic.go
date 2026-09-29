package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"ssh-tunnel-service/internal/services"
)

// trafficSnapshot serves GET /api/traffic: the current traffic of every metered
// tunnel plus the aggregate. ?history=true also returns the retained per-second
// samples (the last five minutes) for charts.
func trafficSnapshot(rt *services.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, rt.TrafficSnapshot(c.Query("history") == "true"))
	}
}

// trafficUsage serves GET /api/traffic/usage: today's, this month's and
// all-time traffic per tunnel and in aggregate.
func trafficUsage(rt *services.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, rt.TrafficUsage(time.Now()))
	}
}

// trafficHistory serves GET /api/traffic/history?range=24h|30d|1y[&tunnel=name]:
// long-term traffic as per-minute, per-hour or per-day buckets. Without a
// tunnel it returns the aggregate of every tunnel, including removed ones.
func trafficHistory(reg *services.Registry, rt *services.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Query("tunnel")
		if name != "" {
			if _, err := reg.GetTunnel(name); err != nil {
				c.JSON(http.StatusNotFound, apiError(err))
				return
			}
		}
		h, err := rt.TrafficHistoryFor(name, services.UsageRange(c.DefaultQuery("range", "24h")), time.Now())
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError(err))
			return
		}
		c.JSON(http.StatusOK, h)
	}
}

// streamTraffic serves GET /api/traffic/stream: a WebSocket that first sends a
// snapshot including history (unless history=false), then one snapshot per
// sample interval as JSON text frames, until either side goes away.
func streamTraffic(serviceCtx context.Context, rt *services.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		withHistory := c.Query("history") != "false"

		conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// A hijacked connection is not cancelled by http.Server.Shutdown, so the
		// stream also stops when the service itself is shutting down.
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		go func() {
			select {
			case <-serviceCtx.Done():
				cancel()
			case <-ctx.Done():
			}
		}()
		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					cancel()
					return
				}
			}
		}()

		samples, unsubscribe := rt.SubscribeTraffic()
		defer unsubscribe()

		if err := conn.WriteJSON(rt.TrafficSnapshot(withHistory)); err != nil {
			return
		}
		for {
			select {
			case <-ctx.Done():
				_ = conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
					time.Now().Add(time.Second))
				return
			case snap := <-samples:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteJSON(snap); err != nil {
					return
				}
			}
		}
	}
}
