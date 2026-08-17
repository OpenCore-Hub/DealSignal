package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth/emailid"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost                   = 12
	accessTokenDuration          = 15 * time.Minute
	refreshTokenDuration         = 7 * 24 * time.Hour
	defaultVerificationTokenTTL  = 24 * time.Hour
	defaultPasswordResetTokenTTL = 30 * time.Minute
	// Background send budget (covers provider retries). Does not block Register.
	defaultVerificationSendTimeout = 30 * time.Second
	// Unverified rows older than this may be deleted so the same mailbox can re-register.
	unverifiedReclaimAfter = 48 * time.Hour
)

var (
	ErrEmailExists      = errors.New("email already registered")
	ErrInvalidEmail     = errors.New("invalid email address")
	ErrUnauthorized     = errors.New("invalid email or password")
	ErrTokenInvalid     = errors.New("invalid or expired token")
	ErrTokenRevoked     = errors.New("token has been revoked")
	ErrWeakPassword     = errors.New("password does not meet complexity requirements")
	ErrEmailNotVerified = errors.New("email not verified")
	ErrDisposableEmail  = errors.New("disposable email addresses are not allowed")
	emailRegex          = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// TokenStore abstracts the operations needed for token revocation and refresh.
type TokenStore interface {
	BlocklistToken(ctx context.Context, token string, ttl time.Duration) error
	IsTokenBlocklisted(ctx context.Context, token string) (bool, error)
	StoreRefreshToken(ctx context.Context, userID, refreshToken string, ttl time.Duration) error
	ValidateRefreshToken(ctx context.Context, userID, refreshToken string) (bool, error)
	RevokeRefreshToken(ctx context.Context, userID, refreshToken string) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID string) error
}

// verificationTokenStore creates and resolves single-use email-verification tokens.
type verificationTokenStore interface {
	CreateVerificationToken(ctx context.Context, userID string, ttl time.Duration) (string, error)
	UserIDByVerificationToken(ctx context.Context, token string) (string, error)
	DeleteVerificationToken(ctx context.Context, token string) error
}

// passwordResetTokenStore creates and consumes hashed, single-use reset tokens.
type passwordResetTokenStore interface {
	CreatePasswordResetToken(ctx context.Context, userID string, ttl time.Duration) (string, error)
	ConsumePasswordResetToken(ctx context.Context, token string) (string, error)
}

// User is the public view of a db.User.
type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	CreatedAt     string `json:"created_at"`
}

// RegisterResult is returned after creating (or re-touching) an account.
// Session tokens are only present when the mailbox is already verified
// (non-production auto-verify). Production register never issues a session.
type RegisterResult struct {
	User                 User
	Pair                 TokenPair
	VerificationRequired bool
}

// accountStore is the user-row surface Register/Login/Verify need.
// *db.Queries implements it; tests inject a memory store.
type accountStore interface {
	CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error)
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	GetUserByID(ctx context.Context, id pgtype.UUID) (db.User, error)
	VerifyUserEmail(ctx context.Context, id pgtype.UUID) error
	DeleteUnverifiedUser(ctx context.Context, id pgtype.UUID) error
	UpdateUserPassword(ctx context.Context, arg db.UpdateUserPasswordParams) error
}

// Service handles user authentication.
type Service struct {
	queries               *db.Queries
	accounts              accountStore
	tokenStore            TokenStore
	verifyStore           verificationTokenStore
	resetStore            passwordResetTokenStore
	mailer                mailer.Mailer
	appBaseURL            string
	verificationTokenTTL  time.Duration
	passwordResetTokenTTL time.Duration
	sendTimeout           time.Duration
	autoVerifyEmail       bool
	trialActivator        TrialActivator
	roomInviteClaimer     RoomInviteClaimer
	inviteMailbox         InviteMailboxProver
}

// TrialActivator starts the 14-day trial on the user's first owned Free
// workspace after email verification. Wired from workspace.Service.
type TrialActivator interface {
	ActivateEligibleTrial(ctx context.Context, userID string) error
}

