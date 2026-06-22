package domain

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestRegisterInvalidSubdomain(t *testing.T) {
	svc := NewService(db.New(&fakeDomainDB{t: t}), NoopProvider{}, "")
	_, err := svc.Register(context.Background(), uuid.NewString(), "ACME Corp", TypeSubdomain, false)
	if !errors.Is(err, ErrInvalidSubdomain) {
		t.Fatalf("expected ErrInvalidSubdomain, got %v", err)
	}
}

func TestRegisterInvalidCustomDomain(t *testing.T) {
	svc := NewService(db.New(&fakeDomainDB{t: t}), NoopProvider{}, "")
	_, err := svc.Register(context.Background(), uuid.NewString(), "-bad.com", TypeCustom, false)
	if !errors.Is(err, ErrInvalidDomain) {
		t.Fatalf("expected ErrInvalidDomain, got %v", err)
	}
}

func TestRegisterDuplicateDomain(t *testing.T) {
	fake := &fakeDomainDB{t: t}
	svc := NewService(db.New(fake), NoopProvider{}, "")
	ctx := context.Background()
	if _, err := svc.Register(ctx, uuid.NewString(), "acme", TypeSubdomain, false); err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, err := svc.Register(ctx, uuid.NewString(), "acme", TypeSubdomain, false)
	if !errors.Is(err, ErrDomainExists) {
		t.Fatalf("expected ErrDomainExists, got %v", err)
	}
}

func TestRegisterAndList(t *testing.T) {
	fake := &fakeDomainDB{t: t}
	svc := NewService(db.New(fake), NoopProvider{}, "")
	ctx := context.Background()
	tenantID := uuid.NewString()

	d, err := svc.Register(ctx, tenantID, "acme", TypeSubdomain, true)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if d.TenantID != tenantID {
		t.Fatalf("expected tenant id %s, got %s", tenantID, d.TenantID)
	}

	list, err := svc.List(ctx, tenantID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(list))
	}
}

func TestVerifySubdomain(t *testing.T) {
	fake := &fakeDomainDB{t: t}
	svc := NewService(db.New(fake), NoopProvider{}, "")
	ctx := context.Background()
	tenantID := uuid.NewString()

	d, _ := svc.Register(ctx, tenantID, "acme", TypeSubdomain, false)
	verified, err := svc.Verify(ctx, tenantID, d.ID)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verified.SSLStatus != SSLIssued {
		t.Fatalf("expected status %s, got %s", SSLIssued, verified.SSLStatus)
	}
	if verified.VerifiedAt == nil {
		t.Fatal("expected verified_at to be set")
	}
}

func TestVerifyCustomDomainCNAMEMismatch(t *testing.T) {
	fake := &fakeDomainDB{t: t}
	svc := NewService(db.New(fake), NoopProvider{}, "cname.example.com")
	svc.cnameLookup = func(_ context.Context, _ string) (string, error) {
		return "wrong.example.com.", nil
	}
	ctx := context.Background()
	tenantID := uuid.NewString()

	d, _ := svc.Register(ctx, tenantID, "brand.example.com", TypeCustom, false)
	_, err := svc.Verify(ctx, tenantID, d.ID)
	if !errors.Is(err, ErrNotVerified) {
		t.Fatalf("expected ErrNotVerified, got %v", err)
	}
}

func TestResolveHostRequiresVerification(t *testing.T) {
	fake := &fakeDomainDB{t: t}
	svc := NewService(db.New(fake), NoopProvider{}, "")
	ctx := context.Background()
	tenantID := uuid.NewString()

	_, _ = svc.Register(ctx, tenantID, "brand.example.com", TypeCustom, false)
	_, err := svc.ResolveHost(ctx, "brand.example.com")
	if !errors.Is(err, ErrNotVerified) {
		t.Fatalf("expected ErrNotVerified, got %v", err)
	}
}

func TestRenewExpiringCertificates(t *testing.T) {
	fake := &fakeDomainDB{t: t}
	svc := NewService(db.New(fake), NoopProvider{}, "")
	ctx := context.Background()
	tenantID := uuid.NewString()

	d, _ := svc.Register(ctx, tenantID, "acme", TypeSubdomain, false)
	if _, err := svc.Verify(ctx, tenantID, d.ID); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Force the certificate to appear expiring soon.
	updated := fake.domainsByID[d.ID]
	updated.SslExpiresAt = pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true}
	fake.domainsByID[d.ID] = updated
	fake.domains[updated.Domain] = updated

	renewed, err := svc.RenewExpiringCertificates(ctx, time.Now().Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if renewed != 1 {
		t.Fatalf("expected 1 renewed, got %d", renewed)
	}
}

// fakeDomainDB is an in-memory DBTX for domain service tests.
type fakeDomainDB struct {
	t           *testing.T
	domains     map[string]db.TenantDomain
	domainsByID map[string]db.TenantDomain
	tenantSlugs map[string]db.GetTenantBySlugRow
}

