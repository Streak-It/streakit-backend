package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/auth"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

type CreateUserInput struct {
	Name     string
	Email    string
	Password string
	Role     domain.Role // empty = admin
}

type UpdateUserInput struct {
	Name  *string
	Email *string
}

// TeamService manages staff accounts. All writes require super_admin.
type TeamService interface {
	Create(ctx context.Context, actor domain.Actor, in CreateUserInput) (*domain.User, error)
	List(ctx context.Context, f repository.UserFilter) (domain.PageResult[domain.User], error)
	Get(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in UpdateUserInput) (*domain.User, error)
	Block(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.User, error)
	Unblock(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.User, error)
	ChangeRole(ctx context.Context, actor domain.Actor, id uuid.UUID, role domain.Role) (*domain.User, error)
}

type TeamDeps struct {
	Tx            repository.TxManager
	Users         repository.UserRepository
	RefreshTokens repository.RefreshTokenRepository
	Audit         repository.AuditRepository
	Hasher        *auth.PasswordHasher
	Clock         Clock
}

type teamService struct {
	TeamDeps
	audit auditor
}

func NewTeamService(d TeamDeps) TeamService {
	d.Clock = clockOrDefault(d.Clock)
	return &teamService{TeamDeps: d, audit: auditor{d.Audit}}
}

var userSorts = []string{"created_at", "updated_at", "name", "email", "last_login_at"}

func (s *teamService) Create(ctx context.Context, actor domain.Actor, in CreateUserInput) (*domain.User, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return nil, err
	}

	var errs fieldErrors
	name, fe := requiredText("name", in.Name)
	errs.add(fe)
	email := normalizeEmail(in.Email)
	if email == "" {
		errs.addf("email", "required")
	} else if validateEmail(email) != nil {
		errs.addf("email", "must be a valid email address")
	}
	if err := auth.ValidatePassword(in.Password); err != nil {
		errs.addf("password", "%s", err.Error())
	}
	role := in.Role
	if role == "" {
		role = domain.RoleAdmin
	}
	if !role.Valid() {
		errs.addf("role", "must be one of: super_admin, admin")
	}
	if err := errs.err(); err != nil {
		return nil, err
	}

	hash, err := s.Hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}

	var created *domain.User
	err = s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		created, err = s.Users.Create(ctx, &domain.User{
			Name:         name,
			Email:        email,
			PasswordHash: hash,
			Role:         role,
			Status:       domain.UserActive,
		})
		if err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, ActionUserCreate, domain.EntityUser, created.ID, nil, userSnapshot(created))
	})
	return created, err
}

func (s *teamService) List(ctx context.Context, f repository.UserFilter) (domain.PageResult[domain.User], error) {
	page, err := normalizePage(f.PageRequest, userSorts, "-created_at")
	if err != nil {
		return domain.PageResult[domain.User]{}, err
	}
	f.PageRequest = page
	res, err := s.Users.List(ctx, f)
	return toPage(res), err
}

func (s *teamService) Get(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.Users.GetByID(ctx, id)
}

func (s *teamService) Update(ctx context.Context, actor domain.Actor, id uuid.UUID, in UpdateUserInput) (*domain.User, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return nil, err
	}

	var (
		errs fieldErrors
		upd  repository.UserUpdate
	)
	if in.Name != nil {
		name, fe := requiredText("name", *in.Name)
		errs.add(fe)
		upd.Name = &name
	}
	if in.Email != nil {
		email := normalizeEmail(*in.Email)
		if validateEmail(email) != nil {
			errs.addf("email", "must be a valid email address")
		}
		upd.Email = &email
	}
	if err := errs.err(); err != nil {
		return nil, err
	}

	var updated *domain.User
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := s.Users.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if (upd.Name == nil || *upd.Name == cur.Name) && (upd.Email == nil || *upd.Email == cur.Email) {
			updated = cur
			return nil
		}
		if updated, err = s.Users.Update(ctx, id, upd); err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, ActionUserUpdate, domain.EntityUser, id, userSnapshot(cur), userSnapshot(updated))
	})
	return updated, err
}

func (s *teamService) Block(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.User, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return nil, err
	}
	if actor.ID == id {
		return nil, apperror.Forbidden("You cannot block yourself")
	}

	var updated *domain.User
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := s.lockTargetForGuard(ctx, id)
		if err != nil {
			return err
		}
		if cur.Status == domain.UserBlocked {
			return apperror.Conflict("User is already blocked")
		}
		if err := s.guardLastSuperAdmin(ctx, cur, "block"); err != nil {
			return err
		}

		if updated, err = s.Users.Update(ctx, id, repository.UserUpdate{Status: ptr(domain.UserBlocked)}); err != nil {
			return err
		}
		if err := s.RefreshTokens.RevokeAllForUser(ctx, id, s.Clock()); err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, ActionUserBlock, domain.EntityUser, id, userSnapshot(cur), userSnapshot(updated))
	})
	return updated, err
}

func (s *teamService) Unblock(ctx context.Context, actor domain.Actor, id uuid.UUID) (*domain.User, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return nil, err
	}

	var updated *domain.User
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := s.Users.LockByID(ctx, id)
		if err != nil {
			return err
		}
		if cur.Status == domain.UserActive {
			return apperror.Conflict("User is not blocked")
		}
		if updated, err = s.Users.Update(ctx, id, repository.UserUpdate{Status: ptr(domain.UserActive)}); err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, ActionUserUnblock, domain.EntityUser, id, userSnapshot(cur), userSnapshot(updated))
	})
	return updated, err
}

func (s *teamService) ChangeRole(ctx context.Context, actor domain.Actor, id uuid.UUID, role domain.Role) (*domain.User, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return nil, err
	}
	if !role.Valid() {
		return nil, apperror.Field("role", "must be one of: super_admin, admin")
	}
	if actor.ID == id {
		return nil, apperror.Forbidden("You cannot change your own role")
	}

	var updated *domain.User
	err := s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		cur, err := s.lockTargetForGuard(ctx, id)
		if err != nil {
			return err
		}
		if cur.Role == role {
			return apperror.Conflict("User already has this role")
		}
		if role != domain.RoleSuperAdmin {
			if err := s.guardLastSuperAdmin(ctx, cur, "demote"); err != nil {
				return err
			}
		}

		if updated, err = s.Users.Update(ctx, id, repository.UserUpdate{Role: &role}); err != nil {
			return err
		}
		if err := s.RefreshTokens.RevokeAllForUser(ctx, id, s.Clock()); err != nil {
			return err
		}
		return s.audit.record(ctx, &actor.ID, ActionUserRoleChange, domain.EntityUser, id, userSnapshot(cur), userSnapshot(updated))
	})
	return updated, err
}

// lockTargetForGuard locks the active super admin set first, then the
// target, so concurrent block/demote requests always lock in the same
// order and cannot both remove the "last" super admin.
func (s *teamService) lockTargetForGuard(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if _, err := s.Users.LockActiveSuperAdminIDs(ctx); err != nil {
		return nil, err
	}
	return s.Users.LockByID(ctx, id)
}

func (s *teamService) guardLastSuperAdmin(ctx context.Context, target *domain.User, verb string) error {
	if target.Role != domain.RoleSuperAdmin || target.Status != domain.UserActive {
		return nil
	}
	ids, err := s.Users.LockActiveSuperAdminIDs(ctx)
	if err != nil {
		return err
	}
	if len(ids) <= 1 {
		return apperror.Conflict("Cannot " + verb + " the last active super admin")
	}
	return nil
}
