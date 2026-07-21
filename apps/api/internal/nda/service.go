// Package nda implements One-Click NDA templates, agreement responses, and
// sealed PDF generation with an Audit Trail page.
package nda

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

var (
	ErrTemplateNotFound      = errors.New("nda template not found")
	ErrTemplateArchived      = errors.New("nda template is archived")
	ErrTemplateHasResponses  = errors.New("nda template has signatures and cannot change source")
	ErrInvalidSignerName     = errors.New("signer name is required")
	ErrAgreementNotFound     = errors.New("nda agreement not found")
	ErrContentHashMismatch   = errors.New("nda content hash mismatch")
)

const maxSignerNameLen = 200

// Service manages NDA templates and sealed agreement artifacts.
type Service struct {
	queries *db.Queries
	storage *storage.Client
	mailer  mailer.Mailer
}

func NewService(q *db.Queries, st *storage.Client, m mailer.Mailer) *Service {
	return &Service{queries: q, storage: st, mailer: m}
}

// TemplateView is the API shape for an NDA template.
type TemplateView struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	SourceDocumentID   string `json:"source_document_id"`
	ContentSHA256      string `json:"content_sha256"`
	RequireSignerName  bool   `json:"require_signer_name"`
	Status             string `json:"status"`
	ResponseCount      int64  `json:"response_count"`
	LinkCount          int64  `json:"link_count"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// ResponseView is the API shape for a signed NDA response.
type ResponseView struct {
	ID             string `json:"id"`
	LinkID         string `json:"link_id"`
	TemplateID     string `json:"nda_template_id"`
	Email          string `json:"email"`
	SignerName     string `json:"signer_name"`
	CertificateID  string `json:"certificate_id"`
	ContentSHA256  string `json:"content_sha256"`
	HasSignedFile  bool   `json:"has_signed_file"`
	SignedAt       string `json:"signed_at"`
	Status         string `json:"status"`
}

func pgUUID(id string) pgtype.UUID {
	u, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

// HashDocumentContent downloads the document object and returns SHA-256 hex.
func (s *Service) HashDocumentContent(ctx context.Context, storageKey string) (string, error) {
	if s.storage == nil || storageKey == "" {
		return "", nil
	}
	obj, err := s.storage.GetObject(ctx, storageKey)
	if err != nil {
		return "", fmt.Errorf("get object: %w", err)
	}
	defer obj.Close()
	h := sha256.New()
	if _, err := io.Copy(h, obj); err != nil {
		return "", fmt.Errorf("hash object: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// EnsureTemplateFromDocument creates or returns an active template for a workspace document.
func (s *Service) EnsureTemplateFromDocument(
	ctx context.Context,
	tenantID, workspaceID pgtype.UUID,
	documentID string,
	createdBy pgtype.UUID,
	nameHint string,
) (db.NdaTemplate, error) {
	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return db.NdaTemplate{}, fmt.Errorf("invalid document id: %w", err)
	}
	docPG := pgtype.UUID{Bytes: docUUID, Valid: true}

	existing, err := s.queries.GetNDATemplateBySourceDocument(ctx, db.GetNDATemplateBySourceDocumentParams{
		WorkspaceID:      workspaceID,
		SourceDocumentID: docPG,
	})
	if err == nil {
		if existing.Status == "archived" {
			return db.NdaTemplate{}, ErrTemplateArchived
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.NdaTemplate{}, fmt.Errorf("get template by document: %w", err)
	}

	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          docPG,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.NdaTemplate{}, fmt.Errorf("document not found")
		}
		return db.NdaTemplate{}, fmt.Errorf("get document: %w", err)
	}

	hash, _ := s.HashDocumentContent(ctx, doc.StorageKey)
	name := strings.TrimSpace(nameHint)
	if name == "" {
		name = strings.TrimSpace(doc.Title)
	}
	if name == "" {
		name = "NDA Agreement"
	}

	tpl, err := s.queries.CreateNDATemplate(ctx, db.CreateNDATemplateParams{
		TenantID:          tenantID,
		WorkspaceID:       workspaceID,
		Name:              name,
		SourceDocumentID:  docPG,
		ContentSha256:     hash,
		RequireSignerName: true,
		Status:            "active",
		CreatedBy:         createdBy,
	})
	if err != nil {
		// Race: another request created the same unique (workspace, source).
		existing, gerr := s.queries.GetNDATemplateBySourceDocument(ctx, db.GetNDATemplateBySourceDocumentParams{
			WorkspaceID:      workspaceID,
			SourceDocumentID: docPG,
		})
		if gerr == nil {
			return existing, nil
		}
		return db.NdaTemplate{}, fmt.Errorf("create nda template: %w", err)
	}
	return tpl, nil
}

// CreateTemplate creates a template from a workspace document.
func (s *Service) CreateTemplate(ctx context.Context, userID, workspaceID, documentID, name string, requireSignerName bool) (TemplateView, error) {
	ws, err := s.queries.GetWorkspaceByID(ctx, pgUUID(workspaceID))
	if err != nil {
		return TemplateView{}, fmt.Errorf("get workspace: %w", err)
	}
	tpl, err := s.EnsureTemplateFromDocument(ctx, ws.TenantID, ws.ID, documentID, pgUUID(userID), name)
	if err != nil {
		return TemplateView{}, err
	}
	if name != "" && name != tpl.Name {
		updated, uerr := s.queries.UpdateNDATemplate(ctx, db.UpdateNDATemplateParams{
			Name:              name,
			RequireSignerName: requireSignerName,
			ID:                tpl.ID,
			WorkspaceID:       ws.ID,
		})
		if uerr == nil {
			tpl = updated
		}
	} else if !requireSignerName && tpl.RequireSignerName {
		updated, uerr := s.queries.UpdateNDATemplate(ctx, db.UpdateNDATemplateParams{
			Name:              tpl.Name,
			RequireSignerName: false,
			ID:                tpl.ID,
			WorkspaceID:       ws.ID,
		})
		if uerr == nil {
			tpl = updated
		}
	}
	return s.toTemplateView(ctx, tpl)
}

// ListTemplates returns templates for a workspace.
func (s *Service) ListTemplates(ctx context.Context, workspaceID string, includeArchived bool) ([]TemplateView, error) {
	wsID := pgUUID(workspaceID)
	var rows []db.NdaTemplate
	var err error
	if includeArchived {
		rows, err = s.queries.ListAllNDATemplatesByWorkspace(ctx, wsID)
	} else {
		rows, err = s.queries.ListNDATemplatesByWorkspace(ctx, db.ListNDATemplatesByWorkspaceParams{
			WorkspaceID: wsID,
			Status:      "active",
		})
	}
	if err != nil {
		return nil, err
	}
	out := make([]TemplateView, 0, len(rows))
	for _, row := range rows {
		v, verr := s.toTemplateView(ctx, row)
		if verr != nil {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// GetTemplate returns one template.
func (s *Service) GetTemplate(ctx context.Context, workspaceID, templateID string) (TemplateView, error) {
	tpl, err := s.queries.GetNDATemplateByID(ctx, db.GetNDATemplateByIDParams{
		ID:          pgUUID(templateID),
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TemplateView{}, ErrTemplateNotFound
		}
		return TemplateView{}, err
	}
	return s.toTemplateView(ctx, tpl)
}

// UpdateTemplate updates mutable fields (name / require_signer_name). Source is immutable.
func (s *Service) UpdateTemplate(ctx context.Context, workspaceID, templateID, name string, requireSignerName bool) (TemplateView, error) {
	tpl, err := s.queries.UpdateNDATemplate(ctx, db.UpdateNDATemplateParams{
		Name:              strings.TrimSpace(name),
		RequireSignerName: requireSignerName,
		ID:                pgUUID(templateID),
		WorkspaceID:       pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TemplateView{}, ErrTemplateNotFound
		}
		return TemplateView{}, err
	}
	return s.toTemplateView(ctx, tpl)
}

// ArchiveTemplate soft-archives a template.
func (s *Service) ArchiveTemplate(ctx context.Context, workspaceID, templateID string) (TemplateView, error) {
	tpl, err := s.queries.ArchiveNDATemplate(ctx, db.ArchiveNDATemplateParams{
		ID:          pgUUID(templateID),
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TemplateView{}, ErrTemplateNotFound
		}
		return TemplateView{}, err
	}
	return s.toTemplateView(ctx, tpl)
}

// ListResponses returns signed responses for a template.
func (s *Service) ListResponses(ctx context.Context, workspaceID, templateID string) ([]ResponseView, error) {
	rows, err := s.queries.ListLinkNDAAgreementsByTemplate(ctx, db.ListLinkNDAAgreementsByTemplateParams{
		WorkspaceID:   pgUUID(workspaceID),
		NdaTemplateID: pgUUID(templateID),
	})
	if err != nil {
		return nil, err
	}
	out := make([]ResponseView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toResponseView(row))
	}
	return out, nil
}

// ListLinkResponses returns signed responses for a link.
func (s *Service) ListLinkResponses(ctx context.Context, workspaceID, linkID string) ([]ResponseView, error) {
	rows, err := s.queries.ListLinkNDAAgreementsByLink(ctx, db.ListLinkNDAAgreementsByLinkParams{
		WorkspaceID: pgUUID(workspaceID),
		LinkID:      pgUUID(linkID),
	})
	if err != nil {
		return nil, err
	}
	out := make([]ResponseView, 0, len(rows))
	for _, row := range rows {
		out = append(out, toResponseView(row))
	}
	return out, nil
}

// GetResponse returns a response by id within a workspace.
func (s *Service) GetResponse(ctx context.Context, workspaceID, responseID string) (db.LinkNdaAgreement, error) {
	row, err := s.queries.GetLinkNDAAgreementByID(ctx, db.GetLinkNDAAgreementByIDParams{
		ID:          pgUUID(responseID),
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.LinkNdaAgreement{}, ErrAgreementNotFound
		}
		return db.LinkNdaAgreement{}, err
	}
	return row, nil
}

// OpenSignedFile streams the sealed PDF for a response.
func (s *Service) OpenSignedFile(ctx context.Context, row db.LinkNdaAgreement) (io.ReadCloser, string, error) {
	if row.SignedFileKey == "" {
		return nil, "", ErrAgreementNotFound
	}
	obj, err := s.storage.GetObject(ctx, row.SignedFileKey)
	if err != nil {
		return nil, "", err
	}
	filename := fmt.Sprintf("nda-signed-%s.pdf", row.CertificateID)
	return obj, filename, nil
}

func (s *Service) toTemplateView(ctx context.Context, tpl db.NdaTemplate) (TemplateView, error) {
	respCount, _ := s.queries.CountNDATemplateResponses(ctx, tpl.ID)
	linkCount, _ := s.queries.CountNDATemplateLinks(ctx, tpl.ID)
	return TemplateView{
		ID:                uuidStr(tpl.ID),
		Name:              tpl.Name,
		SourceDocumentID:  uuidStr(tpl.SourceDocumentID),
		ContentSHA256:     tpl.ContentSha256,
		RequireSignerName: tpl.RequireSignerName,
		Status:            tpl.Status,
		ResponseCount:     respCount,
		LinkCount:         linkCount,
		CreatedAt:         tpl.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt:         tpl.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}, nil
}

func toResponseView(row db.LinkNdaAgreement) ResponseView {
	signedAt := ""
	if row.SignedAt.Valid {
		signedAt = row.SignedAt.Time.UTC().Format(time.RFC3339)
	}
	return ResponseView{
		ID:            uuidStr(row.ID),
		LinkID:        uuidStr(row.LinkID),
		TemplateID:    uuidStr(row.NdaTemplateID),
		Email:         row.Email.String,
		SignerName:    row.SignerName,
		CertificateID: row.CertificateID,
		ContentSHA256: row.ContentSha256,
		HasSignedFile: row.SignedFileKey != "",
		SignedAt:      signedAt,
		Status:        row.Status,
	}
}

// NormalizeSignerName trims and validates a typed signer name.
func NormalizeSignerName(name string, required bool) (string, error) {
	trimmed := strings.TrimSpace(name)
	if required && trimmed == "" {
		return "", ErrInvalidSignerName
	}
	if utf8.RuneCountInString(trimmed) > maxSignerNameLen {
		return "", fmt.Errorf("%w: max %d characters", ErrInvalidSignerName, maxSignerNameLen)
	}
	return trimmed, nil
}

// SealParams holds inputs for sealed PDF generation.
type SealParams struct {
	TemplateName  string
	CertificateID string
	SignerName    string
	SignerEmail   string
	ContentSHA256 string
	LinkID        string
	IPHash        string
	UserAgent     string
	SignedAt      time.Time
}

// SealAgreementPDF merges the source PDF with an Audit Trail page and uploads it.
func (s *Service) SealAgreementPDF(ctx context.Context, tenantID, workspaceID, responseID, sourceKey string, params SealParams) (string, error) {
	if s.storage == nil {
		return "", nil
	}
	obj, err := s.storage.GetObject(ctx, sourceKey)
	if err != nil {
		return "", fmt.Errorf("get source pdf: %w", err)
	}
	defer obj.Close()
	srcBytes, err := io.ReadAll(obj)
	if err != nil {
		return "", fmt.Errorf("read source pdf: %w", err)
	}

	auditPDF, err := buildAuditTrailPDF(params)
	if err != nil {
		return "", fmt.Errorf("build audit pdf: %w", err)
	}

	var sealed bytes.Buffer
	conf := model.NewDefaultConfiguration()
	readers := []io.ReadSeeker{
		bytes.NewReader(srcBytes),
		bytes.NewReader(auditPDF),
	}
	if err := api.MergeRaw(readers, &sealed, false, conf); err != nil {
		// Non-PDF sources: store audit page alone as evidence.
		logger.InfoCtx(ctx, "nda seal merge failed; storing audit page only",
			logger.Attr("error", err.Error()),
			logger.Attr("response_id", responseID),
		)
		sealed.Reset()
		sealed.Write(auditPDF)
	}

	key := storage.ObjectKey(tenantID, workspaceID, responseID, "nda-signed.pdf")
	if err := s.storage.PutObject(ctx, key, bytes.NewReader(sealed.Bytes()), int64(sealed.Len()), "application/pdf"); err != nil {
		return "", fmt.Errorf("upload sealed pdf: %w", err)
	}
	if _, err := s.queries.UpdateLinkNDAAgreementSignedFile(ctx, db.UpdateLinkNDAAgreementSignedFileParams{
		SignedFileKey: key,
		ID:            pgUUID(responseID),
	}); err != nil {
		return "", fmt.Errorf("update signed file key: %w", err)
	}
	return key, nil
}

// NotifySigned sends best-effort emails to signer and link owner.
// When signedFileKey is set, the sealed PDF is attached for the signer only.
func (s *Service) NotifySigned(ctx context.Context, signerEmail, ownerEmail, agreementName, certificateID, linkName, signedFileKey string) {
	if s.mailer == nil {
		return
	}
	subject := fmt.Sprintf("NDA signed: %s", agreementName)
	body := fmt.Sprintf(
		"The confidentiality agreement %q was accepted.\n\nCertificate ID: %s\nLink: %s\n\nYour signed copy is attached.\n",
		agreementName, certificateID, linkName,
	)
	ownerBody := fmt.Sprintf(
		"The confidentiality agreement %q was accepted.\n\nCertificate ID: %s\nLink: %s\nSigner: %s\n",
		agreementName, certificateID, linkName, signerEmail,
	)

	var signerAttachments []mailer.Attachment
	if signedFileKey != "" && s.storage != nil {
		obj, err := s.storage.GetObject(ctx, signedFileKey)
		if err != nil {
			logger.InfoCtx(ctx, "nda signed email: load sealed pdf failed",
				logger.Attr("error", err.Error()),
				logger.Attr("key", signedFileKey),
			)
		} else {
			data, readErr := io.ReadAll(obj)
			_ = obj.Close()
			if readErr != nil {
				logger.InfoCtx(ctx, "nda signed email: read sealed pdf failed",
					logger.Attr("error", readErr.Error()),
				)
			} else if len(data) > 0 {
				filename := "nda-signed.pdf"
				if agreementName = strings.TrimSpace(agreementName); agreementName != "" {
					filename = sanitizeAttachmentFilename(agreementName) + "-signed.pdf"
				}
				signerAttachments = []mailer.Attachment{{
					Filename:    filename,
					ContentType: "application/pdf",
					Content:     data,
				}}
			}
		}
	}

	signerEmail = strings.TrimSpace(signerEmail)
	if signerEmail != "" {
		_, err := s.mailer.SendEmail(ctx, mailer.EmailJob{
			EmailType:   mailer.EmailTypeCustom,
			Recipient:   signerEmail,
			Subject:     subject,
			Body:        body,
			Attachments: signerAttachments,
		})
		if err != nil {
			logger.InfoCtx(ctx, "nda signed email failed",
				logger.Attr("to", signerEmail),
				logger.Attr("error", err.Error()),
			)
		}
	}

	ownerEmail = strings.TrimSpace(ownerEmail)
	if ownerEmail != "" && !strings.EqualFold(ownerEmail, signerEmail) {
		_, err := s.mailer.SendEmail(ctx, mailer.EmailJob{
			EmailType: mailer.EmailTypeCustom,
			Recipient: ownerEmail,
			Subject:   subject,
			Body:      ownerBody,
		})
		if err != nil {
			logger.InfoCtx(ctx, "nda signed email failed",
				logger.Attr("to", ownerEmail),
				logger.Attr("error", err.Error()),
			)
		}
	}
}

func sanitizeAttachmentFilename(name string) string {
	name = strings.TrimSpace(name)
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "", "<", "", ">", "", "|", "-")
	name = replacer.Replace(name)
	name = strings.Trim(name, " .")
	if name == "" {
		return "nda"
	}
	if utf8.RuneCountInString(name) > 80 {
		runes := []rune(name)
		name = string(runes[:80])
	}
	return name
}
