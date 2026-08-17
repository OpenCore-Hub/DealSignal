package workspace

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAuthMiddlewareClaimsOnlyDealRoomPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	wsID := newPGUUID()
	roomID := uuid.New()
	userID := uuid.NewString()
	fake := &fakeDB{
		t:           t,
		actorUserID: userID,
		actorEmail:  "janedoe@gmail.com",
		workspace: db.Workspace{
			ID:        wsID,
			TenantID:  newPGUUID(),
			Name:      "Acme",
			Slug:      "acme",
			CreatedAt: pgtype.Timestamptz{Valid: true},
		},
		claimRows: []roomMemberClaim{{
			WorkspaceID: wsID,
			RoomID:      pgtype.UUID{Bytes: roomID, Valid: true},
			Email:       "jane.doe+vdr@gmail.com",
			Status:      "pending",
		}},
	}
	svc := NewService(db.New(fake))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})
	r.GET("/api/workspaces/:workspaceSlug", AuthMiddleware(svc), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/api/workspaces/:workspaceSlug/deal-rooms/:roomId", AuthMiddleware(svc), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	root := httptest.NewRecorder()
	r.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/api/workspaces/acme", nil))
	if root.Code != http.StatusForbidden {
		t.Fatalf("workspace root=%d want 403", root.Code)
	}
	if fake.claimRows[0].UserID.Valid {
		t.Fatal("workspace root must not consume invite")
	}

	room := httptest.NewRecorder()
	r.ServeHTTP(room, httptest.NewRequest(http.MethodGet, "/api/workspaces/acme/deal-rooms/"+roomID.String(), nil))
	if room.Code != http.StatusOK {
		t.Fatalf("deal-room deep-link=%d want 200", room.Code)
	}
	if !fake.claimRows[0].UserID.Valid {
		t.Fatal("deal-room path must bind plus-tag invite")
	}
}
