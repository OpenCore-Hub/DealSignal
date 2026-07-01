package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost           = 12
	accessTokenDuration  = 15 * time.Minute
	refreshTokenDuration = 7 * 24 * time.Hour
	verificationTokenTTL = 24 * time.Hour
)

var (
	ErrEmailExists      = errors.New("email already registered")
	ErrInvalidEmail     = errors.New("invalid email address")
	ErrUnauthorized     = errors.New("invalid email or password")
	ErrTokenInvalid     = errors.New("invalid or expired token")
	ErrTokenRevoked     = errors.New("token has been revoked")
	ErrWeakPassword     = errors.New("password does not meet complexity requirements")
	ErrEmailNotVerified = errors.New("email not verified")
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

// User is the public view of a db.User.
type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	CreatedAt     string `json:"created_at"`
}

// Service handles user authentication.
type Service struct {
	queries     *db.Queries
	tokenStore  TokenStore
	verifyStore verificationTokenStore
	mailer      mailer.Mailer
	appBaseURL  string
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

// NewService creates an auth service.
func NewService(q *db.Queries, store TokenStore, opts ...ServiceOption) *Service {
	s := &Service{
		queries:    q,
		tokenStore: store,
		mailer:     &noopMailer{},
		appBaseURL: "http://localhost:8080",
	}
	if vs, ok := store.(verificationTokenStore); ok {
		s.verifyStore = vs
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// noopMailer drops verification emails. It is the default when no mailer is configured.
type noopMailer struct{}

func (n *noopMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) error {
	return nil
}

func (n *noopMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) error {
	return nil
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

// Register creates a new user and returns a token pair.
func (s *Service) Register(ctx context.Context, email, password string) (User, TokenPair, error) {
	if !emailRegex.MatchString(email) {
		return User{}, TokenPair{}, ErrInvalidEmail
	}
	if err := validatePassword(password); err != nil {
		return User{}, TokenPair{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return User{}, TokenPair{}, err
	}

	u, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		PasswordHash: string(hash),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, TokenPair{}, ErrEmailExists
		}
		return User{}, TokenPair{}, err
	}

	pair, err := GenerateTokenPair(uuidToString(u.ID), accessTokenDuration, refreshTokenDuration)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	if err := s.tokenStore.StoreRefreshToken(ctx, uuidToString(u.ID), pair.RefreshToken, refreshTokenDuration); err != nil {
		return User{}, TokenPair{}, err
	}
	if err := s.sendVerificationEmail(ctx, uuidToString(u.ID), u.Email); err != nil {
		return User{}, TokenPair{}, err
	}
	return userFromDB(u), pair, nil
}

// Login validates credentials and returns a token pair.
func (s *Service) Login(ctx context.Context, email, password string) (User, TokenPair, error) {
	u, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, TokenPair{}, ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return User{}, TokenPair{}, ErrUnauthorized
	}

	pair, err := GenerateTokenPair(uuidToString(u.ID), accessTokenDuration, refreshTokenDuration)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	if err := s.tokenStore.StoreRefreshToken(ctx, uuidToString(u.ID), pair.RefreshToken, refreshTokenDuration); err != nil {
		return User{}, TokenPair{}, err
	}
	return userFromDB(u), pair, nil
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

// VerifyEmail marks a user's email as verified.
func (s *Service) VerifyEmail(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrTokenInvalid
	}
	return s.queries.VerifyUserEmail(ctx, pgtype.UUID{Bytes: uid, Valid: true})
}

// VerifyEmailByToken verifies a user via a single-use token.
func (s *Service) VerifyEmailByToken(ctx context.Context, token string) error {
	if s.verifyStore == nil {
		return ErrTokenInvalid
	}
	userID, err := s.verifyStore.UserIDByVerificationToken(ctx, token)
	if err != nil {
		return ErrTokenInvalid
	}
	defer func() { _ = s.verifyStore.DeleteVerificationToken(ctx, token) }()
	return s.VerifyEmail(ctx, userID)
}

func (s *Service) sendVerificationEmail(ctx context.Context, userID, email string) error {
	if s.verifyStore == nil || s.mailer == nil {
		return nil
	}
	token, err := s.verifyStore.CreateVerificationToken(ctx, userID, verificationTokenTTL)
	if err != nil {
		return err
	}
	link := fmt.Sprintf("%s/verify-email/%s", strings.TrimRight(s.appBaseURL, "/"), token)
	return s.mailer.SendVerificationEmail(ctx, email, link)
}

// isUniqueViolation is a simple pgx unique-violation check.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "unique constraint")
}
