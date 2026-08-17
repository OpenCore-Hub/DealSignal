package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	InitJWT("test-secret-for-unit-tests")
}

func TestRegisterValidation(t *testing.T) {
	svc := NewService(nil, NewMemoryTokenStore())
	ctx := context.Background()

	cases := []struct {
		name     string
		email    string
		password string
		err      error
	}{
		{"invalid email", "not-an-email", "password123", ErrInvalidEmail},
		{"disposable email", "user@mailinator.com", "Password123!", ErrDisposableEmail},
		{"disposable plus-tag", "user+trial@yopmail.com", "Password123!", ErrDisposableEmail},
		{"short password", "user@example.com", "short", ErrWeakPassword},
		{"weak password no special", "user@example.com", "Password123", ErrWeakPassword},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.Register(ctx, c.email, c.password)
			if err == nil || err.Error() != c.err.Error() {
				t.Fatalf("expected error %q, got %v", c.err, err)
			}
		})
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Error("nil should not be unique violation")
	}
	if !isUniqueViolation(errors.New("pq: duplicate key value violates unique constraint \"users_email_key\" (SQLSTATE 23505)")) {
		t.Error("expected unique violation")
	}
}

type memoryAccounts struct {
	mu    sync.Mutex
	byID  map[string]db.User
	email map[string]string
}

func newMemoryAccounts() *memoryAccounts {
	return &memoryAccounts{byID: map[string]db.User{}, email: map[string]string{}}
}

func (m *memoryAccounts) seed(u db.User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.UUID(u.ID.Bytes).String()
	m.byID[id] = u
	m.email[u.Email] = id
}

func (m *memoryAccounts) CreateUser(_ context.Context, arg db.CreateUserParams) (db.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.email[arg.Email]; ok {
		return db.User{}, errors.New("duplicate key value violates unique constraint (SQLSTATE 23505)")
	}
	id := uuid.New()
	u := db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         arg.Email,
		PasswordHash:  arg.PasswordHash,
		EmailVerified: arg.EmailVerified,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	m.byID[id.String()] = u
	m.email[arg.Email] = id.String()
	return u, nil
}

func (m *memoryAccounts) GetUserByEmail(_ context.Context, email string) (db.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.email[email]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	return m.byID[id], nil
}

func (m *memoryAccounts) GetUserByID(_ context.Context, id pgtype.UUID) (db.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[uuid.UUID(id.Bytes).String()]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *memoryAccounts) VerifyUserEmail(_ context.Context, id pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := uuid.UUID(id.Bytes).String()
	u, ok := m.byID[key]
	if !ok {
		return pgx.ErrNoRows
	}
	u.EmailVerified = true
	m.byID[key] = u
	return nil
}

func (m *memoryAccounts) DeleteUnverifiedUser(_ context.Context, id pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := uuid.UUID(id.Bytes).String()
	u, ok := m.byID[key]
	if !ok || u.EmailVerified {
		return nil
	}
	delete(m.byID, key)
	delete(m.email, u.Email)
	return nil
}

func (m *memoryAccounts) UpdateUserPassword(_ context.Context, arg db.UpdateUserPasswordParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := uuid.UUID(arg.ID.Bytes).String()
	u, ok := m.byID[key]
	if !ok {
		return pgx.ErrNoRows
	}
	u.PasswordHash = arg.PasswordHash
	m.byID[key] = u
	return nil
}

type recordingClaimer struct {
	n      int
	userID string
	email  string
}

