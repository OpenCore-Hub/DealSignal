package workspace

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalidViewerDomain       = errors.New("invalid domain")
	ErrViewerDomainTaken         = errors.New("domain already registered")
	ErrViewerDomainNotFound      = errors.New("viewer domain not found")
	ErrViewerDomainNotVerified   = errors.New("domain is not verified")
	ErrViewerDomainCNAMEMissing  = errors.New("no cname record")
	ErrViewerDomainNotConfigured = errors.New("viewer domain is not configured")

	viewerHostnameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)
)

const (
	viewerDomainPending  = "pending"
	viewerDomainVerified = "verified"
)

// CNAMELookup returns the first-hop CNAME target for a hostname (does not follow the chain).
type CNAMELookup func(ctx context.Context, hostname string) (string, error)

// ViewerDomain is the public view of a workspace custom viewer hostname.
type ViewerDomain struct {
	Hostname    string `json:"hostname"`
	Status      string `json:"status"`
	CnameHost   string `json:"cname_host"`
	CnameTarget string `json:"cname_target"`
	VerifiedAt  string `json:"verified_at,omitempty"`
}

// WithViewerDomain configures CNAME verification for workspace viewer domains.
func WithViewerDomain(cnameTarget string) ServiceOption {
	return func(s *Service) {
		s.cnameTarget = strings.TrimSpace(cnameTarget)
	}
}

func normalizeViewerHostname(raw string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(raw))
	host = strings.TrimSuffix(host, ".")
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	if i := strings.IndexAny(host, "/:?#"); i >= 0 {
		host = host[:i]
	}
	host = strings.TrimSpace(host)
	if host == "" || len(host) > 253 || !viewerHostnameRegex.MatchString(host) {
		return "", ErrInvalidViewerDomain
	}
	if host == "localhost" || net.ParseIP(host) != nil {
		return "", ErrInvalidViewerDomain
	}
	return host, nil
}

func cnameMatches(got, want string) bool {
	normalize := func(v string) string {
		return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(v), "."))
	}
	return normalize(got) != "" && normalize(got) == normalize(want)
}

func (s *Service) requireCNAMETarget() error {
	if strings.TrimSpace(s.cnameTarget) != "" {
		return nil
	}
	return ErrViewerDomainNotConfigured
}

func (s *Service) lookupCNAME(ctx context.Context, hostname string) (string, error) {
	lookup := s.cnameLookup
	if lookup == nil {
		lookup = lookupFirstHopCNAME
	}
	return lookup(ctx, hostname)
}

func viewerDomainFromRow(row db.WorkspaceViewerDomain, fallbackTarget string) ViewerDomain {
	target := strings.TrimSpace(row.CnameTarget)
	if target == "" {
		target = strings.TrimSpace(fallbackTarget)
	}
	out := ViewerDomain{
		Hostname:    row.Hostname,
		Status:      row.Status,
		CnameHost:   row.Hostname,
		CnameTarget: target,
	}
	if row.VerifiedAt.Valid {
		out.VerifiedAt = row.VerifiedAt.Time.Format(time.RFC3339)
	}
	return out
}

func (s *Service) emptyViewerDomain() ViewerDomain {
	return ViewerDomain{CnameTarget: strings.TrimSpace(s.cnameTarget)}
}

func (s *Service) verifiedViewerHostname(ctx context.Context, workspaceID string) string {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return ""
	}
	row, err := s.queries.GetWorkspaceViewerDomain(ctx, wsUUID)
	if err != nil || !strings.EqualFold(row.Status, viewerDomainVerified) {
		return ""
	}
	return strings.TrimSpace(row.Hostname)
}

// GetViewerDomain returns the workspace viewer-domain config, including pending CNAME state.
func (s *Service) GetViewerDomain(ctx context.Context, workspaceID string) (ViewerDomain, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return ViewerDomain{}, err
	}
	row, err := s.queries.GetWorkspaceViewerDomain(ctx, wsUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.emptyViewerDomain(), nil
		}
		return ViewerDomain{}, err
	}
	return viewerDomainFromRow(row, s.cnameTarget), nil
}

