package agent

import (
	"time"

	"github.com/google/uuid"
)

// User represents an agent or user in the system
type User struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	FullName  string    `json:"full_name" db:"full_name"`
	Status    UserStatus `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// UserStatus represents the current status of a user
type UserStatus string

const (
	UserStatusOnline  UserStatus = "online"
	UserStatusOffline UserStatus = "offline"
	UserStatusBusy    UserStatus = "busy"
)

// IsValid checks if the user status is valid
func (s UserStatus) IsValid() bool {
	switch s {
	case UserStatusOnline, UserStatusOffline, UserStatusBusy:
		return true
	default:
		return false
	}
}

// Role represents a role in the RBAC system
type Role struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Slug      string    `json:"slug" db:"slug"`
	IsSystem  bool      `json:"is_system" db:"is_system"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Permission represents a specific permission in the system
type Permission struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Slug        string    `json:"slug" db:"slug"`
	Module      string    `json:"module" db:"module"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// UserWithRoles represents a user with their assigned roles
type UserWithRoles struct {
	User
	Roles []Role `json:"roles"`
}

// RoleWithPermissions represents a role with its permissions
type RoleWithPermissions struct {
	Role
	Permissions []Permission `json:"permissions"`
}

// WorkloadItem represents an agent's current workload
type WorkloadItem struct {
	UserID           uuid.UUID `json:"user_id" db:"user_id"`
	Email            string    `json:"email" db:"email"`
	FullName         string    `json:"full_name" db:"full_name"`
	Status           UserStatus `json:"status" db:"status"`
	ActiveChatsCount int       `json:"active_chats_count" db:"active_chats_count"`
}

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Email    string   `json:"email" binding:"required,email"`
	FullName string   `json:"full_name" binding:"required"`
	RoleIDs  []string `json:"role_ids"`
}

// UpdateUserStatusRequest represents the request to update user status
type UpdateUserStatusRequest struct {
	Status UserStatus `json:"status" binding:"required"`
}

// CreateRoleRequest represents the request to create a new role
type CreateRoleRequest struct {
	Name          string   `json:"name" binding:"required"`
	Slug          string   `json:"slug" binding:"required"`
	PermissionIDs []string `json:"permission_ids"`
}

// AssignPermissionsRequest represents the request to assign permissions to a role
type AssignPermissionsRequest struct {
	PermissionIDs []string `json:"permission_ids" binding:"required"`
}