func (r *recordingClaimer) ClaimRoomInvites(_ context.Context, userID, email string) {
	r.n++
	r.userID = userID
	r.email = email
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

func testAuthService(autoVerify bool) (*Service, *memoryAccounts, *recordingMailer, *recordingClaimer) {
	accounts := newMemoryAccounts()
	mail := &recordingMailer{}
	claimer := &recordingClaimer{}
	svc := NewService(nil, NewMemoryTokenStore(), WithMailer(mail), WithAutoVerifyEmail(autoVerify))
	svc.accounts = accounts
	svc.SetRoomInviteClaimer(claimer)
	return svc, accounts, mail, claimer
}

func TestRegisterDoesNotIssueSessionWhenUnverified(t *testing.T) {
	svc, _, _, claimer := testAuthService(false)
	result, err := svc.Register(context.Background(), "user@example.com", "Password123!")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !result.VerificationRequired {
		t.Fatal("expected verification_required")
	}
	if result.Pair.AccessToken != "" || result.Pair.RefreshToken != "" {
		t.Fatalf("unverified register must not issue tokens: %+v", result.Pair)
	}
	if result.User.EmailVerified {
		t.Fatal("user must stay unverified")
	}
	if claimer.n != 0 {
		t.Fatal("unverified register must not claim room invites")
	}
}

func TestRegisterAutoVerifyIssuesSessionAndClaims(t *testing.T) {
	svc, _, _, claimer := testAuthService(true)
	result, err := svc.Register(context.Background(), "user@example.com", "Password123!")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if result.VerificationRequired || result.Pair.AccessToken == "" {
		t.Fatalf("auto-verify must issue a session: %+v", result)
	}
	if !result.User.EmailVerified {
		t.Fatal("expected verified user")
	}
	if claimer.n != 1 {
		t.Fatalf("auto-verify register must claim invites, got %d", claimer.n)
	}
}

func TestRegisterExistingUnverifiedResendsWithoutConflict(t *testing.T) {
	svc, accounts, mail, claimer := testAuthService(false)
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "OldPass123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
	})
	result, err := svc.Register(context.Background(), "user@example.com", "Password123!")
	if err != nil {
		t.Fatalf("expected silent resend, got %v", err)
	}
	if !result.VerificationRequired || result.User.ID != id.String() {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Pair.AccessToken != "" {
		t.Fatal("must not issue session on resend path")
	}
	if claimer.n != 0 {
		t.Fatal("resend path must not claim invites")
	}
	deadline := time.Now().Add(time.Second)
	for mail.lastTo == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if mail.lastTo != "user@example.com" {
		t.Fatalf("expected resend to user@example.com, got %q", mail.lastTo)
	}
}

func TestRegisterVerifiedConflict(t *testing.T) {
	svc, accounts, _, _ := testAuthService(false)
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: true,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
	})
	_, err := svc.Register(context.Background(), "user@example.com", "Password123!")
	if !errors.Is(err, ErrEmailExists) {
		t.Fatalf("expected ErrEmailExists, got %v", err)
	}
}

func TestRegisterReclaimsStaleUnverified(t *testing.T) {
	svc, accounts, _, _ := testAuthService(false)
	oldID := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: oldID, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "OldPass123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC().Add(-49 * time.Hour), Valid: true},
	})
	result, err := svc.Register(context.Background(), "user@example.com", "Password123!")
	if err != nil {
		t.Fatalf("reclaim register: %v", err)
	}
	if result.User.ID == oldID.String() {
		t.Fatal("stale unverified row should have been replaced")
	}
	if !result.VerificationRequired {
		t.Fatal("reclaimed account still needs verification")
	}
	if _, err := accounts.GetUserByID(context.Background(), pgtype.UUID{Bytes: oldID, Valid: true}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("old row should be gone, got %v", err)
	}
}

func TestLoginRejectsUnverified(t *testing.T) {
	svc, accounts, _, claimer := testAuthService(false)
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	_, _, err := svc.Login(context.Background(), "user@example.com", "Password123!", "")
	if !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
	if claimer.n != 0 {
		t.Fatal("unverified login must not claim invites")
	}
}