// ServiceOption configures the auth service.
type ServiceOption func(*Service)

// WithMailer sets the transactional mailer used for verification emails.
func WithMailer(m mailer.Mailer) ServiceOption {
	return func(s *Service) { s.mailer = m }
}

// WithAppBaseURL sets the public application base URL used in email links.
func WithAppBaseURL(url string) ServiceOption {
	return func(s *Service) { s.appBaseURL = url }
}

// WithVerificationTokenTTL sets the lifetime of email verification tokens.
func WithVerificationTokenTTL(ttl time.Duration) ServiceOption {
	return func(s *Service) { s.verificationTokenTTL = ttl }
}

// WithSendTimeout caps how long the *background* verification email send may
// run (including provider retries). Registration never waits on this budget.
func WithSendTimeout(timeout time.Duration) ServiceOption {
	return func(s *Service) { s.sendTimeout = timeout }
}

// WithAutoVerifyEmail marks new accounts verified at register. Production
// must leave this off so a mailbox click is required before Trial.
func WithAutoVerifyEmail(enabled bool) ServiceOption {
	return func(s *Service) { s.autoVerifyEmail = enabled }
}

// SetTrialActivator is assigned after workspace.Service is constructed.
func (s *Service) SetTrialActivator(a TrialActivator) {
	if s != nil {
		s.trialActivator = a
	}
}

// RoomInviteClaimer binds pending room invites after login/register.
type RoomInviteClaimer interface {
	ClaimRoomInvites(ctx context.Context, userID, email string)
}

// SetRoomInviteClaimer is assigned after workspace.Service is constructed.
func (s *Service) SetRoomInviteClaimer(c RoomInviteClaimer) {
	if s != nil {
		s.roomInviteClaimer = c
	}
}

// InviteMailboxProver reports whether a workspace invitation token proves
// this mailbox (mail already reached the address). Same token plane as AcceptInvitation.
type InviteMailboxProver interface {
	ValidInviteMailbox(ctx context.Context, email, token string) bool
}

// SetInviteMailboxProver is assigned after workspace.Service is constructed.
func (s *Service) SetInviteMailboxProver(p InviteMailboxProver) {
	if s != nil {
		s.inviteMailbox = p
	}
}

func (s *Service) claimRoomInvites(ctx context.Context, userID, email string) {
	if s == nil || s.roomInviteClaimer == nil {
		return
	}
	s.roomInviteClaimer.ClaimRoomInvites(ctx, userID, email)
}

func (s *Service) accountStore() accountStore {
	if s == nil {
		return nil
	}
	if s.accounts != nil {
		return s.accounts
	}
	if s.queries != nil {
		return s.queries
	}
	return nil
}

