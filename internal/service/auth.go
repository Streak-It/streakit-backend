package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/auth"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

// Session is the credential pair placed in cookies by the handler.
// Tokens never appear in response bodies.
type Session struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type LoginInput struct {
	Email    string
	Password string
	ClientIP string
}

type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

type BootstrapInput struct {
	Email    string
	Password string
	Name     string
}

type AuthService interface {
	Login(ctx context.Context, in LoginInput) (*domain.User, *Session, error)
	// Refresh rotates the refresh token. Reuse of a revoked token revokes
	// every session of its owner.
	Refresh(ctx context.Context, refreshToken string) (*domain.User, *Session, error)
	// Logout revokes the given refresh token; unknown tokens are ignored.
	Logout(ctx context.Context, refreshToken string) error
	// Authenticate verifies an access token and loads the caller from the
	// DB, so role and status are never taken from the token.
	Authenticate(ctx context.Context, accessToken string) (*domain.User, error)
	// ChangePassword revokes all of the caller's refresh tokens on success.
	ChangePassword(ctx context.Context, actor domain.Actor, in ChangePasswordInput) error
	// Bootstrap ensures at least one active super admin exists.
	Bootstrap(ctx context.Context, in BootstrapInput) error
}

type AuthDeps struct {
	Tx            repository.TxManager
	Users         repository.UserRepository
	RefreshTokens repository.RefreshTokenRepository
	Audit         repository.AuditRepository
	Hasher        *auth.PasswordHasher
	Tokens        *auth.TokenManager
	RefreshMinter *auth.RefreshTokens
	RefreshTTL    time.Duration
	IPLimiter     Limiter
	EmailLimiter  Limiter
	Clock         Clock
	Logger        *slog.Logger
}

type authService struct {
	AuthDeps
	audit auditor
}

func NewAuthService(d AuthDeps) AuthService {
	d.Clock = clockOrDefault(d.Clock)
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &authService{AuthDeps: d, audit: auditor{d.Audit}}
}

func (s *authService) Login(ctx context.Context, in LoginInput) (*domain.User, *Session, error) {
	email := normalizeEmail(in.Email)

	// Check both buckets so neither is skipped by short-circuiting.
	ipOK := s.IPLimiter == nil || s.IPLimiter.Allow("ip:"+in.ClientIP)
	emailOK := s.EmailLimiter == nil || s.EmailLimiter.Allow("email:"+email)
	if !ipOK || !emailOK {
		return nil, nil, apperror.RateLimited()
	}

	u, err := s.Users.GetByEmail(ctx, email)
	if apperror.HasCode(err, apperror.CodeNotFound) {
		// Equalise timing with the wrong-password path.
		s.Hasher.CompareDummy(in.Password)
		return nil, nil, apperror.InvalidCredentials()
	}
	if err != nil {
		return nil, nil, err
	}

	// Always compare first so blocked accounts take the same time.
	if !s.Hasher.Compare(u.PasswordHash, in.Password) || !u.IsActive() {
		return nil, nil, apperror.InvalidCredentials()
	}

	var session *Session
	err = s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		now := s.Clock()
		updated, err := s.Users.Update(ctx, u.ID, repository.UserUpdate{LastLoginAt: &now})
		if err != nil {
			return err
		}
		u = updated

		if session, err = s.issueSession(ctx, u); err != nil {
			return err
		}
		return s.audit.record(ctx, &u.ID, ActionAuthLogin, domain.EntityUser, u.ID, nil, nil)
	})
	if err != nil {
		return nil, nil, err
	}
	return u, session, nil
}

func (s *authService) Refresh(ctx context.Context, raw string) (*domain.User, *Session, error) {
	if raw == "" {
		return nil, nil, apperror.InvalidCredentials()
	}

	var (
		u          *domain.User
		session    *Session
		reuseOwner uuid.UUID
	)

	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		now := s.Clock()

		rt, err := s.RefreshTokens.LockByHash(ctx, auth.Hash(raw))
		if apperror.HasCode(err, apperror.CodeNotFound) {
			// A token we minted (valid MAC) whose row is gone counts as reuse.
			if owner, ok := s.RefreshMinter.Owner(raw); ok {
				reuseOwner = owner
			}
			return apperror.InvalidCredentials()
		}
		if err != nil {
			return err
		}

		if rt.RevokedAt != nil {
			reuseOwner = rt.UserID
			return apperror.InvalidCredentials()
		}
		if !now.Before(rt.ExpiresAt) {
			return apperror.InvalidCredentials()
		}

		u, err = s.Users.GetByID(ctx, rt.UserID)
		if err != nil {
			if apperror.HasCode(err, apperror.CodeNotFound) {
				return apperror.InvalidCredentials()
			}
			return err
		}
		if !u.IsActive() {
			return apperror.InvalidCredentials()
		}

		if err := s.RefreshTokens.Revoke(ctx, rt.ID, now); err != nil {
			return err
		}
		session, err = s.issueSession(ctx, u)
		return err
	})

	if reuseOwner != uuid.Nil {
		// Runs outside the (rolled back) transaction so it always commits.
		s.Logger.WarnContext(ctx, "refresh token reuse detected; revoking all sessions", "user_id", reuseOwner)
		if rErr := s.RefreshTokens.RevokeAllForUser(context.WithoutCancel(ctx), reuseOwner, s.Clock()); rErr != nil {
			return nil, nil, errors.Join(err, fmt.Errorf("revoke sessions after reuse: %w", rErr))
		}
	}
	if err != nil {
		return nil, nil, err
	}
	return u, session, nil
}

