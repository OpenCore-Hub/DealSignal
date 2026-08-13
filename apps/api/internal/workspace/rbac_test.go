package workspace

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHasCapabilityMatrix(t *testing.T) {
	cases := []struct {
		role, cap string
		want      bool
	}{
		{RoleGuest, CapabilityReadContent, true},
		{RoleGuest, CapabilityWriteContent, false},
		{RoleGuest, CapabilityManageWorkspace, false},
		{RoleMember, CapabilityWriteContent, true},
		{RoleMember, CapabilityManageWorkspace, false},
		{RoleAdmin, CapabilityManageMembers, true},
		{RoleOwner, CapabilityManageWorkspace, true},
	}
	for _, tc := range cases {
		if got := HasCapability(tc.role, tc.cap); got != tc.want {
			t.Fatalf("HasCapability(%q,%q)=%v want %v", tc.role, tc.cap, got, tc.want)
		}
	}
}

func withRole(role string, register func(*gin.Engine)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("workspaceRole", role)
		c.Next()
	})
	r.Use(RBACMiddleware())
	register(r)
	return r
}

func TestRBACMiddlewareGuestCannotWrite(t *testing.T) {
	r := withRole(RoleGuest, func(r *gin.Engine) {
		r.POST("/api/workspaces/demo/links", func(c *gin.Context) { c.Status(http.StatusCreated) })
	})
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/demo/links", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRBACMiddlewareMemberCanWriteContent(t *testing.T) {
	r := withRole(RoleMember, func(r *gin.Engine) {
		r.POST("/api/workspaces/demo/links", func(c *gin.Context) { c.Status(http.StatusCreated) })
	})
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/demo/links", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestRBACMiddlewareMemberCannotManageSettings(t *testing.T) {
	r := withRole(RoleMember, func(r *gin.Engine) {
		r.PUT("/api/workspaces/demo/settings", func(c *gin.Context) { c.Status(http.StatusOK) })
		r.GET("/api/workspaces/demo/settings", func(c *gin.Context) { c.Status(http.StatusOK) })
		r.GET("/api/workspaces/demo/members", func(c *gin.Context) { c.Status(http.StatusOK) })
	})

	putReq := httptest.NewRequest(http.MethodPut, "/api/workspaces/demo/settings", nil)
	putW := httptest.NewRecorder()
	r.ServeHTTP(putW, putReq)
	if putW.Code != http.StatusForbidden {
		t.Fatalf("PUT settings expected 403, got %d", putW.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/workspaces/demo/settings", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET settings expected 200 for member chrome, got %d", getW.Code)
	}

	memReq := httptest.NewRequest(http.MethodGet, "/api/workspaces/demo/members", nil)
	memW := httptest.NewRecorder()
	r.ServeHTTP(memW, memReq)
	if memW.Code != http.StatusForbidden {
		t.Fatalf("GET members expected 403 for non-manager, got %d", memW.Code)
	}
}

func TestRBACMiddlewareGuestCanReadDealRooms(t *testing.T) {
	r := withRole(RoleGuest, func(r *gin.Engine) {
		r.GET("/api/workspaces/demo/deal-rooms", func(c *gin.Context) { c.Status(http.StatusOK) })
	})
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/demo/deal-rooms", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET deal-rooms expected 200 for guest, got %d", w.Code)
	}
}

func TestRBACMiddlewareGuestCannotWriteDealRooms(t *testing.T) {
	r := withRole(RoleGuest, func(r *gin.Engine) {
		r.POST("/api/workspaces/demo/deal-rooms", func(c *gin.Context) { c.Status(http.StatusCreated) })
	})
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/demo/deal-rooms", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST deal-rooms expected 403 for guest, got %d", w.Code)
	}
}

func TestRBACMiddlewareDealRoomMembersNotManagerPath(t *testing.T) {
	r := withRole(RoleMember, func(r *gin.Engine) {
		r.POST("/api/workspaces/demo/deal-rooms/r1/members", func(c *gin.Context) { c.Status(http.StatusCreated) })
	})
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/demo/deal-rooms/r1/members", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("deal-room members must not be treated as workspace manage path, got %d", w.Code)
	}
}

func TestRBACMiddlewareUnknownMutateFailsClosedForMember(t *testing.T) {
	r := withRole(RoleMember, func(r *gin.Engine) {
		r.POST("/api/workspaces/demo/future-admin", func(c *gin.Context) { c.Status(http.StatusCreated) })
	})
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/demo/future-admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("unknown mutate resource must fail closed for member, got %d", w.Code)
	}
}

func TestRBACMiddlewareUnknownMutateAllowedForManager(t *testing.T) {
	r := withRole(RoleAdmin, func(r *gin.Engine) {
		r.POST("/api/workspaces/demo/future-admin", func(c *gin.Context) { c.Status(http.StatusCreated) })
	})
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/demo/future-admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("manager may mutate unknown resource, got %d", w.Code)
	}
}
