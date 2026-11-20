package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/domain"
	"github.com/niteshs1/streakit-api/internal/repository"
	"github.com/niteshs1/streakit-api/internal/response"
	"github.com/niteshs1/streakit-api/internal/service"
)

// ---- Auth ----

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=255" example:"owner@streakit.app"`
	Password string `json:"password" binding:"required,max=256" example:"correct-horse-battery"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required,max=256"`
	NewPassword     string `json:"new_password" binding:"required,max=256" minLength:"12"`
}

// ProfileResponse is the only body returned by login/refresh/me.
type ProfileResponse struct {
	ID    uuid.UUID   `json:"id" example:"0b9c2f3e-8a1d-4c55-9f2e-3d1b7a6c9e10"`
	Name  string      `json:"name" example:"Asha Rao"`
	Email string      `json:"email" example:"owner@streakit.app"`
	Role  domain.Role `json:"role" enums:"super_admin,admin" example:"super_admin"`
}

func ToProfile(u *domain.User) ProfileResponse {
	return ProfileResponse{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role}
}

// ---- Team ----

type CreateUserRequest struct {
	Name     string `json:"name" binding:"required,max=100" example:"Ravi Kumar"`
	Email    string `json:"email" binding:"required,email,max=255" example:"ravi@streakit.app"`
	Password string `json:"password" binding:"required,max=256" minLength:"12" example:"a-long-initial-password"`
	Role     string `json:"role" binding:"omitempty,oneof=super_admin admin" enums:"super_admin,admin" example:"admin"`
}

func (r CreateUserRequest) ToInput() service.CreateUserInput {
	return service.CreateUserInput{Name: r.Name, Email: r.Email, Password: r.Password, Role: domain.Role(r.Role)}
}

// UpdateUserRequest edits profile fields only; status and role have
// dedicated endpoints.
type UpdateUserRequest struct {
	Name  *string `json:"name" binding:"omitempty,max=100" example:"Ravi K."`
	Email *string `json:"email" binding:"omitempty,email,max=255" example:"ravi.k@streakit.app"`
}

func (r UpdateUserRequest) ToInput() service.UpdateUserInput {
	return service.UpdateUserInput{Name: r.Name, Email: r.Email}
}

type ChangeRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=super_admin admin" enums:"super_admin,admin" example:"admin"`
}

type UserListQuery struct {
	PageQuery
	Role   string `form:"role" binding:"omitempty,oneof=super_admin admin" enums:"super_admin,admin"`
	Status string `form:"status" binding:"omitempty,oneof=active blocked" enums:"active,blocked"`
	Q      string `form:"q" binding:"omitempty,max=100"`
}

func (q UserListQuery) ToFilter() repository.UserFilter {
	return repository.UserFilter{
		PageRequest: q.ToDomain(),
		Role:        optEnum[domain.Role](q.Role),
		Status:      optEnum[domain.UserStatus](q.Status),
		Query:       q.Q,
	}
}

type UserResponse struct {
	ID          uuid.UUID         `json:"id"`
	Name        string            `json:"name"`
	Email       string            `json:"email"`
	Role        domain.Role       `json:"role" enums:"super_admin,admin"`
	Status      domain.UserStatus `json:"status" enums:"active,blocked"`
	LastLoginAt *time.Time        `json:"last_login_at"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

func ToUser(u *domain.User) UserResponse {
	return UserResponse{
		ID:          u.ID,
		Name:        u.Name,
		Email:       u.Email,
		Role:        u.Role,
		Status:      u.Status,
		LastLoginAt: utcPtr(u.LastLoginAt),
		CreatedAt:   u.CreatedAt.UTC(),
		UpdatedAt:   u.UpdatedAt.UTC(),
	}
}

func ToUserPage(p domain.PageResult[domain.User]) response.Page[UserResponse] {
	return mapPage(p, ToUser)
}
