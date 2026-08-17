package turnstile

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDisabledSkipsVerify(t *testing.T) {
	t.Parallel()
	v := New("", "site")
	if v.Enabled() {
		t.Fatal("empty secret must disable Turnstile")
	}
	if err := v.Verify(context.Background(), "", ""); err != nil {
		t.Fatalf("disabled verify: %v", err)
	}
}

func TestVerifyMissingAndTooLong(t *testing.T) {
	t.Parallel()
	v := New("secret", "site", WithVerifyURL("http://127.0.0.1:1/unused"))
	if err := v.Verify(context.Background(), "  ", ""); err != ErrMissing {
		t.Fatalf("empty token: %v", err)
	}
	if err := v.Verify(context.Background(), strings.Repeat("a", maxTokenBytes+1), ""); err != ErrFailed {
		t.Fatalf("oversized token: %v", err)
	}
}

func TestVerifySuccessAndConstraints(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("parse form: %v", err)
		}
		if form.Get("secret") == "" || form.Get("response") == "" {
			t.Errorf("missing form fields: %v", form)
		}
		if form.Get("remoteip") != "203.0.113.9" {
			t.Errorf("remoteip=%q", form.Get("remoteip"))
		}
		_ = json.NewEncoder(w).Encode(siteverifyResponse{
			Success:  true,
			Hostname: "app.example.com",
			Action:   ActionRegister,
		})
	}))
	t.Cleanup(srv.Close)

	v := New("prod-secret", "site",
		WithVerifyURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithExpectedHost("app.example.com"),
		WithAction(ActionRegister),
	)
	if err := v.Verify(context.Background(), "token", "203.0.113.9"); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyRejectsFailedChallenge(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(siteverifyResponse{
			Success:    false,
			ErrorCodes: []string{"invalid-input-response"},
		})
	}))
	t.Cleanup(srv.Close)
	v := New("prod-secret", "site", WithVerifyURL(srv.URL), WithHTTPClient(srv.Client()))
	if err := v.Verify(context.Background(), "bad", ""); err != ErrFailed {
		t.Fatalf("failed token: %v", err)
	}
}

func TestVerifyHostnameMismatch(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(siteverifyResponse{
			Success:  true,
			Hostname: "evil.example",
			Action:   ActionRegister,
		})
	}))
	t.Cleanup(srv.Close)
	v := New("prod-secret", "site",
		WithVerifyURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithExpectedHost("app.example.com"),
	)
	if err := v.Verify(context.Background(), "token", ""); err != ErrFailed {
		t.Fatalf("hostname mismatch: %v", err)
	}
}

func TestDummySecretSkipsHostname(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(siteverifyResponse{
			Success:  true,
			Hostname: "example.com",
			Action:   "login",
		})
	}))
	t.Cleanup(srv.Close)
	v := New("1x0000000000000000000000000000000AA", "1x00000000000000000000AA",
		WithVerifyURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithExpectedHost("app.example.com"),
	)
	if err := v.Verify(context.Background(), "XXXX.DUMMY.TOKEN.XXXX", ""); err != nil {
		t.Fatalf("dummy secret: %v", err)
	}
}

func TestVerifyUnavailableOnHTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	v := New("prod-secret", "site", WithVerifyURL(srv.URL), WithHTTPClient(srv.Client()))
	if err := v.Verify(context.Background(), "token", ""); err != ErrUnavailable {
		t.Fatalf("unavailable: %v", err)
	}
}

func TestVerifyContextTimeout(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(siteverifyResponse{Success: true})
	}))
	t.Cleanup(srv.Close)
	client := srv.Client()
	client.Timeout = 50 * time.Millisecond
	v := New("prod-secret", "site", WithVerifyURL(srv.URL), WithHTTPClient(client))
	if err := v.Verify(context.Background(), "token", ""); err != ErrUnavailable {
		t.Fatalf("timeout: %v", err)
	}
}

func TestHostFromURL(t *testing.T) {
	t.Parallel()
	if got := HostFromURL("https://App.Example.com:443/path"); got != "app.example.com" {
		t.Fatalf("got %q", got)
	}
	if !isLocalHost(HostFromURL("http://localhost:5173")) {
		t.Fatal("localhost should skip hostname pin")
	}
}