// PutViewerDomain registers or replaces a pending workspace viewer hostname.
func (s *Service) PutViewerDomain(ctx context.Context, workspaceID, hostname string) (ViewerDomain, error) {
	if err := s.requireCNAMETarget(); err != nil {
		return ViewerDomain{}, err
	}
	hostname, err := normalizeViewerHostname(hostname)
	if err != nil {
		return ViewerDomain{}, err
	}
	if cnameMatches(hostname, s.cnameTarget) {
		return ViewerDomain{}, ErrInvalidViewerDomain
	}

	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return ViewerDomain{}, err
	}

	existing, err := s.queries.GetWorkspaceViewerDomain(ctx, wsUUID)
	if err == nil && strings.EqualFold(existing.Hostname, hostname) {
		return viewerDomainFromRow(existing, s.cnameTarget), nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ViewerDomain{}, err
	}

	// New registration or hostname change — gated by plan (grandfather same-host early return above).
	if err := s.AssertCanUseCustomDomain(ctx, workspaceID); err != nil {
		return ViewerDomain{}, err
	}

	if _, err := s.queries.GetTenantDomainByDomain(ctx, hostname); err == nil {
		return ViewerDomain{}, ErrViewerDomainTaken
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ViewerDomain{}, err
	}

	taken, err := s.queries.GetWorkspaceViewerDomainByHostname(ctx, hostname)
	if err == nil && uuidToString(taken.WorkspaceID) != workspaceID {
		return ViewerDomain{}, ErrViewerDomainTaken
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ViewerDomain{}, err
	}

	row, err := s.queries.UpsertWorkspaceViewerDomain(ctx, db.UpsertWorkspaceViewerDomainParams{
		WorkspaceID: wsUUID,
		Hostname:    hostname,
		CnameTarget: s.cnameTarget,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ViewerDomain{}, ErrViewerDomainTaken
		}
		return ViewerDomain{}, err
	}
	return viewerDomainFromRow(row, s.cnameTarget), nil
}

// VerifyViewerDomain fail-closed CNAME-checks a pending hostname and marks it verified.
func (s *Service) VerifyViewerDomain(ctx context.Context, workspaceID string) (ViewerDomain, error) {
	if err := s.requireCNAMETarget(); err != nil {
		return ViewerDomain{}, err
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return ViewerDomain{}, err
	}
	row, err := s.queries.GetWorkspaceViewerDomain(ctx, wsUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ViewerDomain{}, ErrViewerDomainNotFound
		}
		return ViewerDomain{}, err
	}
	if strings.EqualFold(row.Status, viewerDomainVerified) {
		return viewerDomainFromRow(row, s.cnameTarget), nil
	}

	cname, err := s.lookupCNAME(ctx, row.Hostname)
	if errors.Is(err, errNoCNAME) || errors.Is(err, ErrViewerDomainCNAMEMissing) || (err == nil && strings.TrimSpace(cname) == "") {
		return ViewerDomain{}, ErrViewerDomainCNAMEMissing
	}
	if err != nil || !cnameMatches(cname, s.cnameTarget) {
		return ViewerDomain{}, ErrViewerDomainNotVerified
	}

	verified, err := s.queries.MarkWorkspaceViewerDomainVerified(ctx, wsUUID)
	if err != nil {
		return ViewerDomain{}, err
	}
	return viewerDomainFromRow(verified, s.cnameTarget), nil
}

// DeleteViewerDomain removes the workspace viewer hostname.
func (s *Service) DeleteViewerDomain(ctx context.Context, workspaceID string) error {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	return s.queries.DeleteWorkspaceViewerDomain(ctx, wsUUID)
}
