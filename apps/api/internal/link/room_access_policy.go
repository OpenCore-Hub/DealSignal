package link

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	// ErrRoomAccessPolicyInvalid is returned when room security policy fields fail validation.
	ErrRoomAccessPolicyInvalid = errors.New("invalid room access policy")
	// ErrRoomSecurityFloor is returned when a share link violates room-mandated floors.
	ErrRoomSecurityFloor = errors.New("room security floor violated")
)

// RoomAccessPolicy is the thin deal-room security surface:
// room-wide blocklist + optional outbound floors (must verify / must NDA).
// Allowlists and full protection toggles belong on each share link.
type RoomAccessPolicy struct {
	DealRoomID                     string   `json:"dealRoomId"`
	Configured                     bool     `json:"configured"`
	RequireEmailVerificationFloor  bool     `json:"requireEmailVerificationFloor"`
	RequireNdaFloor                bool     `json:"requireNdaFloor"`
	BlockedEmails                  []string `json:"blockedEmails"`
	UpdatedAt                      string   `json:"updatedAt,omitempty"`

	// Legacy wire fields — always zero/empty so old clients do not treat the room
	// page as a second full access-control form.
	RequireEmail                bool     `json:"requireEmail"`
	RequireEmailVerification    bool     `json:"requireEmailVerification"`
	RequirePassword             bool     `json:"requirePassword"`
	HasPassword                 bool     `json:"hasPassword"`
	RequireNDA                  bool     `json:"requireNda"`
	WatermarkEnabled            bool     `json:"watermarkEnabled"`
	DownloadEnabled             bool     `json:"downloadEnabled"`
	ScreenshotProtectionEnabled bool     `json:"screenshotProtectionEnabled"`
	FileRequestsEnabled         bool     `json:"fileRequestsEnabled"`
	IndexFileEnabled            bool     `json:"indexFileEnabled"`
	QaEnabled                   bool     `json:"qaEnabled"`
	AllowedEmails               []string `json:"allowedEmails"`
}

// UpsertRoomAccessPolicyRequest is the owner payload for Room Security.
type UpsertRoomAccessPolicyRequest struct {
	RequireEmailVerificationFloor bool
	RequireNdaFloor               bool
	BlockedEmails                 []string
}

func defaultRoomAccessPolicy(dealRoomID string) RoomAccessPolicy {
	return RoomAccessPolicy{
		DealRoomID:    dealRoomID,
		Configured:    false,
		BlockedEmails: []string{},
		AllowedEmails: []string{},
	}
}