func (f *fakeDomainDB) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	sqlLower := strings.ToLower(sql)
	if strings.Contains(sqlLower, "update tenant_domains") {
		id := argUUID(arguments, 3)
		if d, ok := f.domainsByID[uuid.UUID(id.Bytes).String()]; ok {
			status := argString(arguments, 0)
			exp := argTimestamptz(arguments, 1)
			verified := argTimestamptz(arguments, 2)
			d.SslStatus = status
			d.SslExpiresAt = exp
			d.VerifiedAt = verified
			d.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			f.domainsByID[uuid.UUID(id.Bytes).String()] = d
			f.domains[d.Domain] = d
		}
		return pgconn.CommandTag{}, nil
	}
	if strings.Contains(sqlLower, "delete from tenant_domains") {
		id := argUUID(arguments, 0)
		if d, ok := f.domainsByID[uuid.UUID(id.Bytes).String()]; ok {
			delete(f.domains, d.Domain)
			delete(f.domainsByID, uuid.UUID(id.Bytes).String())
		}
		return pgconn.CommandTag{}, nil
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeDomainDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	sqlLower := strings.ToLower(sql)
	var rows [][]interface{}
	if strings.Contains(sqlLower, "from tenant_domains") && strings.Contains(sqlLower, "tenant_id = $1") {
		tenantID := argUUID(args, 0)
		for _, d := range f.domains {
			if uuid.UUID(d.TenantID.Bytes).String() == uuid.UUID(tenantID.Bytes).String() {
				rows = append(rows, domainRowValues(d))
			}
		}
	} else if strings.Contains(sqlLower, "ssl_expires_at < $1") {
		threshold := argTimestamptz(args, 0)
		for _, d := range f.domains {
			if d.SslStatus == SSLIssued && d.SslExpiresAt.Valid && d.SslExpiresAt.Time.Before(threshold.Time) {
				rows = append(rows, domainRowValues(d))
			}
		}
	}
	return &fakeRows{rows: rows}, nil
}

func (f *fakeDomainDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	sqlLower := strings.ToLower(sql)
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	if strings.Contains(sqlLower, "insert into tenant_domains") {
		d := db.TenantDomain{
			ID:         newPGUUID(),
			TenantID:   argUUID(args, 0),
			Domain:     argString(args, 1),
			DomainType: argString(args, 2),
			IsPrimary:  argBool(args, 3),
			SslStatus:  SSLPending,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		f.ensureMaps()
		f.domains[d.Domain] = d
		f.domainsByID[uuid.UUID(d.ID.Bytes).String()] = d
		return fakeRow{values: domainRowValues(d)}
	}

	if strings.Contains(sqlLower, "from tenant_domains") && strings.Contains(sqlLower, "domain = $1") {
		f.ensureMaps()
		domain := argString(args, 0)
		if d, ok := f.domains[domain]; ok {
			return fakeRow{values: domainRowValues(d)}
		}
		return fakeRow{err: pgx.ErrNoRows}
	}

	if strings.Contains(sqlLower, "from tenants") && strings.Contains(sqlLower, "slug = $1") {
		slug := argString(args, 0)
		if t, ok := f.tenantSlugs[slug]; ok {
			return fakeRow{values: []interface{}{t.ID, t.Name, t.CreatedAt}}
		}
		return fakeRow{err: pgx.ErrNoRows}
	}

	return fakeRow{err: errors.New("unexpected query")}
}

func (f *fakeDomainDB) ensureMaps() {
	if f.domains == nil {
		f.domains = make(map[string]db.TenantDomain)
	}
	if f.domainsByID == nil {
		f.domainsByID = make(map[string]db.TenantDomain)
	}
	if f.tenantSlugs == nil {
		f.tenantSlugs = make(map[string]db.GetTenantBySlugRow)
	}

}

func domainRowValues(d db.TenantDomain) []interface{} {
	return []interface{}{
		d.ID, d.TenantID, d.Domain, d.DomainType, d.IsPrimary,
		d.SslStatus, d.SslExpiresAt, d.VerifiedAt, d.CreatedAt, d.UpdatedAt,
	}
}

type fakeRow struct {
	values []interface{}
	err    error
}

func (r fakeRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan count mismatch: got %d, want %d", len(dest), len(r.values))
	}
	for i, v := range r.values {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr {
			return fmt.Errorf("destination is not a pointer")
		}
		sv := reflect.ValueOf(v)
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			return fmt.Errorf("cannot assign %s to %s", sv.Type(), dv.Elem().Type())
		}
		dv.Elem().Set(sv)
	}
	return nil
}

type fakeRows struct {
	rows [][]interface{}
	pos  int
}

func (r *fakeRows) Next() bool { return r.pos < len(r.rows) }
func (r *fakeRows) Err() error { return nil }
func (r *fakeRows) Close()     {}
func (r *fakeRows) CommandTag() pgconn.CommandTag                  { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) Scan(dest ...interface{}) error {
	if r.pos >= len(r.rows) {
		return pgx.ErrNoRows
	}
	row := r.rows[r.pos]
	r.pos++
	if len(dest) != len(row) {
		return fmt.Errorf("scan count mismatch: got %d, want %d", len(dest), len(row))
	}
	for i, v := range row {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr {
			return fmt.Errorf("destination is not a pointer")
		}
		sv := reflect.ValueOf(v)
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			return fmt.Errorf("cannot assign %s to %s", sv.Type(), dv.Elem().Type())
		}
		dv.Elem().Set(sv)
	}
	return nil
}

func newPGUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.New(), Valid: true}
}

func argString(args []interface{}, i int) string {
	if i >= len(args) {
		return ""
	}
	if s, ok := args[i].(string); ok {
		return s
	}
	if t, ok := args[i].(pgtype.Text); ok {
		return t.String
	}
	return ""
}

func argUUID(args []interface{}, i int) pgtype.UUID {
	if i >= len(args) {
		return pgtype.UUID{}
	}
	if u, ok := args[i].(pgtype.UUID); ok {
		return u
	}
	return pgtype.UUID{}
}

func argBool(args []interface{}, i int) bool {
	if i >= len(args) {
		return false
	}
	if b, ok := args[i].(bool); ok {
		return b
	}
	return false
}

func argTimestamptz(args []interface{}, i int) pgtype.Timestamptz {
	if i >= len(args) {
		return pgtype.Timestamptz{}
	}
	if t, ok := args[i].(pgtype.Timestamptz); ok {
		return t
	}
	return pgtype.Timestamptz{}
}
