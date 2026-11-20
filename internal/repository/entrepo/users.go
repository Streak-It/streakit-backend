package entrepo

import (
	"context"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/ent"
	"github.com/niteshs1/streakit-api/ent/user"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
)

const userEntity = "User"
const emailConflict = "Email is already in use"

type UserRepo struct{ base }

func NewUserRepo(c *ent.Client) *UserRepo { return &UserRepo{base{c}} }

var _ repository.UserRepository = (*UserRepo)(nil)

func toUser(u *ent.User) *domain.User {
	return &domain.User{
		ID:           u.ID,
		Name:         u.Name,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         domain.Role(u.Role),
		Status:       domain.UserStatus(u.Status),
		LastLoginAt:  u.LastLoginAt,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) (*domain.User, error) {
	row, err := r.db(ctx).User.Create().
		SetName(u.Name).
		SetEmail(u.Email).
		SetPasswordHash(u.PasswordHash).
		SetRole(user.Role(u.Role)).
		SetStatus(user.Status(u.Status)).
		Save(ctx)
	if err != nil {
		return nil, mapErr(err, userEntity, emailConflict)
	}
	return toUser(row), nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row, err := r.db(ctx).User.Get(ctx, id)
	if err != nil {
		return nil, mapErr(err, userEntity, "")
	}
	return toUser(row), nil
}

func (r *UserRepo) LockByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row, err := r.db(ctx).User.Query().Where(user.ID(id)).ForUpdate().Only(ctx)
	if err != nil {
		return nil, mapErr(err, userEntity, "")
	}
	return toUser(row), nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row, err := r.db(ctx).User.Query().Where(user.Email(email)).Only(ctx)
	if err != nil {
		return nil, mapErr(err, userEntity, "")
	}
	return toUser(row), nil
}

func (r *UserRepo) List(ctx context.Context, f repository.UserFilter) (domain.PageResult[domain.User], error) {
	q := r.db(ctx).User.Query()
	if f.Role != nil {
		q = q.Where(user.RoleEQ(user.Role(*f.Role)))
	}
	if f.Status != nil {
		q = q.Where(user.StatusEQ(user.Status(*f.Status)))
	}
	if f.Query != "" {
		q = q.Where(user.Or(user.NameContainsFold(f.Query), user.EmailContainsFold(f.Query)))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return domain.PageResult[domain.User]{}, mapErr(err, userEntity, "")
	}

	field, desc := f.SortField()
	rows, err := q.Order(order[user.OrderOption](field, desc)...).
		Offset(f.Offset()).Limit(f.PageSize).All(ctx)
	if err != nil {
		return domain.PageResult[domain.User]{}, mapErr(err, userEntity, "")
	}

	items := make([]domain.User, len(rows))
	for i, row := range rows {
		items[i] = *toUser(row)
	}
	return domain.PageResult[domain.User]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}

func (r *UserRepo) Update(ctx context.Context, id uuid.UUID, u repository.UserUpdate) (*domain.User, error) {
	m := r.db(ctx).User.UpdateOneID(id)
	if u.Name != nil {
		m.SetName(*u.Name)
	}
	if u.Email != nil {
		m.SetEmail(*u.Email)
	}
	if u.PasswordHash != nil {
		m.SetPasswordHash(*u.PasswordHash)
	}
	if u.Role != nil {
		m.SetRole(user.Role(*u.Role))
	}
	if u.Status != nil {
		m.SetStatus(user.Status(*u.Status))
	}
	if u.LastLoginAt != nil {
		m.SetLastLoginAt(*u.LastLoginAt)
	}
	row, err := m.Save(ctx)
	if err != nil {
		return nil, mapErr(err, userEntity, emailConflict)
	}
	return toUser(row), nil
}

func (r *UserRepo) LockActiveSuperAdminIDs(ctx context.Context) ([]uuid.UUID, error) {
	ids, err := r.db(ctx).User.Query().
		Where(user.RoleEQ(user.RoleSuperAdmin), user.StatusEQ(user.StatusActive)).
		// Consistent lock order across transactions avoids deadlocks.
		Order(user.ByID()).
		ForUpdate().
		IDs(ctx)
	if err != nil {
		return nil, mapErr(err, userEntity, "")
	}
	return ids, nil
}
