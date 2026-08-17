package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

func seedVerified(t *testing.T, accounts *memoryAccounts, email, password string) db.User {
	t.Helper()
	id := uuid.New()
	u := db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         email,
		PasswordHash:  mustHash(t, password),
		EmailVerified: true,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	accounts.seed(u)
	return u
}

func waitForMail(t *testing.T, mail *recordingMailer) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for mail.lastTo == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if mail.lastTo == "" {
		t.Fatal("expected a password reset email")
	}
}

func TestForgotPasswordOnlyForVerified(t *testing.T) {
	svc, accounts, mail, _ := testAuthService(false)
	svc.ForgotPassword(context.Background(), "missing@example.com")
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Email:         "pending@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	svc.ForgotPassword(context.Background(), "pending@example.com")
	time.Sleep(40 * time.Millisecond)
	if mail.lastTo != "" {
		t.Fatalf("must not email missing or unverified mailboxes, got %q", mail.lastTo)
	}

	seedVerified(t, accounts, "user@example.com", "Password123!")
	svc.ForgotPassword(context.Background(), "user@example.com")
	waitForMail(t, mail)
	if mail.lastTo != "user@example.com" {
		t.Fatalf("expected reset mail to user@example.com, got %q", mail.lastTo)
	}
	if !strings.Contains(mail.lastLink, "/reset-password/") {
		t.Fatalf("unexpected reset link: %s", mail.lastLink)
	}
}

func TestResetPasswordUpdatesHashAndRevokesRefresh(t *testing.T) {
	svc, accounts, mail, _ := testAuthService(false)
	u := seedVerified(t, accounts, "user@example.com", "OldPass123!")
	userID := uuid.UUID(u.ID.Bytes).String()
	pair, err := svc.issueSession(context.Background(), userID)
	if err != nil {
		t.Fatalf("session: %v", err)
	}

	svc.ForgotPassword(context.Background(), "user@example.com")
	waitForMail(t, mail)
	token := mail.lastLink[strings.LastIndex(mail.lastLink, "/")+1:]

	if err := svc.ResetPassword(context.Background(), token, "NewPass123!"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	got, err := accounts.GetUserByID(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.EmailVerified {
		t.Fatal("reset must not clear email_verified")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("NewPass123!")); err != nil {
		t.Fatal("new password must match")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("OldPass123!")); err == nil {
		t.Fatal("old password must no longer match")
	}
	ok, err := svc.tokenStore.ValidateRefreshToken(context.Background(), userID, pair.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("refresh tokens must be revoked")
	}
	if _, _, err := svc.Login(context.Background(), "user@example.com", "NewPass123!", ""); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if _, _, err := svc.Login(context.Background(), "user@example.com", "OldPass123!", ""); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("old password must fail, got %v", err)
	}
}

func TestResetPasswordConsumesTokenAndRejectsWeak(t *testing.T) {
	svc, accounts, mail, _ := testAuthService(false)
	seedVerified(t, accounts, "user@example.com", "Password123!")
	svc.ForgotPassword(context.Background(), "user@example.com")
	waitForMail(t, mail)
	token := mail.lastLink[strings.LastIndex(mail.lastLink, "/")+1:]

	if err := svc.ResetPassword(context.Background(), token, "short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("expected weak password, got %v", err)
	}
	if err := svc.ResetPassword(context.Background(), token, "NewPass123!"); err != nil {
		t.Fatalf("valid reset after weak attempt: %v", err)
	}
	if err := svc.ResetPassword(context.Background(), token, "Another123!"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("spent token must be invalid, got %v", err)
	}
}

func TestResetPasswordDoesNotVerifyMailbox(t *testing.T) {
	svc, accounts, _, _ := testAuthService(false)
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "pending@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	token, err := svc.resetStore.CreatePasswordResetToken(context.Background(), id.String(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(context.Background(), token, "NewPass123!"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("reset must not verify an unverified mailbox, got %v", err)
	}
	got, err := accounts.GetUserByID(context.Background(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.EmailVerified {
		t.Fatal("email_verified must stay false")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("Password123!")); err != nil {
		t.Fatal("password must be unchanged")
	}
}

func TestPasswordResetTokenLatestOnly(t *testing.T) {
	store := NewMemoryTokenStore()
	first, err := store.CreatePasswordResetToken(context.Background(), "user-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreatePasswordResetToken(context.Background(), "user-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumePasswordResetToken(context.Background(), first); err == nil {
		t.Fatal("previous reset token must be invalidated")
	}
	if id, err := store.ConsumePasswordResetToken(context.Background(), second); err != nil || id != "user-1" {
		t.Fatalf("latest token should work, id=%s err=%v", id, err)
	}
}

func TestPasswordResetTokenStoresHashNotRaw(t *testing.T) {
	store := NewMemoryTokenStore()
	raw, err := store.CreatePasswordResetToken(context.Background(), "user-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.RLock()
	_, rawPresent := store.resetTokens[raw]
	_, hashPresent := store.resetTokens[hashPasswordResetToken(raw)]
	store.mu.RUnlock()
	if rawPresent {
		t.Fatal("raw token must not be stored")
	}
	if !hashPresent {
		t.Fatal("hashed token must be stored")
	}
}

func TestPasswordResetTokenExpires(t *testing.T) {
	store := NewMemoryTokenStore()
	token, err := store.CreatePasswordResetToken(context.Background(), "user-1", -time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumePasswordResetToken(context.Background(), token); err == nil {
		t.Fatal("expired token must fail")
	}
}
