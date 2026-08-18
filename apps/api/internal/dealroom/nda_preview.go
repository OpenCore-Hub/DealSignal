package dealroom

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/roomacl"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxMemberNDAPreviewPages = 50

const memberNDAPreviewTTL = 15 * time.Minute

// MemberNDAPreview is the bound room agreement a pending member must read
// before SignMemberNDA. It is not attached to GetRoomDetail (pendingNDAShell
// still strips template/document IDs and description).
type MemberNDAPreview struct {
	NDATemplate     MemberNDATemplateMeta `json:"ndaTemplate"`
	Document        MemberNDADocumentMeta `json:"document"`
	PreviewPageURLs []string              `json:"previewPageUrls"`
	DocumentURL     string                `json:"documentUrl,omitempty"`
	SignerEmail     string                `json:"signerEmail"`
	ExpiresAt       string                `json:"expiresAt"`
}

// MemberNDATemplateMeta is the bound template the member will stamp on sign.
type MemberNDATemplateMeta struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ContentSHA256    string `json:"contentSha256"`
	SourceDocumentID string `json:"sourceDocumentId"`
}

// MemberNDADocumentMeta is the source document for in-page preview.
type MemberNDADocumentMeta struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	PageCount  int32  `json:"pageCount"`
	SourceType string `json:"sourceType"`
}

type ndaSigner struct {
	email         string
	sessionEmail  string
	userID        pgtype.UUID
	alreadyActive bool
}

// PreviewMemberNDA returns the room's bound NDA for a pending invitee.
// NDA-off rooms, already-active members, and missing bindings are not found.
func (s *Service) PreviewMemberNDA(ctx context.Context, roomID, workspaceID, userID string) (MemberNDAPreview, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return MemberNDAPreview{}, err
	}
	if !room.RequiresNda {
		return MemberNDAPreview{}, ErrNDANotRequired
	}
	signer, err := s.resolveNDASigner(ctx, room, userID)
	if err != nil {
		return MemberNDAPreview{}, err
	}
	if signer.alreadyActive {
		return MemberNDAPreview{}, ErrNDANotRequired
	}

	tpl, doc, err := s.loadBoundMemberNDA(ctx, room)
	if err != nil {
		return MemberNDAPreview{}, err
	}

	docID := uuid.UUID(doc.ID.Bytes).String()
	previewPageURLs := make([]string, 0, 8)
	pages, pagesErr := s.queries.ListPagesByDocument(ctx, doc.ID)
	if pagesErr == nil {
		for _, page := range pages {
			if !page.ImageObjectKey.Valid || page.ImageObjectKey.String == "" {
				continue
			}
			if u := s.signMemberNDAFileURL(page.ImageObjectKey.String, roomID, userID); u != "" {
				previewPageURLs = append(previewPageURLs, u)
			}
			if len(previewPageURLs) >= maxMemberNDAPreviewPages {
				break
			}
		}
	}
	documentURL := ""
	if signed := s.signMemberNDAFileURL(doc.StorageKey, roomID, userID); signed != "" {
		documentURL = signed + "&disposition=inline"
	}
	if len(previewPageURLs) == 0 && documentURL == "" {
		return MemberNDAPreview{}, ErrNDAPreviewUnavailable
	}

	signerEmail := signer.sessionEmail
	if signerEmail == "" {
		signerEmail = signer.email
	}

	return MemberNDAPreview{
		NDATemplate: MemberNDATemplateMeta{
			ID:               uuid.UUID(tpl.ID.Bytes).String(),
			Name:             tpl.Name,
			ContentSHA256:    tpl.ContentSha256,
			SourceDocumentID: uuid.UUID(tpl.SourceDocumentID.Bytes).String(),
		},
		Document: MemberNDADocumentMeta{
			ID:         docID,
			Title:      doc.Title,
			PageCount:  doc.PageCount.Int32,
			SourceType: doc.SourceType,
		},
		PreviewPageURLs: previewPageURLs,
		DocumentURL:     documentURL,
		SignerEmail:     signerEmail,
		ExpiresAt:       time.Now().Add(memberNDAPreviewTTL).UTC().Format(time.RFC3339),
	}, nil
}

func (s *Service) resolveNDASigner(ctx context.Context, room db.DealRoom, userID string) (ndaSigner, error) {
	uid := pgUUID(userID)
	user, err := s.queries.GetUserByID(ctx, uid)
	if err != nil {
		return ndaSigner{}, ErrMemberNotFound
	}
	sessionEmail := strings.ToLower(strings.TrimSpace(user.Email))

	caps, err := roomacl.Resolve(ctx, s.queries, room.WorkspaceID, room.ID, userID)
	if err != nil {
		return ndaSigner{}, err
	}
	if caps.View && !caps.InvitedPending {
		return ndaSigner{sessionEmail: sessionEmail, userID: uid, alreadyActive: true}, nil
	}
	signEmail := strings.ToLower(strings.TrimSpace(caps.MemberEmail))
	if !caps.InvitedPending {
		if sessionEmail == "" {
			return ndaSigner{}, ErrMemberNotFound
		}
		member, merr := s.findRoomMemberByMailbox(ctx, room.ID, sessionEmail)
		if merr != nil || !member.ID.Valid {
			return ndaSigner{}, ErrMemberNotFound
		}
		if member.Status == "active" {
			return ndaSigner{email: member.Email, sessionEmail: sessionEmail, userID: uid, alreadyActive: true}, nil
		}
		if member.Status != "pending" {
			return ndaSigner{}, ErrMemberNotFound
		}
		signEmail = member.Email
	}
	if strings.TrimSpace(signEmail) == "" {
		return ndaSigner{}, ErrMemberNotFound
	}
	return ndaSigner{email: signEmail, sessionEmail: sessionEmail, userID: uid}, nil
}