// NewService creates an auth service.
func NewService(q *db.Queries, store TokenStore, opts ...ServiceOption) *Service {
	s := &Service{
		queries:               q,
		tokenStore:            store,
		mailer:                &noopMailer{},
		appBaseURL:            "http://localhost:8080",
		verificationTokenTTL:  defaultVerificationTokenTTL,
		passwordResetTokenTTL: defaultPasswordResetTokenTTL,
		sendTimeout:           defaultVerificationSendTimeout,
	}
	if vs, ok := store.(verificationTokenStore); ok {
		s.verifyStore = vs
	}
	if rs, ok := store.(passwordResetTokenStore); ok {
		s.resetStore = rs
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// noopMailer drops verification emails. It is the default when no mailer is configured.
type noopMailer struct{}

func (n *noopMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) (string, error) {
	return "", nil
}

func (n *noopMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (string, error) {
	return "", nil
}

func (n *noopMailer) SendEmail(ctx context.Context, job mailer.EmailJob) (string, error) {
	return "", nil
}

func userFromDB(u db.User) User {
	return User{
		ID:            uuidToString(u.ID),
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt.Time.Format(time.RFC3339),
	}
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

// validatePassword checks length and character classes.
func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	var (
		hasUpper   bool
		hasLower   bool
		hasDigit   bool
		hasSpecial bool
	)
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return ErrWeakPassword
	}
	return nil
}

// Register creates a new user. Production does not issue a session until the
// mailbox is verified. Non-production auto-verify still returns tokens so E2E
// can complete without a real inbox.
func (s *Service) Register(ctx context.Context, email, password string) (RegisterResult, error) {
	canonical := emailid.Canonical(email)
	if !emailRegex.MatchString(canonical) {
		return RegisterResult{}, ErrInvalidEmail
	}
	if emailid.IsDisposable(canonical) {
		return RegisterResult{}, ErrDisposableEmail
	}
	if err := validatePassword(password); err != nil {
		return RegisterResult{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("bcrypt hash: %w", err)
	}

	store := s.accountStore()
	if store == nil {
		return RegisterResult{}, fmt.Errorf("create user: account store is not configured")
	}

	u, err := store.CreateUser(ctx, db.CreateUserParams{
		Email:         canonical,
		PasswordHash:  string(hash),
		EmailVerified: s.autoVerifyEmail,
	})
	if err != nil {
		if isUniqueViolation(err) {
			existing, resolved, resolveErr := s.resolveRegisterConflict(ctx, store, canonical, string(hash))
			if resolveErr != nil {
				return RegisterResult{}, resolveErr
			}
			u = existing
			if !resolved {
				if !u.EmailVerified {
					s.enqueueVerificationEmail(ctx, uuidToString(u.ID), u.Email)
				}
				return RegisterResult{User: userFromDB(u), VerificationRequired: !u.EmailVerified}, nil
			}
		} else {
			return RegisterResult{}, fmt.Errorf("create user: %w", err)
		}
	}

	if !u.EmailVerified {
		s.enqueueVerificationEmail(ctx, uuidToString(u.ID), u.Email)
		return RegisterResult{User: userFromDB(u), VerificationRequired: true}, nil
	}

	pair, err := s.issueSession(ctx, uuidToString(u.ID))
	if err != nil {
		return RegisterResult{}, err
	}
	s.claimRoomInvites(ctx, uuidToString(u.ID), u.Email)
	return RegisterResult{User: userFromDB(u), Pair: pair}, nil
}

// resolveRegisterConflict handles a unique email clash.
// verified → ErrEmailExists.
// unverified older than 48h → delete and recreate (resolved=true).
// unverified still fresh → return the existing row (resolved=false) so the
// caller can resend without leaking "already registered".
func (s *Service) resolveRegisterConflict(ctx context.Context, store accountStore, canonical, passwordHash string) (db.User, bool, error) {
	existing, err := s.lookupUserByEmail(ctx, canonical)
	if err != nil {
		return db.User{}, false, ErrEmailExists
	}
	if existing.EmailVerified {
		return db.User{}, false, ErrEmailExists
	}
	created := existing.CreatedAt.Time
	if !existing.CreatedAt.Valid || time.Since(created) <= unverifiedReclaimAfter {
		return existing, false, nil
	}
	if delErr := store.DeleteUnverifiedUser(ctx, existing.ID); delErr != nil {
		return db.User{}, false, ErrEmailExists
	}
	u, err := store.CreateUser(ctx, db.CreateUserParams{
		Email:         canonical,
		PasswordHash:  passwordHash,
		EmailVerified: s.autoVerifyEmail,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.User{}, false, ErrEmailExists
		}
		return db.User{}, false, fmt.Errorf("create user: %w", err)
	}
	return u, true, nil
}

// Login validates credentials and returns a token pair.
// inviteToken is optional. When the password matches an unverified account,
// a valid workspace invitation for the same mailbox is mailbox proof and
// completes verification (no third token plane).
func (s *Service) Login(ctx context.Context, email, password, inviteToken string) (User, TokenPair, error) {
	u, err := s.lookupUserByEmail(ctx, email)
	if err != nil {
		return User{}, TokenPair{}, ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return User{}, TokenPair{}, ErrUnauthorized
	}
	if !u.EmailVerified {
		if !s.proveInviteMailbox(ctx, u.Email, inviteToken) {
			return User{}, TokenPair{}, ErrEmailNotVerified
		}
		if err := s.VerifyEmail(ctx, uuidToString(u.ID)); err != nil {
			return User{}, TokenPair{}, err
		}
		u.EmailVerified = true
	}

	pair, err := s.issueSession(ctx, uuidToString(u.ID))
	if err != nil {
		return User{}, TokenPair{}, err
	}
	s.claimRoomInvites(ctx, uuidToString(u.ID), u.Email)
	return userFromDB(u), pair, nil
}

func (s *Service) lookupUserByEmail(ctx context.Context, raw string) (db.User, error) {
	store := s.accountStore()
	if store == nil {
		return db.User{}, pgx.ErrNoRows
	}
	canonical := emailid.Canonical(raw)
	u, err := store.GetUserByEmail(ctx, canonical)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, err
	}
	fallback := strings.ToLower(strings.TrimSpace(raw))
	if fallback == "" || fallback == canonical {
		return db.User{}, err
	}
	return store.GetUserByEmail(ctx, fallback)
}

func (s *Service) proveInviteMailbox(ctx context.Context, email, token string) bool {
	if s == nil || s.inviteMailbox == nil || strings.TrimSpace(token) == "" {
		return false
	}
	return s.inviteMailbox.ValidInviteMailbox(ctx, email, token)
}

func (s *Service) issueSession(ctx context.Context, userID string) (TokenPair, error) {
	pair, err := GenerateTokenPair(userID, accessTokenDuration, refreshTokenDuration)
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate token pair: %w", err)
	}
	if err := s.tokenStore.StoreRefreshToken(ctx, userID, pair.RefreshToken, refreshTokenDuration); err != nil {
		return TokenPair{}, fmt.Errorf("store refresh token: %w", err)
	}
	return pair, nil
}

// Logout revokes the current access token and its refresh token.
func (s *Service) Logout(ctx context.Context, accessToken, refreshToken string) error {
	claims, err := ParseToken(accessToken)
	if err != nil {
		return err
	}
	accessTTL := time.Until(time.Unix(claims.Expires, 0))
	if accessTTL > 0 {
		if err := s.tokenStore.BlocklistToken(ctx, accessToken, accessTTL); err != nil {
			return err
		}
	}
	if refreshToken != "" {
		if err := s.tokenStore.RevokeRefreshToken(ctx, claims.Subject, refreshToken); err != nil {
			return err
		}
	}
	return nil
}

// Refresh issues a new access token given a valid refresh token.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims, err := ParseToken(refreshToken)
	if err != nil {
		return TokenPair{}, err
	}
	valid, err := s.tokenStore.ValidateRefreshToken(ctx, claims.Subject, refreshToken)
	if err != nil {
		return TokenPair{}, err
	}
	if !valid {
		return TokenPair{}, ErrTokenInvalid
	}
	user, err := s.GetUser(ctx, claims.Subject)
	if err != nil {
		return TokenPair{}, ErrTokenInvalid
	}
	if !user.EmailVerified {
		_ = s.tokenStore.RevokeRefreshToken(ctx, claims.Subject, refreshToken)
		return TokenPair{}, ErrEmailNotVerified
	}
	pair, err := GenerateTokenPair(claims.Subject, accessTokenDuration, refreshTokenDuration)
	if err != nil {
		return TokenPair{}, err
	}
	if err := s.tokenStore.RevokeRefreshToken(ctx, claims.Subject, refreshToken); err != nil {
		return TokenPair{}, err
	}
	if err := s.tokenStore.StoreRefreshToken(ctx, claims.Subject, pair.RefreshToken, refreshTokenDuration); err != nil {
		return TokenPair{}, err
	}
	return pair, nil
}

