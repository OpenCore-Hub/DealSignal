package domain

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidDomain   = errors.New("invalid domain format")
	ErrDomainExists    = errors.New("domain already registered")
	ErrDomainNotFound  = errors.New("domain not found")
	ErrNotVerified     = errors.New("domain is not verified")
	ErrForbidden       = errors.New("not allowed to manage this tenant")
	ErrInvalidSubdomain = errors.New("subdomain must be lowercase alphanumeric with hyphens")

	subdomainRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	domainRegex    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)
)

const (
	TypeSubdomain  = "SUBDOMAIN"
	TypeCustom     = "CUSTOM"
	TypePublicLink = "PUBLIC_LINK"

	SSLPending = "pending"
	SSLIssued  = "issued"
	SSLExpired = "expired"
	SSLError   = "error"
)

// Domain is the public view of a tenant domain record.
type Domain struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Domain       string    `json:"domain"`
	DomainType   string    `json:"domain_type"`
	IsPrimary    bool      `json:"is_primary"`
	SSLStatus    string    `json:"ssl_status"`
	SSLExpiresAt *string   `json:"ssl_expires_at,omitempty"`
	VerifiedAt   *string   `json:"verified_at,omitempty"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

// Service manages tenant domains and SSL lifecycle.
type Service struct {
	queries    *db.Queries
	provider   CertificateProvider
	cnameTarget string
}

// NewService creates a domain service with the given certificate provider.
func NewService(q *db.Queries, provider CertificateProvider, cnameTarget string) *Service {
	if provider == nil {
		provider = NoopProvider{}
	}
	return &Service{queries: q, provider: provider, cnameTarget: cnameTarget}
}

// Register allocates a new domain for a tenant.
func (s *Service) Register(ctx context.Context, tenantID, domain, domainType string, isPrimary bool) (Domain, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if err := validateDomain(domain, domainType); err != nil {
		return Domain{}, err
	}

	if _, err := s.queries.GetTenantDomainByDomain(ctx, domain); err == nil {
		return Domain{}, ErrDomainExists
	}

	tenantUUID, err := pgUUID(tenantID)
	if err != nil {
		return Domain{}, err
	}

	row, err := s.queries.CreateTenantDomain(ctx, db.CreateTenantDomainParams{
		TenantID:   tenantUUID,
		Domain:     domain,
		DomainType: domainType,
		IsPrimary:  isPrimary,
	})
	if err != nil {
		return Domain{}, err
	}

	return domainFromRow(row), nil
}

// Verify checks domain ownership and issues/renews a certificate.
func (s *Service) Verify(ctx context.Context, tenantID, domainID string) (Domain, error) {
	tenantUUID, err := pgUUID(tenantID)
	if err != nil {
		return Domain{}, err
	}

	domains, err := s.queries.ListTenantDomainsByTenant(ctx, tenantUUID)
	if err != nil {
		return Domain{}, err
	}

	var found db.TenantDomain
	for _, d := range domains {
		if uuidToString(d.ID) == domainID {
			found = d
			break
		}
	}
	if found.Domain == "" {
		return Domain{}, ErrDomainNotFound
	}

	if found.DomainType == TypeCustom {
		if err := s.verifyCNAME(ctx, found.Domain); err != nil {
			return Domain{}, fmt.Errorf("%w: %v", ErrNotVerified, err)
		}
	}

	expiresAt, err := s.provider.Issue(ctx, found.Domain)
	if err != nil {
		_ = s.queries.UpdateTenantDomainSSL(ctx, db.UpdateTenantDomainSSLParams{
			SslStatus:    SSLError,
			SslExpiresAt: pgtype.Timestamptz{},
			VerifiedAt:   pgtype.Timestamptz{},
			ID:           found.ID,
			TenantID:     tenantUUID,
		})
		return Domain{}, err
	}

	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	exp := pgtype.Timestamptz{Time: expiresAt, Valid: true}
	if err := s.queries.UpdateTenantDomainSSL(ctx, db.UpdateTenantDomainSSLParams{
		SslStatus: SSLIssued,
		SslExpiresAt: exp,
		VerifiedAt: now,
		ID:       found.ID,
		TenantID: tenantUUID,
	}); err != nil {
		return Domain{}, err
	}

	found.SslStatus = SSLIssued
	found.SslExpiresAt = exp
	found.VerifiedAt = now
	return domainFromRow(found), nil
}

// List returns all domains for a tenant.
func (s *Service) List(ctx context.Context, tenantID string) ([]Domain, error) {
	tenantUUID, err := pgUUID(tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListTenantDomainsByTenant(ctx, tenantUUID)
	if err != nil {
		return nil, err
	}
	out := make([]Domain, len(rows))
	for i, r := range rows {
		out[i] = domainFromRow(r)
	}
	return out, nil
}

// Delete removes a domain.
func (s *Service) Delete(ctx context.Context, tenantID, domainID string) error {
	tenantUUID, err := pgUUID(tenantID)
	if err != nil {
		return err
	}
	id, err := pgUUID(domainID)
	if err != nil {
		return err
	}
	return s.queries.DeleteTenantDomain(ctx, db.DeleteTenantDomainParams{ID: id, TenantID: tenantUUID})
}

// LookupByHost finds a verified custom or public-link domain.
func (s *Service) LookupByHost(ctx context.Context, host string) (Domain, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	row, err := s.queries.GetTenantDomainByDomain(ctx, host)
	if err != nil {
		return Domain{}, ErrDomainNotFound
	}
	return domainFromRow(row), nil
}

// GetTenantBySlug resolves a tenant by its subdomain slug.
func (s *Service) GetTenantBySlug(ctx context.Context, slug string) (db.GetTenantBySlugRow, error) {
	return s.queries.GetTenantBySlug(ctx, pgtype.Text{String: slug, Valid: true})
}

// ResolveHost returns the tenant ID for a custom or public-link host.
func (s *Service) ResolveHost(ctx context.Context, host string) (string, error) {
	d, err := s.LookupByHost(ctx, host)
	if err != nil {
		return "", err
	}
	if d.VerifiedAt == nil || d.SSLStatus != SSLIssued {
		return "", ErrNotVerified
	}
	return d.TenantID, nil
}

func (s *Service) verifyCNAME(ctx context.Context, domain string) error {
	if s.cnameTarget == "" {
		return nil
	}
	cname, err := net.DefaultResolver.LookupCNAME(ctx, domain)
	if err != nil {
		return err
	}
	cname = strings.ToLower(strings.TrimSuffix(cname, "."))
	if cname != strings.ToLower(strings.TrimSuffix(s.cnameTarget, ".")) {
		return fmt.Errorf("cname points to %s, expected %s", cname, s.cnameTarget)
	}
	return nil
}

func validateDomain(domain, domainType string) error {
	switch domainType {
	case TypeSubdomain:
		if !subdomainRegex.MatchString(domain) {
			return ErrInvalidSubdomain
		}
	case TypeCustom, TypePublicLink:
		if !domainRegex.MatchString(domain) {
			return ErrInvalidDomain
		}
	default:
		return errors.New("invalid domain type")
	}
	return nil
}

func domainFromRow(r db.TenantDomain) Domain {
	d := Domain{
		ID:         uuidToString(r.ID),
		TenantID:   uuidToString(r.TenantID),
		Domain:     r.Domain,
		DomainType: r.DomainType,
		IsPrimary:  r.IsPrimary,
		SSLStatus:  r.SslStatus,
		CreatedAt:  r.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt:  r.UpdatedAt.Time.Format(time.RFC3339),
	}
	if r.SslExpiresAt.Valid {
		s := r.SslExpiresAt.Time.Format(time.RFC3339)
		d.SSLExpiresAt = &s
	}
	if r.VerifiedAt.Valid {
		s := r.VerifiedAt.Time.Format(time.RFC3339)
		d.VerifiedAt = &s
	}
	return d
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

func pgUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}
