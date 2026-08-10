package knowledge

import (
	"context"
	"sync"

	"github.com/gin-gonic/gin"
)

// askLease binds L0 single-flight to the full ask lifecycle (through audit).
// Client disconnect cancels upstream RAG via workCtx but keeps the admission
// slot until the handler finishes and release() runs.
type askLease struct {
	h          *Handler
	roomID     string
	userID     string
	workCtx    context.Context
	workCancel context.CancelFunc
	releaseOnce sync.Once
	cancelOnce  sync.Once
}

func (h *Handler) acquireAskLease(c *gin.Context, roomID, userID, transport string) (*askLease, error) {
	// Capture request context before spawning: gin may recycle *gin.Context
	// after ServeHTTP returns, and reading c.Request from that goroutine races.
	reqCtx := c.Request.Context()
	if err := h.admitAsk(reqCtx, roomID, userID, transport); err != nil {
		return nil, err
	}
	workCtx, workCancel := context.WithCancel(reqCtx)
	lease := &askLease{
		h:          h,
		roomID:     roomID,
		userID:     userID,
		workCtx:    workCtx,
		workCancel: workCancel,
	}
	go func() {
		<-reqCtx.Done()
		lease.cancelWork()
	}()
	return lease, nil
}

func (l *askLease) cancelWork() {
	l.cancelOnce.Do(func() {
		l.workCancel()
	})
}

func (l *askLease) release() {
	l.releaseOnce.Do(func() {
		l.h.releaseAsk(context.Background(), l.roomID, l.userID)
	})
}