func normalizePolicyEmailList(emails []string) ([]string, error) {
	out := make([]string, 0, len(emails))
	seen := make(map[string]struct{}, len(emails))
	for _, raw := range emails {
		v := strings.TrimSpace(strings.ToLower(raw))
		if v == "" {
			continue
		}
		if _, err := mail.ParseAddress(v); err != nil {
			return nil, fmt.Errorf("%w: invalid email %q", ErrRoomAccessPolicyInvalid, raw)
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out, nil
}

func blockedEmailsRemoved(previous, next []string) []string {
	prevSet := make(map[string]struct{}, len(previous))
	for _, raw := range previous {
		v := strings.TrimSpace(strings.ToLower(raw))
		if v == "" {
			continue
		}
		prevSet[v] = struct{}{}
	}
	nextSet := make(map[string]struct{}, len(next))
	for _, v := range next {
		nextSet[v] = struct{}{}
	}
	removed := make([]string, 0)
	for email := range prevSet {
		if _, stillBlocked := nextSet[email]; stillBlocked {
			continue
		}
		removed = append(removed, email)
	}
	return removed
}

func dbRoomAccessPolicyToDomain(row db.DealRoomAccessPolicy) RoomAccessPolicy {
	blocked := row.BlockedEmails
	if blocked == nil {
		blocked = []string{}
	}
	out := RoomAccessPolicy{
		DealRoomID:                    uuid.UUID(row.DealRoomID.Bytes).String(),
		Configured:                    row.Configured,
		RequireEmailVerificationFloor: row.RequireEmailVerification,
		RequireNdaFloor:               row.RequireNda,
		BlockedEmails:                 blocked,
		AllowedEmails:                 []string{},
		// Mirror floors onto legacy keys for older FE builds during rollout.
		RequireEmailVerification: row.RequireEmailVerification,
		RequireNDA:               row.RequireNda,
	}
	if row.UpdatedAt.Valid {
		out.UpdatedAt = row.UpdatedAt.Time.UTC().Format(timeRFC3339)
	}
	return out
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

// GetRoomAccessPolicy returns the room security policy, or an unconfigured default.
func (s *Service) GetRoomAccessPolicy(ctx context.Context, workspaceID, dealRoomID string) (RoomAccessPolicy, error) {
	room, err := s.getDealRoomForWorkspace(ctx, workspaceID, dealRoomID)
	if err != nil {
		return RoomAccessPolicy{}, err
	}
	row, err := s.queries.GetDealRoomAccessPolicy(ctx, db.GetDealRoomAccessPolicyParams{
		DealRoomID:  room.ID,
		WorkspaceID: room.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultRoomAccessPolicy(dealRoomID), nil
		}
		return RoomAccessPolicy{}, fmt.Errorf("get room access policy: %w", err)
	}
	return dbRoomAccessPolicyToDomain(row), nil
}

func (s *Service) getDealRoomForWorkspace(ctx context.Context, workspaceID, dealRoomID string) (db.DealRoom, error) {
	roomUUID, err := uuid.Parse(dealRoomID)
	if err != nil {
		return db.DealRoom{}, ErrDealRoomNotFound
	}
	workspaceUUID := pgUUID(workspaceID)
	room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          pgtype.UUID{Bytes: roomUUID, Valid: true},
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.DealRoom{}, ErrDealRoomNotFound
		}
		return db.DealRoom{}, fmt.Errorf("get deal room: %w", err)
	}
	return room, nil
}

// UpsertRoomAccessPolicy saves the thin room security policy. Room blocklist is
// enforced at runtime during access evaluation — it is not pushed to links.
func (s *Service) UpsertRoomAccessPolicy(
	ctx context.Context,
	userID, workspaceID, dealRoomID string,
	req UpsertRoomAccessPolicyRequest,
) (RoomAccessPolicy, error) {
	room, err := s.getDealRoomForWorkspace(ctx, workspaceID, dealRoomID)
	if err != nil {
		return RoomAccessPolicy{}, err
	}

	blocked, err := normalizePolicyEmailList(req.BlockedEmails)
	if err != nil {
		return RoomAccessPolicy{}, err
	}

	previousBlocked := []string{}
	if existing, existingErr := s.queries.GetDealRoomAccessPolicy(ctx, db.GetDealRoomAccessPolicyParams{
		DealRoomID:  room.ID,
		WorkspaceID: room.WorkspaceID,
	}); existingErr == nil {
		previousBlocked = existing.BlockedEmails
	} else if !errors.Is(existingErr, pgx.ErrNoRows) {
		return RoomAccessPolicy{}, fmt.Errorf("get room access policy: %w", existingErr)
	}

	row, err := s.queries.UpsertDealRoomAccessPolicy(ctx, db.UpsertDealRoomAccessPolicyParams{
		DealRoomID:                  room.ID,
		TenantID:                    room.TenantID,
		WorkspaceID:                 room.WorkspaceID,
		RequireEmail:                false,
		RequireEmailVerification:    req.RequireEmailVerificationFloor,
		RequirePassword:             false,
		PasswordHash:                pgtype.Text{},
		RequireNda:                  req.RequireNdaFloor,
		NdaTemplateID:               pgtype.UUID{},
		NdaDocumentID:               pgtype.UUID{},
		WatermarkEnabled:            false,
		DownloadEnabled:             false,
		ScreenshotProtectionEnabled: false,
		FileRequestsEnabled:         false,
		IndexFileEnabled:            false,
		QaEnabled:                   false,
		AllowedEmails:               []string{},
		BlockedEmails:               blocked,
		Configured:                  true,
		UpdatedBy:                   pgUUID(userID),
	})
	if err != nil {
		return RoomAccessPolicy{}, fmt.Errorf("upsert room access policy: %w", err)
	}

	s.invalidateRoomAccessPolicyCache(ctx, dealRoomID)

	if removed := blockedEmailsRemoved(previousBlocked, blocked); len(removed) > 0 {
		if err := s.queries.DeleteDealRoomLinkBlocksForEmails(ctx, db.DeleteDealRoomLinkBlocksForEmailsParams{
			DealRoomID:  room.ID,
			WorkspaceID: room.WorkspaceID,
			Emails:      removed,
		}); err != nil {
			return RoomAccessPolicy{}, fmt.Errorf("purge removed room blocks from links: %w", err)
		}
	}

	return dbRoomAccessPolicyToDomain(row), nil
}

// validateNoRoomBlockedAllows rejects allow rules for emails on the room blocklist.
func validateNoRoomBlockedAllows(rules []AccessRule, roomBlockedEmails []string) error {
	roomBlocked, err := normalizePolicyEmailList(roomBlockedEmails)
	if err != nil || len(roomBlocked) == 0 {
		return nil
	}
	roomSet := make(map[string]struct{}, len(roomBlocked))
	for _, email := range roomBlocked {
		roomSet[email] = struct{}{}
	}
	for _, r := range rules {
		if r.Action != "allow" || r.RuleType != "email" {
			continue
		}
		v := strings.TrimSpace(strings.ToLower(r.Value))
		if _, hit := roomSet[v]; hit {
			return fmt.Errorf("%w: %s is blocked by deal room access policy", ErrInvalidAccessRule, v)
		}
	}
	return nil
}

// stripRoomBlocksFromLinkRules removes redundant room block rules from link-scoped
// saves. Room blocks are enforced at runtime and must not be duplicated in link rules.
func stripRoomBlocksFromLinkRules(rules []AccessRule, roomBlockedEmails []string) []AccessRule {
	roomBlocked, err := normalizePolicyEmailList(roomBlockedEmails)
	if err != nil || len(roomBlocked) == 0 {
		return rules
	}
	roomSet := make(map[string]struct{}, len(roomBlocked))
	for _, email := range roomBlocked {
		roomSet[email] = struct{}{}
	}
	out := make([]AccessRule, 0, len(rules))
	for _, r := range rules {
		if r.Action == "block" && r.RuleType == "email" {
			v := strings.TrimSpace(strings.ToLower(r.Value))
			if _, hit := roomSet[v]; hit {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// evaluateAccessWithRoomBlocks applies the room blocklist before link rules.
func evaluateAccessWithRoomBlocks(linkRules []AccessRule, roomBlocked []string, email string) AccessEvaluation {
	normalized := strings.TrimSpace(strings.ToLower(email))
	if normalized != "" {
		for _, blocked := range roomBlocked {
			v := strings.TrimSpace(strings.ToLower(blocked))
			if v == "" {
				continue
			}
			if constantTimeEmailCompare(v, normalized) {
				return AccessEvaluation{
					Allowed:     false,
					Reason:      "blocked_email",
					MatchedRule: &AccessRule{RuleType: "email", Value: v, Action: "block"},
				}
			}
		}
	}
	return evaluateAccessRules(linkRules, email)
}

func (s *Service) loadRoomAccessPolicyRow(
	ctx context.Context,
	workspaceID, dealRoomID string,
) (db.DealRoomAccessPolicy, bool, error) {
	room, err := s.getDealRoomForWorkspace(ctx, workspaceID, dealRoomID)
	if err != nil {
		return db.DealRoomAccessPolicy{}, false, err
	}
	row, err := s.queries.GetDealRoomAccessPolicy(ctx, db.GetDealRoomAccessPolicyParams{
		DealRoomID:  room.ID,
		WorkspaceID: room.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.DealRoomAccessPolicy{}, false, nil
		}
		return db.DealRoomAccessPolicy{}, false, err
	}
	return row, true, nil
}

// applyRoomSecurityToDealRoomLinkRequest enforces outbound floors on create/edit.
// Room blocklist is runtime-only and is never copied into link-scoped rules.
func applyRoomSecurityToDealRoomLinkRequest(req DealRoomLinkRequest, policy db.DealRoomAccessPolicy) (DealRoomLinkRequest, error) {
	if !policy.Configured {
		return req, nil
	}
	if policy.RequireEmailVerification {
		req.RequireEmailVerification = true
		req.RequireEmail = false
	}
	if policy.RequireNda {
		req.RequireNDA = true
	}
	return req, nil
}

func enforceRoomSecurityFloors(policy db.DealRoomAccessPolicy, requireEmailVerification, requireNDA bool) error {
	if !policy.Configured {
		return nil
	}
	if policy.RequireEmailVerification && !requireEmailVerification {
		return fmt.Errorf("%w: email verification is required by room security", ErrRoomSecurityFloor)
	}
	if policy.RequireNda && !requireNDA {
		return fmt.Errorf("%w: NDA is required by room security", ErrRoomSecurityFloor)
	}
	return nil
}

func (s *Service) bootstrapRoomAccessPolicyFromLinkRequest(
	ctx context.Context,
	userID, workspaceID, dealRoomID string,
	req DealRoomLinkRequest,
	_ pgtype.Text,
) error {
	// Creating a link does not invent room floors — only persists the blocklist
	// union so room security stays available for later edits.
	_, err := s.UpsertRoomAccessPolicy(ctx, userID, workspaceID, dealRoomID, UpsertRoomAccessPolicyRequest{
		RequireEmailVerificationFloor: false,
		RequireNdaFloor:               false,
		BlockedEmails:                 req.BlockedEmails,
	})
	return err
}
