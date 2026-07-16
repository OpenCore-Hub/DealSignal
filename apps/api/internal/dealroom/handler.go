// Package dealroom exposes data-room HTTP endpoints.
package dealroom

import (
	"errors"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler exposes deal room endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a deal room handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterWorkspaceRoutes mounts authenticated workspace routes.
func (h *Handler) RegisterWorkspaceRoutes(r *gin.RouterGroup) {
	g := r.Group("/deal-rooms")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:roomId", h.Get)

	g.GET("/:roomId/folders", h.ListFolders)
	g.POST("/:roomId/folders", h.CreateFolder)
	g.PATCH("/:roomId/folders/*path", h.RenameFolder)
	g.DELETE("/:roomId/folders/*path", h.DeleteFolder)

	g.GET("/:roomId/documents", h.GetRoomDocuments)
	g.POST("/:roomId/documents", h.AddDocument)
	g.DELETE("/:roomId/documents/:docId", h.RemoveDocument)
	g.PATCH("/:roomId/documents/:docId", h.UpdateDocument)

	g.GET("/:roomId/members", h.ListMembers)
	g.POST("/:roomId/members", h.AddMember)
	g.DELETE("/:roomId/members/:memberId", h.RemoveMember)

	g.GET("/:roomId/access-requests", h.ListAccessRequests)
	g.POST("/:roomId/access-requests/:requestId/approve", h.ApproveRequest)
	g.POST("/:roomId/access-requests/:requestId/reject", h.RejectAccessRequest)

	g.POST("/:roomId/folder-permissions", h.SetFolderPermission)

	r.GET("/deal-room-templates", h.ListTemplates)
}

// RegisterPublicRoutes mounts public room routes.
func (h *Handler) RegisterPublicRoutes(r *gin.RouterGroup) {
	r.GET("/deal-rooms/:slug", h.PublicView)
	r.POST("/deal-rooms/:slug/access-requests", h.CreateAccessRequest)
	r.POST("/deal-rooms/:slug/nda", h.RecordNDA)
}

// CreateRequest is the JSON body for creating a room.
type CreateRequest struct {
	Slug             string                 `json:"slug" binding:"required"`
	Name             string                 `json:"name" binding:"required"`
	Description      string                 `json:"description,omitempty"`
	TemplateType     string                 `json:"template_type,omitempty"`
	Settings         map[string]interface{} `json:"settings,omitempty"`
	RequiresNDA      bool                   `json:"requires_nda,omitempty"`
	RequiresApproval bool                   `json:"requires_approval,omitempty"`
}

// List returns all deal rooms in the workspace.
func (h *Handler) List(c *gin.Context) {
	rooms, err := h.service.ListRooms(c.Request.Context(), middleware.WorkspaceIDFrom(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	out := make([]gin.H, len(rooms))
	for i, r := range rooms {
		out[i] = roomSummaryResponse(r)
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// ListTemplates returns available deal room templates.
func (h *Handler) ListTemplates(c *gin.Context) {
	out := make([]gin.H, len(roomTemplates))
	for i, t := range roomTemplates {
		folders := make([]gin.H, len(t.FolderStructure))
		for j, f := range t.FolderStructure {
			folders[j] = gin.H{
				"path":        f.Path,
				"name":        f.Name,
				"description": f.Description,
				"sort_order":  f.SortOrder,
			}
		}
		out[i] = gin.H{
			"id":                     t.ID,
			"name":                   t.Name,
			"description":            t.Description,
			"scenario":               t.Scenario,
			"folderStructure":        folders,
			"recommendedFiles":       t.RecommendedFiles,
			"defaultPermissionLevel": t.DefaultPermissionLevel,
			"ndaEnabled":             t.NDAEnabled,
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Create handles data room creation.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	room, err := h.service.CreateRoom(c.Request.Context(), middleware.UserIDFrom(c), middleware.WorkspaceIDFrom(c), CreateRoomRequest(req))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidSlug):
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_slug", "message": err.Error()})
		case errors.Is(err, ErrDuplicateSlug):
			c.JSON(http.StatusConflict, gin.H{"code": "duplicate_slug", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, roomResponse(room))
}

// Get returns a data room with full detail.
func (h *Handler) Get(c *gin.Context) {
	detail, err := h.service.GetRoomDetail(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
	)
	if err != nil {
		if errors.Is(err, ErrRoomNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "room_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, roomDetailResponse(detail))
}

// AddMemberRequest invites a member.
type AddMemberRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role,omitempty"`
}

// AddMember handles member invitation.
func (h *Handler) AddMember(c *gin.Context) {
	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}
	member, err := h.service.AddMember(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), req.Email, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrInvalidEmail):
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_email", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, memberResponse(member))
}

// ApproveRequest handles access request approval.
func (h *Handler) ApproveRequest(c *gin.Context) {
	req, err := h.service.ApproveAccessRequest(c.Request.Context(), c.Param("requestId"), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "request_not_found", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, requestResponse(req))
}

// PublicView returns a room for an authorized visitor.
func (h *Handler) PublicView(c *gin.Context) {
	email := c.Query("email")
	room, member, err := h.service.PublicAccess(c.Request.Context(), c.Param("slug"), email)
	if err != nil {
		mapPublicError(c, err)
		return
	}

	ctx := c.Request.Context()
	roomID := uuid.UUID(room.ID.Bytes).String()
	workspaceID := uuid.UUID(room.WorkspaceID.Bytes).String()

	summary, _ := h.service.GetRoomSummary(ctx, roomID, workspaceID)

	folders, _ := h.service.ListFolders(ctx, roomID, workspaceID)
	docs, err := h.service.ListDocuments(ctx, roomID, workspaceID, email)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"room":      roomSummaryResponse(summary),
		"member":    memberResponse(member),
		"folders":   folderListResponse(folders),
		"documents": folderDocsListResponse(docs),
	})
}

// AccessRequestRequest is the public access request body.
type AccessRequestRequest struct {
	Email  string `json:"email" binding:"required,email"`
	Reason string `json:"reason,omitempty"`
}

// CreateAccessRequest handles public access requests.
func (h *Handler) CreateAccessRequest(c *gin.Context) {
	var req AccessRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	request, err := h.service.CreateAccessRequest(c.Request.Context(), c.Param("slug"), req.Email, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrRoomNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "room_not_found", "message": err.Error()})
		case errors.Is(err, ErrInvalidEmail):
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_email", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, requestResponse(request))
}

// NDARequest is the NDA agreement body.
type NDARequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RecordNDA records visitor NDA agreement.
func (h *Handler) RecordNDA(c *gin.Context) {
	var req NDARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	if err := h.service.RecordNDA(c.Request.Context(), c.Param("slug"), req.Email, c.ClientIP(), c.Request.UserAgent()); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "room_not_found", "message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// AddDocumentRequest adds a document to a room.
type AddDocumentRequest struct {
	DocumentID string `json:"document_id" binding:"required"`
	FolderPath string `json:"folder_path,omitempty"`
	SortOrder  int32  `json:"sort_order,omitempty"`
}

// AddDocument handles adding a document to a room.
func (h *Handler) AddDocument(c *gin.Context) {
	var req AddDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	if req.FolderPath == "" || req.FolderPath == "/" {
		folders, ferr := h.service.ListFolders(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c))
		if ferr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": ferr.Error()})
			return
		}
		if len(folders) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "no target folder available"})
			return
		}
		req.FolderPath = folders[0].Path
	}
	doc, err := h.service.AddDocument(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), req.DocumentID, req.FolderPath, req.SortOrder)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, documentResponse(doc))
}

