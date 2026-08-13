package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
)

func TestNormalizeViewerHostname(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    string
		wantErr error
	}{
		{"invest.example.com", "invest.example.com", nil},
		{"  Invest.Example.COM. ", "invest.example.com", nil},
		{"https://view.acme.com/path", "view.acme.com", nil},
		{"view.acme.com:443", "view.acme.com", nil},
		{"localhost", "", ErrInvalidViewerDomain},
		{"127.0.0.1", "", ErrInvalidViewerDomain},
		{"not a host", "", ErrInvalidViewerDomain},
		{"single", "", ErrInvalidViewerDomain},
		{"", "", ErrInvalidViewerDomain},
	}
	for _, tc := range cases {
		got, err := normalizeViewerHostname(tc.in)
		if !errors.Is(err, tc.wantErr) {
			t.Fatalf("%q: err=%v want %v", tc.in, err, tc.wantErr)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestCNAMEMatches(t *testing.T) {
	t.Parallel()
	if !cnameMatches("cname.dealsignal.com.", "cname.dealsignal.com") {
		t.Fatal("expected trailing-dot match")
	}
	if cnameMatches("other.example.com", "cname.dealsignal.com") {
		t.Fatal("expected mismatch")
	}
	if cnameMatches("", "cname.dealsignal.com") {
		t.Fatal("empty cname must not match")
	}
}

func TestRequireCNAMETarget(t *testing.T) {
	t.Parallel()
	s := &Service{}
	if err := s.requireCNAMETarget(); !errors.Is(err, ErrViewerDomainNotConfigured) {
		t.Fatalf("empty target: %v", err)
	}
	s.cnameTarget = "cname.dealsignal.com"
	if err := s.requireCNAMETarget(); err != nil {
		t.Fatalf("configured target: %v", err)
	}
}

func TestPutViewerDomainHappyPath(t *testing.T) {
	fake := &fakeDB{t: t, billing: activeTrialBilling()}
	svc := NewService(db.New(fake), WithViewerDomain("cname.dealsignal.com"))
	wsID := uuid.NewString()

	got, err := svc.PutViewerDomain(context.Background(), wsID, "Invest.Example.com")
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if got.Hostname != "invest.example.com" || got.Status != viewerDomainPending {
		t.Fatalf("got %+v", got)
	}
	if got.CnameTarget != "cname.dealsignal.com" || got.CnameHost != "invest.example.com" {
		t.Fatalf("cname fields: %+v", got)
	}

	again, err := svc.PutViewerDomain(context.Background(), wsID, "invest.example.com")
	if err != nil {
		t.Fatalf("idempotent put: %v", err)
	}
	if again.Status != viewerDomainPending {
		t.Fatalf("expected pending stay pending, got %s", again.Status)
	}
}

func TestPutViewerDomainRejectsUnconfigured(t *testing.T) {
	svc := NewService(db.New(&fakeDB{t: t}))
	_, err := svc.PutViewerDomain(context.Background(), uuid.NewString(), "invest.example.com")
	if !errors.Is(err, ErrViewerDomainNotConfigured) {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyViewerDomainCNAMEMissing(t *testing.T) {
	fake := &fakeDB{t: t, billing: activeTrialBilling()}
	svc := NewService(db.New(fake), WithViewerDomain("cname.dealsignal.com"))
	svc.cnameLookup = func(_ context.Context, _ string) (string, error) {
		return "", errNoCNAME
	}
	wsID := uuid.NewString()
	if _, err := svc.PutViewerDomain(context.Background(), wsID, "www.m3u.vip"); err != nil {
		t.Fatalf("put: %v", err)
	}
	_, err := svc.VerifyViewerDomain(context.Background(), wsID)
	if !errors.Is(err, ErrViewerDomainCNAMEMissing) {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyViewerDomainFailClosed(t *testing.T) {
	fake := &fakeDB{t: t, billing: activeTrialBilling()}
	svc := NewService(db.New(fake), WithViewerDomain("cname.dealsignal.com"))
	svc.cnameLookup = func(_ context.Context, _ string) (string, error) {
		return "wrong.example.net.", nil
	}
	wsID := uuid.NewString()
	if _, err := svc.PutViewerDomain(context.Background(), wsID, "invest.example.com"); err != nil {
		t.Fatalf("put: %v", err)
	}
	_, err := svc.VerifyViewerDomain(context.Background(), wsID)
	if !errors.Is(err, ErrViewerDomainNotVerified) {
		t.Fatalf("got %v", err)
	}
	got, err := svc.GetViewerDomain(context.Background(), wsID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != viewerDomainPending {
		t.Fatalf("mismatch must leave pending, got %s", got.Status)
	}
}

func TestVerifyViewerDomainSuccess(t *testing.T) {
	fake := &fakeDB{t: t, billing: activeTrialBilling()}
	svc := NewService(db.New(fake), WithViewerDomain("cname.dealsignal.com"))
	svc.cnameLookup = func(_ context.Context, host string) (string, error) {
		if host != "invest.example.com" {
			t.Fatalf("lookup host %q", host)
		}
		return "cname.dealsignal.com.", nil
	}
	wsID := uuid.NewString()
	if _, err := svc.PutViewerDomain(context.Background(), wsID, "invest.example.com"); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := svc.VerifyViewerDomain(context.Background(), wsID)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Status != viewerDomainVerified || got.VerifiedAt == "" {
		t.Fatalf("got %+v", got)
	}
	if svc.verifiedViewerHostname(context.Background(), wsID) != "invest.example.com" {
		t.Fatalf("settings host should be verified hostname")
	}
}

func TestGetViewerDomainEmpty(t *testing.T) {
	svc := NewService(db.New(&fakeDB{t: t}), WithViewerDomain("cname.dealsignal.com"))
	got, err := svc.GetViewerDomain(context.Background(), uuid.NewString())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Hostname != "" || got.Status != "" || got.CnameTarget != "cname.dealsignal.com" {
		t.Fatalf("got %+v", got)
	}
}

func TestDeleteViewerDomain(t *testing.T) {
	fake := &fakeDB{t: t, billing: activeTrialBilling()}
	svc := NewService(db.New(fake), WithViewerDomain("cname.dealsignal.com"))
	wsID := uuid.NewString()
	if _, err := svc.PutViewerDomain(context.Background(), wsID, "invest.example.com"); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := svc.DeleteViewerDomain(context.Background(), wsID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, err := svc.GetViewerDomain(context.Background(), wsID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Hostname != "" || got.Status != "" {
		t.Fatalf("expected empty after delete, got %+v", got)
	}
}
