package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth/emailid"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

func (s *Service) passwordResetTTL() time.Duration {
	if s == nil || s.passwordResetTokenTTL <= 0 {
		return defaultPasswordResetTokenTTL
	}
	return s.passwordResetTokenTTL
}

// ForgotPassword emails a reset link when a verified account exists.
// Always returns to the caller so handlers can 200 (anti-enum).
func (s *Service) ForgotPassword(ctx context.Context, email string) {
	canonical := emailid.Canonical(email)
	if !emailRegex.MatchString(canonical) {
		return
	}
	u, err := s.lookupUserByEmail(ctx, email)
	if err != nil || !u.EmailVerified {
		return
	}
	s.enqueuePasswordResetEmail(ctx, uuidToString(u.ID), u.Email)
}

func (s *Service) enqueuePasswordResetEmail(parent context.Context, userID, email string) {
	if s == nil || s.resetStore == nil || s.mailer == nil {
		return
	}
	timeout := s.sendTimeout
	if timeout <= 0 {
		timeout = defaultVerificationSendTimeout
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
		defer cancel()
		if err := s.sendPasswordResetEmail(ctx, userID, email); err != nil {
			logger.ErrorCtx(ctx, "auth: password reset email send failed", err,
				slog.String("user_id", userID),
				slog.String("email", email),
			)
		}
	}()
}

func (s *Service) sendPasswordResetEmail(ctx context.Context, userID, email string) error {
	if s.resetStore == nil || s.mailer == nil {
		return nil
	}
	token, err := s.resetStore.CreatePasswordResetToken(ctx, userID, s.passwordResetTTL())
	if err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}
	link := fmt.Sprintf("%s/reset-password/%s", strings.TrimRight(s.appBaseURL, "/"), token)
	minutes := fmt.Sprintf("%d", int(s.passwordResetTTL().Minutes()))
	_, err = s.mailer.SendEmail(ctx, mailer.EmailJob{
		EmailType: mailer.EmailTypePasswordReset,
		Recipient: email,
		ResetLink: link,
		TemplateVariables: map[string]string{
			"ResetLink":     link,
			"ExpiryMinutes": minutes,
		},
	})
	if err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
	return nil
}

// ResetPassword consumes a one-time token, updates the password hash, and
// revokes every refresh token. It never issues a session and never marks
// the mailbox verified.
func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if s == nil || s.resetStore == nil {
		return ErrTokenInvalid
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	userID, err := s.resetStore.ConsumePasswordResetToken(ctx, strings.TrimSpace(token))
	if err != nil || userID == "" {
		return ErrTokenInvalid
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrTokenInvalid
	}
	store := s.accountStore()
	if store == nil {
		return ErrTokenInvalid
	}
	u, err := store.GetUserByID(ctx, pgtype.UUID{Bytes: uid, Valid: true})
	if err != nil || !u.EmailVerified {
		return ErrTokenInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := store.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:           u.ID,
		PasswordHash: string(hash),
	}); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if s.tokenStore != nil {
		_ = s.tokenStore.RevokeAllUserRefreshTokens(ctx, userID)
	}
	return nil
}