// SetFolderPermissionRequest sets folder permission.
type SetFolderPermissionRequest struct {
	Email      string `json:"email" binding:"required,email"`
	FolderPath string `json:"folder_path" binding:"required"`
	Permission string `json:"permission" binding:"required"`
}

// SetFolderPermission handles folder permission updates.
func (h *Handler) SetFolderPermission(c *gin.Context) {
	var req SetFolderPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	perm, err := h.service.SetFolderPermission(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), req.Email, req.FolderPath, req.Permission)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, folderPermissionResponse(perm))
}

// ListFolders returns the folder structure of a room.
func (h *Handler) ListFolders(c *gin.Context) {
	folders, err := h.service.ListFolders(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c))
	if err != nil {
		if errors.Is(err, ErrRoomNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "room_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": folderListResponse(folders)})
}

// CreateFolderRequest is the body for creating a folder.
type CreateFolderRequest struct {
	Name       string `json:"name" binding:"required"`
	ParentPath string `json:"parent_path,omitempty"`
}

// CreateFolder handles folder creation.
func (h *Handler) CreateFolder(c *gin.Context) {
	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	folders, err := h.service.CreateFolder(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), req.Name, req.ParentPath)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrFolderExists):
			c.JSON(http.StatusConflict, gin.H{"code": "folder_exists", "message": err.Error()})
		case errors.Is(err, ErrFolderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "folder_not_found", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": folderListResponse(folders)})
}