// ValidateAccessToken checks that a token is syntactically valid and not revoked.
func (s *Service) ValidateAccessToken(ctx context.Context, token string) (*TokenClaims, error) {
	claims, err := ParseToken(token)
	if err != nil {
		return nil, err
	}
	revoked, err := s.tokenStore.IsTokenBlocklisted(ctx, token)
	if err != nil {
		return nil, err
	}
	if revoked {
		return nil, ErrTokenRevoked
	}
	return claims, nil
}

// GetUser returns the public profile for a user ID (e.g. current session /auth/me).
func (s *Service) GetUser(ctx context.Context, userID string) (User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return User{}, ErrUnauthorized
	}
	store := s.accountStore()
	if store == nil {
		return User{}, ErrUnauthorized
	}
	u, err := store.GetUserByID(ctx, pgtype.UUID{Bytes: uid, Valid: true})
	if err != nil {
		return User{}, ErrUnauthorized
	}
	return userFromDB(u), nil
}

// VerifyEmail marks a user's email as verified.
func (s *Service) VerifyEmail(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrTokenInvalid
	}
	store := s.accountStore()
	if store == nil {
		return ErrTokenInvalid
	}
	return store.VerifyUserEmail(ctx, pgtype.UUID{Bytes: uid, Valid: true})
}