func (s *Service) loadBoundMemberNDA(ctx context.Context, room db.DealRoom) (db.NdaTemplate, db.GetDocumentByIDRow, error) {
	tpl, err := s.loadBoundMemberNDATemplate(ctx, room)
	if err != nil {
		return db.NdaTemplate{}, db.GetDocumentByIDRow{}, err
	}
	if !tpl.SourceDocumentID.Valid {
		return db.NdaTemplate{}, db.GetDocumentByIDRow{}, ErrNDAAgreementRequired
	}
	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          tpl.SourceDocumentID,
		WorkspaceID: room.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.NdaTemplate{}, db.GetDocumentByIDRow{}, ErrNDAAgreementRequired
		}
		return db.NdaTemplate{}, db.GetDocumentByIDRow{}, err
	}
	if IsArchivedDocumentStatus(doc.Status) {
		return db.NdaTemplate{}, db.GetDocumentByIDRow{}, ErrNDAAgreementRequired
	}
	return tpl, doc, nil
}

func (s *Service) loadBoundMemberNDATemplate(ctx context.Context, room db.DealRoom) (db.NdaTemplate, error) {
	if !roomHasMemberNDA(room) {
		return db.NdaTemplate{}, ErrNDAAgreementRequired
	}
	tplID := room.NdaTemplateID
	if !tplID.Valid && room.NdaDocumentID.Valid {
		resolved, _, err := s.ensureRoomNDATemplateFromDocument(
			ctx,
			room,
			"",
			uuid.UUID(room.NdaDocumentID.Bytes).String(),
		)
		if err != nil {
			return db.NdaTemplate{}, err
		}
		tplID = resolved
	}
	if !tplID.Valid {
		return db.NdaTemplate{}, ErrNDAAgreementRequired
	}
	tpl, err := s.queries.GetNDATemplateByID(ctx, db.GetNDATemplateByIDParams{
		ID:          tplID,
		WorkspaceID: room.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.NdaTemplate{}, ErrNDAAgreementRequired
		}
		return db.NdaTemplate{}, err
	}
	if tpl.Status != "active" {
		return db.NdaTemplate{}, ErrNDAAgreementRequired
	}
	return tpl, nil
}

func (s *Service) matchMemberNDAContent(ctx context.Context, room db.DealRoom, contentSHA256 string) error {
	tpl, err := s.loadBoundMemberNDATemplate(ctx, room)
	if err != nil {
		return err
	}
	have := strings.ToLower(strings.TrimSpace(tpl.ContentSha256))
	if have == "" {
		return nil
	}
	want := strings.ToLower(strings.TrimSpace(contentSHA256))
	if want == "" || !hmac.Equal([]byte(have), []byte(want)) {
		return ErrNDAContentMismatch
	}
	return nil
}

func (s *Service) boundMemberNDAContentSHA(ctx context.Context, workspaceID, tplID pgtype.UUID) string {
	if !tplID.Valid {
		return ""
	}
	tpl, err := s.queries.GetNDATemplateByID(ctx, db.GetNDATemplateByIDParams{
		ID:          tplID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(tpl.ContentSha256)
}

// signMemberNDAFileURL builds the same HMAC proxy contract as link.SignResource
// so <img> tags can hit /api/v1/public/files/signed without importing link
// (link already imports dealroom).
func (s *Service) signMemberNDAFileURL(storageKey, roomID, userID string) string {
	if s.cfg == nil {
		return ""
	}
	secret := strings.TrimSpace(s.cfg.URLSigningSecret)
	baseURL := strings.TrimSpace(s.cfg.AppBaseURL)
	if secret == "" || baseURL == "" || strings.TrimSpace(storageKey) == "" {
		return ""
	}
	return signMemberNDAResource(secret, storageKey, "room-nda:"+roomID, userID, baseURL, memberNDAPreviewTTL)
}

func signMemberNDAResource(secret, storageKey, token, userID, baseURL string, ttl time.Duration) string {
	expires := time.Now().Add(ttl).Unix()
	payload := fmt.Sprintf("%s|%s|%s|%d", storageKey, token, userID, expires)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))

	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	u.Path = "/api/v1/public/files/signed"
	q := u.Query()
	q.Set("key", base64.URLEncoding.EncodeToString([]byte(storageKey)))
	q.Set("token", token)
	q.Set("expires", strconv.FormatInt(expires, 10))
	q.Set("vid", userID)
	q.Set("sig", sig)
	u.RawQuery = q.Encode()
	return u.String()
}