// RenameFolderRequest is the body for renaming a folder.
type RenameFolderRequest struct {
	Name string `json:"name" binding:"required"`
}

// RenameFolder handles folder renaming.
func (h *Handler) RenameFolder(c *gin.Context) {
	var req RenameFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	folders, err := h.service.RenameFolder(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), path.Join("/", c.Param("path")), req.Name)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrFolderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "folder_not_found", "message": err.Error()})
		case errors.Is(err, ErrFolderExists):
			c.JSON(http.StatusConflict, gin.H{"code": "folder_exists", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": folderListResponse(folders)})
}

// DeleteFolder handles folder deletion.
func (h *Handler) DeleteFolder(c *gin.Context) {
	folders, err := h.service.DeleteFolder(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), path.Join("/", c.Param("path")))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrFolderNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "folder_not_found", "message": err.Error()})
		case errors.Is(err, ErrFolderNotEmpty):
			c.JSON(http.StatusConflict, gin.H{"code": "folder_not_empty", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": folderListResponse(folders)})
}

// GetRoomDocuments returns documents grouped by folder for a room.
func (h *Handler) GetRoomDocuments(c *gin.Context) {
	docs, err := h.service.GetRoomDocuments(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrRoomNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "room_not_found", "message": err.Error()})
		case errors.Is(err, ErrApprovalRequired):
			c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": folderDocsListResponse(docs)})
}

// RemoveDocument removes a document from a room.
func (h *Handler) RemoveDocument(c *gin.Context) {
	if err := h.service.RemoveDocument(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), c.Param("docId")); err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// UpdateDocumentRequest updates a document's folder or sort order.
type UpdateDocumentRequest struct {
	FolderPath string `json:"folder_path,omitempty"`
	SortOrder  *int32 `json:"sort_order,omitempty"`
}

