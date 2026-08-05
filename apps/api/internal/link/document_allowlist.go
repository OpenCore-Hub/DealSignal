package link

import (
	"context"
	"fmt"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// accessRuleStore is the subset of db.Queries used to sync allow rules.
type accessRuleStore interface {
	ListLinkAccessRulesByLink(ctx context.Context, linkID pgtype.UUID) ([]db.LinkAccessRule, error)
	DeleteLinkAccessRulesByLink(ctx context.Context, linkID pgtype.UUID) error
	CreateLinkAccessRule(ctx context.Context, arg db.CreateLinkAccessRuleParams) error
}

type linkContactEmailStore interface {
	GetLinkContactsByPublicToken(ctx context.Context, publicToken string) ([]db.GetLinkContactsByPublicTokenRow, error)
}

// normalizeEmailList lowercases, trims, drops empties, and de-duplicates.
func normalizeEmailList(emails []string) []string {
	if len(emails) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(emails))
	out := make([]string, 0, len(emails))
	for _, email := range emails {
		v := strings.TrimSpace(strings.ToLower(email))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// documentAllowEmailsFromCodes returns contact emails collected while
// provisioning document-link access codes. For document links these are the
// allowlist source of truth (matching deal-room allowed_emails semantics).
func documentAllowEmailsFromCodes(codes []emailCode) []string {
	if len(codes) == 0 {
		return nil
	}
	emails := make([]string, 0, len(codes))
	for _, c := range codes {
		emails = append(emails, c.email)
	}
	return normalizeEmailList(emails)
}

// replaceAllowEmailRules rewrites email allow rules while preserving block rules.
// Used by document-link Create/Update so contact_ids stay aligned with access_rules.
func replaceAllowEmailRules(
	ctx context.Context,
	q accessRuleStore,
	link db.Link,
	workspaceID pgtype.UUID,
	allowEmails []string,
) error {
	existing, err := q.ListLinkAccessRulesByLink(ctx, link.ID)
	if err != nil {
		return fmt.Errorf("list access rules: %w", err)
	}

	blocks := make([]AccessRule, 0)
	blocked := make(map[string]struct{})
	for _, r := range existing {
		if r.Action != "block" || r.RuleType != "email" {
			continue
		}
		v := strings.TrimSpace(strings.ToLower(r.Value))
		if v == "" {
			continue
		}
		if _, ok := blocked[v]; ok {
			continue
		}
		blocked[v] = struct{}{}
		blocks = append(blocks, AccessRule{RuleType: "email", Value: v, Action: "block"})
	}

	allows := normalizeEmailList(allowEmails)
	for _, email := range allows {
		if _, ok := blocked[email]; ok {
			return fmt.Errorf("%w: %s cannot be both allowed and blocked", ErrConflictingAccessRule, email)
		}
	}

	if err := q.DeleteLinkAccessRulesByLink(ctx, link.ID); err != nil {
		return fmt.Errorf("delete access rules: %w", err)
	}

	sortOrder := int32(0)
	for _, email := range allows {
		if err := q.CreateLinkAccessRule(ctx, db.CreateLinkAccessRuleParams{
			TenantID:    link.TenantID,
			WorkspaceID: workspaceID,
			LinkID:      link.ID,
			RuleType:    "email",
			Value:       email,
			Action:      "allow",
			SortOrder:   sortOrder,
		}); err != nil {
			return fmt.Errorf("create allow rule: %w", err)
		}
		sortOrder++
	}
	for _, block := range blocks {
		if err := q.CreateLinkAccessRule(ctx, db.CreateLinkAccessRuleParams{
			TenantID:    link.TenantID,
			WorkspaceID: workspaceID,
			LinkID:      link.ID,
			RuleType:    block.RuleType,
			Value:       block.Value,
			Action:      block.Action,
			SortOrder:   sortOrder,
		}); err != nil {
			return fmt.Errorf("create block rule: %w", err)
		}
		sortOrder++
	}
	return nil
}

// ensureDocumentLinkAllowlistFromContacts makes sure every document-link
// contact email has an allow rule. Additive only: never removes extra allow
// rules (approved access requests / UpdateAccessRules) or block rules.
// No-op for deal-room links.
func (s *Service) ensureDocumentLinkAllowlistFromContacts(ctx context.Context, link db.Link) error {
	return ensureDocumentLinkAllowlistFromContacts(ctx, s.queries, s.queries, link)
}

func ensureDocumentLinkAllowlistFromContacts(
	ctx context.Context,
	rules accessRuleStore,
	contacts linkContactEmailStore,
	link db.Link,
) error {
	if link.DealRoomID.Valid || !link.RequireEmailVerification {
		return nil
	}

	rows, err := contacts.GetLinkContactsByPublicToken(ctx, link.PublicToken)
	if err != nil {
		return fmt.Errorf("list link contacts: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	emails := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ContactEmail.Valid {
			emails = append(emails, row.ContactEmail.String)
		}
	}
	emails = normalizeEmailList(emails)
	if len(emails) == 0 {
		return nil
	}

	existing, err := rules.ListLinkAccessRulesByLink(ctx, link.ID)
	if err != nil {
		return fmt.Errorf("list access rules: %w", err)
	}
	allowed := make(map[string]struct{}, len(existing))
	maxSort := int32(-1)
	for _, r := range existing {
		if r.SortOrder > maxSort {
			maxSort = r.SortOrder
		}
		if r.Action == "allow" && r.RuleType == "email" {
			allowed[strings.TrimSpace(strings.ToLower(r.Value))] = struct{}{}
		}
	}

	sortOrder := maxSort + 1
	for _, email := range emails {
		if _, ok := allowed[email]; ok {
			continue
		}
		if err := rules.CreateLinkAccessRule(ctx, db.CreateLinkAccessRuleParams{
			TenantID:    link.TenantID,
			WorkspaceID: link.WorkspaceID,
			LinkID:      link.ID,
			RuleType:    "email",
			Value:       email,
			Action:      "allow",
			SortOrder:   sortOrder,
		}); err != nil {
			return fmt.Errorf("create allow rule: %w", err)
		}
		sortOrder++
	}
	return nil
}