func (s *authService) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	return s.RefreshTokens.RevokeByHash(ctx, auth.Hash(raw), s.Clock())
}

func (s *authService) Authenticate(ctx context.Context, accessToken string) (*domain.User, error) {
	if accessToken == "" {
		return nil, apperror.Unauthorized("")
	}
	userID, err := s.Tokens.VerifyAccess(accessToken)
	switch {
	case errors.Is(err, auth.ErrTokenExpired):
		return nil, apperror.TokenExpired()
	case err != nil:
		return nil, apperror.Unauthorized("Invalid access token")
	}

	u, err := s.Users.GetByID(ctx, userID)
	if apperror.HasCode(err, apperror.CodeNotFound) {
		return nil, apperror.Unauthorized("Invalid access token")
	}
	if err != nil {
		return nil, err
	}
	if !u.IsActive() {
		return nil, apperror.Unauthorized("Account is blocked")
	}
	return u, nil
}

func (s *authService) ChangePassword(ctx context.Context, actor domain.Actor, in ChangePasswordInput) error {
	u, err := s.Users.GetByID(ctx, actor.ID)
	if err != nil {
		return err
	}
	if !s.Hasher.Compare(u.PasswordHash, in.CurrentPassword) {
		return apperror.Field("current_password", "is incorrect")
	}
	if err := auth.ValidatePassword(in.NewPassword); err != nil {
		return apperror.Field("new_password", err.Error())
	}
	if in.NewPassword == in.CurrentPassword {
		return apperror.Field("new_password", "must differ from the current password")
	}

	hash, err := s.Hasher.Hash(in.NewPassword)
	if err != nil {
		return err
	}

	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.Users.Update(ctx, u.ID, repository.UserUpdate{PasswordHash: &hash}); err != nil {
			return err
		}
		if err := s.RefreshTokens.RevokeAllForUser(ctx, u.ID, s.Clock()); err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, ActionAuthPasswordChange, domain.EntityUser, u.ID, nil, nil)
	})
}

// ErrBootstrapIncomplete is returned when no super admin exists and the
// bootstrap environment variables are missing.
var ErrBootstrapIncomplete = errors.New("no active super admin exists: set SUPERADMIN_EMAIL, SUPERADMIN_PASSWORD and SUPERADMIN_NAME to create one")

func (s *authService) Bootstrap(ctx context.Context, in BootstrapInput) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		ids, err := s.Users.LockActiveSuperAdminIDs(ctx)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			s.Logger.InfoContext(ctx, "bootstrap skipped: active super admin exists", "count", len(ids))
			return nil
		}

		email := normalizeEmail(in.Email)
		if email == "" || in.Password == "" || in.Name == "" {
			return ErrBootstrapIncomplete
		}
		if err := validateEmail(email); err != nil {
			return fmt.Errorf("SUPERADMIN_EMAIL: %w", err)
		}
		if err := auth.ValidatePassword(in.Password); err != nil {
			return fmt.Errorf("SUPERADMIN_PASSWORD %w", err)
		}

		hash, err := s.Hasher.Hash(in.Password)
		if err != nil {
			return err
		}
		u, err := s.Users.Create(ctx, &domain.User{
			Name:         in.Name,
			Email:        email,
			PasswordHash: hash,
			Role:         domain.RoleSuperAdmin,
			Status:       domain.UserActive,
		})
		if err != nil {
			if apperror.HasCode(err, apperror.CodeConflict) {
				return fmt.Errorf("bootstrap: a user with SUPERADMIN_EMAIL already exists but is not an active super admin; fix it manually: %w", err)
			}
			return fmt.Errorf("bootstrap: create super admin: %w", err)
		}
		s.Logger.InfoContext(ctx, "bootstrap: super admin created", "user_id", u.ID)
		return s.audit.record(ctx, nil, ActionUserBootstrap, domain.EntityUser, u.ID, nil, userSnapshot(u))
	})
}

func (s *authService) issueSession(ctx context.Context, u *domain.User) (*Session, error) {
	access, accessExp, err := s.Tokens.IssueAccess(u.ID, string(u.Role))
	if err != nil {
		return nil, err
	}
	raw, hash, err := s.RefreshMinter.Generate(u.ID)
	if err != nil {
		return nil, err
	}
	refreshExp := s.Clock().Add(s.RefreshTTL)
	if err := s.RefreshTokens.Create(ctx, u.ID, hash, refreshExp); err != nil {
		return nil, err
	}
	return &Session{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     raw,
		RefreshExpiresAt: refreshExp,
	}, nil
}