func TestLoginWrongPasswordDoesNotRevealUnverified(t *testing.T) {
	svc, accounts, _, _ := testAuthService(false)
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	_, _, err := svc.Login(context.Background(), "user@example.com", "WrongPass1!", "invite-token")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestLoginVerifiedClaimsInvites(t *testing.T) {
	svc, accounts, _, claimer := testAuthService(false)
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: true,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	user, pair, err := svc.Login(context.Background(), "user@example.com", "Password123!", "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if pair.AccessToken == "" || !user.EmailVerified {
		t.Fatalf("expected session, got user=%+v pair=%+v", user, pair)
	}
	if claimer.n != 1 {
		t.Fatalf("verified login must claim invites, got %d", claimer.n)
	}
}

type stubInviteProof struct{ ok bool }

func (s stubInviteProof) ValidInviteMailbox(context.Context, string, string) bool {
	return s.ok
}

func TestLoginUnverifiedWithValidInviteCompletes(t *testing.T) {
	svc, accounts, _, claimer := testAuthService(false)
	svc.SetInviteMailboxProver(stubInviteProof{ok: true})
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	user, pair, err := svc.Login(context.Background(), "user@example.com", "Password123!", "invite-token")
	if err != nil {
		t.Fatalf("invite ticket should complete login: %v", err)
	}
	if !user.EmailVerified || pair.AccessToken == "" {
		t.Fatalf("expected verified session, got user=%+v", user)
	}
	stored, err := accounts.GetUserByID(context.Background(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil || !stored.EmailVerified {
		t.Fatal("mailbox proof must persist email_verified")
	}
	if claimer.n != 1 {
		t.Fatalf("verified invite login must claim invites, got %d", claimer.n)
	}
}

func TestLoginUnverifiedWithInvalidInviteStillRejected(t *testing.T) {
	svc, accounts, _, claimer := testAuthService(false)
	svc.SetInviteMailboxProver(stubInviteProof{ok: false})
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	_, _, err := svc.Login(context.Background(), "user@example.com", "Password123!", "invite-token")
	if !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
	if claimer.n != 0 {
		t.Fatal("rejected invite proof must not claim invites")
	}
}

func TestRefreshRejectsUnverified(t *testing.T) {
	svc, accounts, _, _ := testAuthService(false)
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	pair, err := GenerateTokenPair(id.String(), accessTokenDuration, refreshTokenDuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.tokenStore.StoreRefreshToken(context.Background(), id.String(), pair.RefreshToken, refreshTokenDuration); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Refresh(context.Background(), pair.RefreshToken)
	if !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
}

func TestVerifyEmailByTokenIssuesSessionAndClaims(t *testing.T) {
	svc, accounts, _, claimer := testAuthService(false)
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	token, err := svc.verifyStore.CreateVerificationToken(context.Background(), id.String(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	user, pair, err := svc.VerifyEmailByToken(context.Background(), token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !user.EmailVerified || pair.AccessToken == "" {
		t.Fatalf("expected verified session, got user=%+v", user)
	}
	if claimer.n != 1 {
		t.Fatalf("verify must claim invites, got %d", claimer.n)
	}
	if _, _, err := svc.VerifyEmailByToken(context.Background(), token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("spent token must be invalid, got %v", err)
	}
}

func TestResendVerificationOnlyForUnverified(t *testing.T) {
	svc, accounts, mail, _ := testAuthService(false)
	svc.ResendVerification(context.Background(), "missing@example.com")
	id := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: id, Valid: true},
		Email:         "user@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: true,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	svc.ResendVerification(context.Background(), "user@example.com")
	time.Sleep(30 * time.Millisecond)
	if mail.lastTo != "" {
		t.Fatalf("must not email missing or verified mailboxes, got %q", mail.lastTo)
	}

	unverified := uuid.New()
	accounts.seed(db.User{
		ID:            pgtype.UUID{Bytes: unverified, Valid: true},
		Email:         "pending@example.com",
		PasswordHash:  mustHash(t, "Password123!"),
		EmailVerified: false,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	svc.ResendVerification(context.Background(), "pending@example.com")
	deadline := time.Now().Add(time.Second)
	for mail.lastTo == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if mail.lastTo != "pending@example.com" {
		t.Fatalf("expected resend to pending@example.com, got %q", mail.lastTo)
	}
}