// VerifyEmailByToken verifies a user via a single-use token, claims room
// invites, activates Trial, and issues a session (clicking the link completes
// registration).
func (s *Service) VerifyEmailByToken(ctx context.Context, token string) (User, TokenPair, error) {
	if s.verifyStore == nil {
		return User{}, TokenPair{}, ErrTokenInvalid
	}
	userID, err := s.verifyStore.UserIDByVerificationToken(ctx, token)
	if err != nil {
		return User{}, TokenPair{}, ErrTokenInvalid
	}
	defer func() { _ = s.verifyStore.DeleteVerificationToken(ctx, token) }()
	if err := s.VerifyEmail(ctx, userID); err != nil {
		return User{}, TokenPair{}, err
	}
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	user.EmailVerified = true
	s.claimRoomInvites(ctx, userID, user.Email)
	if s.trialActivator != nil {
		if actErr := s.trialActivator.ActivateEligibleTrial(ctx, userID); actErr != nil {
			logger.ErrorCtx(ctx, "auth: activate trial after verify failed", actErr,
				slog.String("user_id", userID),
			)
		}
	}
	pair, err := s.issueSession(ctx, userID)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	return user, pair, nil
}

// ResendVerification sends a new activation email when an unverified account
// exists. Always returns nil to the caller so handlers can 200 (anti-enum).
func (s *Service) ResendVerification(ctx context.Context, email string) {
	canonical := emailid.Canonical(email)
	if !emailRegex.MatchString(canonical) {
		return
	}
	u, err := s.lookupUserByEmail(ctx, email)
	if err != nil || u.EmailVerified {
		return
	}
	s.enqueueVerificationEmail(ctx, uuidToString(u.ID), u.Email)
}

// enqueueVerificationEmail starts a detached send so HTTP Register can return
// immediately. Uses WithoutCancel so client disconnect cannot abort token minting.
func (s *Service) enqueueVerificationEmail(parent context.Context, userID, email string) {
	if s == nil || s.verifyStore == nil || s.mailer == nil {
		return
	}
	timeout := s.sendTimeout
	if timeout <= 0 {
		timeout = defaultVerificationSendTimeout
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
		defer cancel()
		if err := s.sendVerificationEmail(ctx, userID, email); err != nil {
			logger.ErrorCtx(ctx, "auth: verification email send failed", err,
				slog.String("user_id", userID),
				slog.String("email", email),
			)
		}
	}()
}

func (s *Service) sendVerificationEmail(ctx context.Context, userID, email string) error {
	if s.verifyStore == nil || s.mailer == nil {
		return nil
	}
	token, err := s.verifyStore.CreateVerificationToken(ctx, userID, s.verificationTokenTTL)
	if err != nil {
		return fmt.Errorf("create verification token: %w", err)
	}
	link := fmt.Sprintf("%s/verify-email/%s", strings.TrimRight(s.appBaseURL, "/"), token)
	_, err = s.mailer.SendVerificationEmail(ctx, email, link)
	if err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}
	return nil
}

// isUniqueViolation is a simple pgx unique-violation check.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "unique constraint")
}
