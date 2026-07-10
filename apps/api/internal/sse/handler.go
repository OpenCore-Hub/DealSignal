package sse

import (
	"fmt"
	"net/http"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handler provides SSE endpoints.
type Handler struct {
	hub *Hub
}

// NewHandler creates an SSE handler.
func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

// StreamEvents connects a client to the SSE event stream for a link.
// Channel: sse:workspace:{workspaceID}
func (h *Handler) StreamEvents(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)

	// SSE headers.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	channel := fmt.Sprintf("sse:workspace:%s", workspaceID)
	clientCh := make(chan Event, 32)
	stop := h.hub.Subscribe(c.Request.Context(), channel, clientCh)
	defer stop()

	// Heartbeat ticker.
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Send initial connected event.
	c.SSEvent("connected", map[string]interface{}{"workspaceId": workspaceID, "timestamp": time.Now().Unix()})
	flusher.Flush()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			c.SSEvent("heartbeat", time.Now().Unix())
			flusher.Flush()
		case event := <-clientCh:
			c.SSEvent(event.Type, nil) // payload sent as data
			// Re-send with data for structured payloads
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, event.Payload)
			flusher.Flush()
		}
	}
}
