// Package dealroom exposes data-room HTTP endpoints.
package dealroom

import (
	"errors"
	"net/http"
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
	g.GET("/:id", h.Get)
	g.POST("/:id/members", h.AddMember)
	g.POST("/:id/access-requests/:requestId/approve", h.ApproveRequest)
	g.POST("/:id/documents", h.AddDocument)
	g.POST("/:id/folder-permissions", h.SetFolderPermission)

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
	out := []gin.H{
		{
			"id":                     "tmpl_seed",
			"name":                   "Seed Round Due Diligence",
			"description":            "Standard due diligence room for seed investors, including deck, financial model, team and legal documents.",
			"scenario":               "seed",
			"folderStructure": []gin.H{
				{"name": "01 Pitch Deck", "description": "Latest fundraising deck"},
				{"name": "02 Financials", "description": "Historical financials, projections and key assumptions"},
				{"name": "03 Team", "description": "Founder resumes, org chart and hiring plan"},
				{"name": "04 Product", "description": "Product demo, roadmap and technical architecture"},
				{"name": "05 Legal", "description": "Incorporation, shareholder agreement and option plan"},
			},
			"recommendedFiles":       []string{"Pitch Deck.pdf", "Financial Model.xlsx", "Cap Table"},
			"defaultPermissionLevel": "medium",
			"ndaEnabled":             false,
		},
		{
			"id":                     "tmpl_series_a",
			"name":                   "Series A Data Room",
			"description":            "In-depth due diligence for Series A firms, emphasizing product data, growth metrics and financial transparency.",
			"scenario":               "series-a",
			"folderStructure": []gin.H{
				{"name": "01 Executive Summary"},
				{"name": "02 Growth Metrics"},
				{"name": "03 Financials & Projections"},
				{"name": "04 Customer Proof"},
				{"name": "05 Product & Tech"},
				{"name": "06 Legal & Compliance"},
			},
			"recommendedFiles":       []string{"Investor Deck.pdf", "Metrics Dashboard", "ARR Waterfall"},
			"defaultPermissionLevel": "high",
			"ndaEnabled":             true,
		},
		{
			"id":                     "tmpl_lp_update",
			"name":                   "LP Quarterly Update",
			"description":            "Quarterly performance, distribution and operations report for LPs, with bulk distribution and access tracking.",
			"scenario":               "lp-update",
			"folderStructure": []gin.H{
				{"name": "01 GP Letter"},
				{"name": "02 Performance & IRR"},
				{"name": "03 Portfolio Updates"},
				{"name": "04 Distributions & Capital Calls"},
				{"name": "05 Governance & AML/KYC"},
			},
			"recommendedFiles":       []string{"GP Letter.pdf", "LP Report.xlsx"},
			"defaultPermissionLevel": "low",
			"ndaEnabled":             false,
		},
		{
			"id":                     "tmpl_sales_proposal",
			"name":                   "Enterprise Sales Proposal",
			"description":            "Proposal data room for enterprise procurement committees, including proposal, case studies, security and implementation plan.",
			"scenario":               "sales-proposal",
			"folderStructure": []gin.H{
				{"name": "01 Proposal"},
				{"name": "02 ROI & Business Case"},
				{"name": "03 Case Studies"},
				{"name": "04 Security & Compliance"},
				{"name": "05 Implementation Plan"},
			},
			"recommendedFiles":       []string{"Proposal.pdf", "Security Whitepaper", "Implementation Plan"},
			"defaultPermissionLevel": "medium",
			"ndaEnabled":             true,
		},
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

// Get returns a data room.
func (h *Handler) Get(c *gin.Context) {
	room, err := h.service.GetRoom(c.Request.Context(), c.Param("id"), middleware.WorkspaceIDFrom(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "room_not_found", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, roomResponse(room))
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
	member, err := h.service.AddMember(c.Request.Context(), c.Param("id"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), req.Email, req.Role)
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
	req, err := h.service.ApproveAccessRequest(c.Request.Context(), c.Param("requestId"), c.Param("id"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c))
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

	docs, err := h.service.ListDocuments(c.Request.Context(), uuid.UUID(room.ID.Bytes).String(), email)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"room":    roomResponse(room),
		"member":  memberResponse(member),
		"documents": documentList(docs),
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
	if req.FolderPath == "" {
		req.FolderPath = "/"
	}
	doc, err := h.service.AddDocument(c.Request.Context(), c.Param("id"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), req.DocumentID, req.FolderPath, req.SortOrder)
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
	perm, err := h.service.SetFolderPermission(c.Request.Context(), c.Param("id"), middleware.WorkspaceIDFrom(c), middleware.UserIDFrom(c), req.Email, req.FolderPath, req.Permission)
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
	return resp
}

func baseRoomResponse(r db.DealRoom) gin.H {
	template := ""
	if r.TemplateType.Valid {
		template = strings.ReplaceAll(r.TemplateType.String, "_", "-")
	}
	resp := gin.H{
		"id":                uuid.UUID(r.ID.Bytes).String(),
		"slug":              r.Slug,
		"name":              r.Name,
		"template":          template,
		"ndaEnabled":        r.RequiresNda,
		"requiresApproval":  r.RequiresApproval,
		"status":            r.Status,
		"createdAt":         r.CreatedAt.Time.Format(time.RFC3339),
		"updatedAt":         r.UpdatedAt.Time.Format(time.RFC3339),
		"lastAccessedAt":    nil,
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
		"id":           uuid.UUID(d.ID.Bytes).String(),
		"document_id":  uuid.UUID(d.DocumentID.Bytes).String(),
		"folder_path":  d.FolderPath,
		"sort_order":   d.SortOrder,
		"created_at":   d.CreatedAt.Time.Format(time.RFC3339),
	}
}

func documentList(docs []db.DealRoomDocument) []gin.H {
	out := make([]gin.H, len(docs))
	for i, d := range docs {
		out[i] = documentResponse(d)
	}
	return out
}

func folderPermissionResponse(p db.RoomMemberFolderPermission) gin.H {
	return gin.H{
		"id":          uuid.UUID(p.ID.Bytes).String(),
		"email":       p.Email,
		"folder_path": p.FolderPath,
		"permission":  p.Permission,
	}
}
