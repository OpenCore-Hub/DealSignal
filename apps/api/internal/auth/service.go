package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost  = 12
	jwtDuration = 24 * time.Hour
)

var (
	ErrEmailExists    = errors.New("email already registered")
	ErrInvalidEmail   = errors.New("invalid email address")
	ErrUnauthorized   = errors.New("invalid email or password")
	ErrTokenInvalid   = errors.New("invalid or expired token")
	emailRegex        = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// User is the public view of a db.User.
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

// Service handles user authentication.
type Service struct {
	queries *db.Queries
}

// NewService creates an auth service.
func NewService(q *db.Queries) *Service {
	return &Service{queries: q}
}

func userFromDB(u db.User) User {
	return User{
		ID:        uuidToString(u.ID),
		Email:     u.Email,
		CreatedAt: u.CreatedAt.Time.Format(time.RFC3339),
	}
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

// Register creates a new user and returns a JWT.
func (s *Service) Register(ctx context.Context, email, password string) (User, string, error) {
	if !emailRegex.MatchString(email) {
		return User{}, "", ErrInvalidEmail
	}
	if len(password) < 8 {
		return User{}, "", errors.New("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return User{}, "", err
	}

	u, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		PasswordHash: string(hash),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, "", ErrEmailExists
		}
		return User{}, "", err
	}

	token, err := GenerateToken(uuidToString(u.ID), jwtDuration)
	if err != nil {
		return User{}, "", err
	}
	return userFromDB(u), token, nil
}

// Login validates credentials and returns a JWT.
func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	u, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, "", ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return User{}, "", ErrUnauthorized
	}

	token, err := GenerateToken(uuidToString(u.ID), jwtDuration)
	if err != nil {
		return User{}, "", err
	}
	return userFromDB(u), token, nil
}

// isUniqueViolation is a simple pgx unique-violation check.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "unique constraint")
}
