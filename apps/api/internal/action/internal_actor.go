package action

import (
	"context"
	"fmt"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// NormalizeEmail lowercases and trims for membership comparisons.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// MemberEmailSet is a normalized set of workspace member emails.
type MemberEmailSet map[string]struct{}

// NewMemberEmailSet builds a set from raw emails (empty entries ignored).
func NewMemberEmailSet(emails []string) MemberEmailSet {
	out := make(MemberEmailSet, len(emails))
	for _, e := range emails {
		if n := NormalizeEmail(e); n != "" {
			out[n] = struct{}{}
		}
	}
	return out
}

// Contains reports whether email belongs to a workspace member.
func (s MemberEmailSet) Contains(email string) bool {
	if len(s) == 0 {
		return false
	}
	n := NormalizeEmail(email)
	if n == "" {
		return false
	}
	_, ok := s[n]
	return ok
}

// LoadMemberEmailSet loads workspace member emails for internal-actor filtering.
func LoadMemberEmailSet(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID) (MemberEmailSet, error) {
	rows, err := q.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace members: %w", err)
	}
	emails := make([]string, 0, len(rows))
	for _, r := range rows {
		emails = append(emails, r.Email)
	}
	return NewMemberEmailSet(emails), nil
}

// SkipVisitorAttributedActor reports whether a visitor-attributed row must not
// become an operational action / radar card.
//
// Policy (aligned with Leak Watch SQL): only skip when we can positively prove
// the actor is a workspace member. Unknown attribution stays — hosts still need
// to see anonymous / NULL-email Ask and access work.
//
//   - emailPresent=false (SQL NULL): keep (cannot prove internal).
//   - emailPresent=true with empty string: keep (anonymous open-link visitor).
//   - workspace member email: skip.
func SkipVisitorAttributedActor(internal MemberEmailSet, emailPresent bool, email string) bool {
	if !emailPresent {
		return false
	}
	if NormalizeEmail(email) == "" {
		return false
	}
	return internal.Contains(email)
}
