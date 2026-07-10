package sse

import (
	"context"
	"encoding/json"
	"fmt"
)

// LinkPublisher adapts the SSE Hub to the link.EventPublisher interface.
type LinkPublisher struct {
	hub *Hub
}

// NewLinkPublisher creates a new link event publisher.
func NewLinkPublisher(hub *Hub) *LinkPublisher {
	return &LinkPublisher{hub: hub}
}

// PublishLinkEvent sends a link-scoped event to the SSE stream.
func (p *LinkPublisher) PublishLinkEvent(ctx context.Context, workspaceID, linkID string, eventType string, payload any) {
	channel := fmt.Sprintf("sse:workspace:%s", workspaceID)
	data, _ := json.Marshal(payload)
	_ = p.hub.Publish(ctx, channel, Event{
		Type:    eventType,
		Payload: data,
	})
}