// UpdateDocument handles document folder move and reorder.
func (h *Handler) UpdateDocument(c *gin.Context) {
	var req UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	if req.FolderPath == "" && req.SortOrder == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "folder_path or sort_order required"})
		return
	}
	if req.FolderPath != "" {
		if req.FolderPath == "/" {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "folder_path cannot be root"})
			return
		}
		if err := h.service.MoveDocument(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), c.Param("docId"), req.FolderPath, req.SortOrder); err != nil {
			switch {
			case errors.Is(err, ErrNotRoomAdmin):
				c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
			case errors.Is(err, ErrFolderNotFound):
				c.JSON(http.StatusNotFound, gin.H{"code": "folder_not_found", "message": err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
			}
			return
		}
	} else {
		if err := h.service.ReorderDocuments(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), []DocumentOrder{{DocumentID: c.Param("docId"), SortOrder: *req.SortOrder}}); err != nil {
			switch {
			case errors.Is(err, ErrNotRoomAdmin):
				c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
			}
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// ListMembers returns room members.
func (h *Handler) ListMembers(c *gin.Context) {
	members, err := h.service.ListMembers(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrRoomNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "room_not_found", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": memberDetailListResponse(members)})
}

// RemoveMember removes a member from a room.
func (h *Handler) RemoveMember(c *gin.Context) {
	if err := h.service.RemoveMember(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), c.Param("memberId")); err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrMemberNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "member_not_found", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// ListAccessRequests returns pending access requests for a room.
func (h *Handler) ListAccessRequests(c *gin.Context) {
	requests, err := h.service.ListAccessRequests(c.Request.Context(), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrRoomNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "room_not_found", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": requestListResponse(requests)})
}

// RejectAccessRequest handles access request rejection.
func (h *Handler) RejectAccessRequest(c *gin.Context) {
	req, err := h.service.RejectAccessRequest(c.Request.Context(), c.Param("requestId"), c.Param("roomId"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotRoomAdmin):
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case errors.Is(err, ErrRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "request_not_found", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, requestResponse(req))
}

func mapPublicError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrRoomNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "room_not_found", "message": err.Error()})
	case errors.Is(err, ErrNDARequired):
		c.JSON(http.StatusForbidden, gin.H{"code": "nda_required", "message": err.Error()})
	case errors.Is(err, ErrApprovalRequired):
		c.JSON(http.StatusForbidden, gin.H{"code": "approval_required", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
	}
}

func roomResponse(r db.DealRoom) gin.H {
	resp := baseRoomResponse(r)
	resp["documentCount"] = 0
	resp["memberCount"] = 0
	resp["pendingApprovals"] = 0
	return resp
}

func roomSummaryResponse(r RoomSummary) gin.H {
	resp := baseRoomResponse(r.Room)
	resp["documentCount"] = r.DocumentCount
	resp["memberCount"] = r.MemberCount
	resp["pendingApprovals"] = r.PendingApprovals
	resp["visitorCount"] = r.VisitorCount
	resp["unreadQuestions"] = r.UnreadQuestions
	resp["heatScore"] = r.HeatScore
	if r.LastAccessedAt.Valid {
		resp["lastAccessedAt"] = r.LastAccessedAt.Time.Format(time.RFC3339)
	}
	return resp
}

func roomDetailResponse(r RoomDetail) gin.H {
	resp := baseRoomResponse(r.Room)
	resp["documentCount"] = r.DocumentCount
	resp["memberCount"] = r.MemberCount
	resp["pendingApprovals"] = r.PendingApprovals
	resp["folders"] = folderListResponse(r.Folders)
	resp["documents"] = folderDocsListResponse(r.Documents)
	resp["members"] = memberDetailListResponse(r.Members)
	resp["accessRequests"] = requestListResponse(r.AccessRequests)
	return resp
}

func baseRoomResponse(r db.DealRoom) gin.H {
	template := ""
	if r.TemplateType.Valid {
		template = strings.ReplaceAll(r.TemplateType.String, "_", "-")
	}
	resp := gin.H{
		"id":               uuid.UUID(r.ID.Bytes).String(),
		"slug":             r.Slug,
		"name":             r.Name,
		"template":         template,
		"ndaEnabled":       r.RequiresNda,
		"requiresApproval": r.RequiresApproval,
		"status":           r.Status,
		"createdAt":        r.CreatedAt.Time.Format(time.RFC3339),
		"updatedAt":        r.UpdatedAt.Time.Format(time.RFC3339),
		"lastAccessedAt":   nil,
	}
	if r.Description.Valid {
		resp["description"] = r.Description.String
	}
	return resp
}

func memberResponse(m db.RoomMember) gin.H {
	resp := gin.H{
		"id":         uuid.UUID(m.ID.Bytes).String(),
		"email":      m.Email,
		"role":       m.Role,
		"nda_status": m.NdaStatus,
		"status":     m.Status,
	}
	if m.NdaSignedAt.Valid {
		resp["nda_signed_at"] = m.NdaSignedAt.Time.Format(time.RFC3339)
	}
	return resp
}

func requestResponse(r db.RoomAccessRequest) gin.H {
	resp := gin.H{
		"id":     uuid.UUID(r.ID.Bytes).String(),
		"email":  r.Email,
		"status": r.Status,
	}
	if r.Reason.Valid {
		resp["reason"] = r.Reason.String
	}
	if r.ReviewedAt.Valid {
		resp["reviewed_at"] = r.ReviewedAt.Time.Format(time.RFC3339)
	}
	return resp
}

func documentResponse(d db.DealRoomDocument) gin.H {
	return gin.H{
		"id":          uuid.UUID(d.ID.Bytes).String(),
		"document_id": uuid.UUID(d.DocumentID.Bytes).String(),
		"folder_path": d.FolderPath,
		"sort_order":  d.SortOrder,
		"created_at":  d.CreatedAt.Time.Format(time.RFC3339),
	}
}

func folderPermissionResponse(p db.RoomMemberFolderPermission) gin.H {
	return gin.H{
		"id":          uuid.UUID(p.ID.Bytes).String(),
		"email":       p.Email,
		"folder_path": p.FolderPath,
		"permission":  p.Permission,
	}
}

func folderResponse(f Folder) gin.H {
	resp := gin.H{
		"path":       f.Path,
		"name":       f.Name,
		"sort_order": f.SortOrder,
	}
	if f.Description != "" {
		resp["description"] = f.Description
	}
	return resp
}

func folderListResponse(folders []Folder) []gin.H {
	out := make([]gin.H, len(folders))
	for i, f := range folders {
		out[i] = folderResponse(f)
	}
	return out
}

func documentMetaResponse(d RoomDocument) gin.H {
	resp := gin.H{
		"id":          d.ID,
		"document_id": d.DocumentID,
		"title":       d.Title,
		"folder_path": d.FolderPath,
		"sort_order":  d.SortOrder,
		"source_type": d.SourceType,
		"status":      d.Status,
		"created_at":  d.CreatedAt.Format(time.RFC3339),
	}
	if d.PageCount > 0 {
		resp["page_count"] = d.PageCount
	}
	if d.FileSize > 0 {
		resp["file_size"] = d.FileSize
	}
	return resp
}

func folderDocsResponse(fd FolderDocs) gin.H {
	docs := make([]gin.H, len(fd.Documents))
	for i, d := range fd.Documents {
		docs[i] = documentMetaResponse(d)
	}
	return gin.H{
		"folder":     fd.Folder.Path,
		"permission": fd.Permission,
		"documents":  docs,
	}
}

func folderDocsListResponse(list []FolderDocs) []gin.H {
	out := make([]gin.H, len(list))
	for i, fd := range list {
		out[i] = folderDocsResponse(fd)
	}
	return out
}

func memberDetailResponse(m RoomMemberDetail) gin.H {
	resp := gin.H{
		"id":         uuid.UUID(m.ID.Bytes).String(),
		"email":      m.Email,
		"role":       m.Role,
		"nda_status": m.NdaStatus,
		"status":     m.Status,
	}
	if m.UserName != "" {
		resp["name"] = m.UserName
	}
	if m.NdaSignedAt.Valid {
		resp["nda_signed_at"] = m.NdaSignedAt.Time.Format(time.RFC3339)
	}
	return resp
}

func memberDetailListResponse(members []RoomMemberDetail) []gin.H {
	out := make([]gin.H, len(members))
	for i, m := range members {
		out[i] = memberDetailResponse(m)
	}
	return out
}

func requestListResponse(requests []db.RoomAccessRequest) []gin.H {
	out := make([]gin.H, len(requests))
	for i, r := range requests {
		out[i] = requestResponse(r)
	}
	return out
}
