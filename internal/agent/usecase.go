package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Usecase defines business logic for agent operations
type Usecase interface {
	// User management
	InviteAgent(ctx context.Context, req *CreateUserRequest) (*User, error)
	GetAgent(ctx context.Context, id uuid.UUID) (*UserWithRoles, error)
	ListAgents(ctx context.Context, status *UserStatus) ([]User, error)
	UpdateAgentStatus(ctx context.Context, id uuid.UUID, status UserStatus) error
	DeleteAgent(ctx context.Context, id uuid.UUID) error

	// Role management
	CreateRole(ctx context.Context, req *CreateRoleRequest) (*Role, error)
	GetRole(ctx context.Context, id uuid.UUID) (*RoleWithPermissions, error)
	ListRoles(ctx context.Context) ([]Role, error)
	DeleteRole(ctx context.Context, id uuid.UUID) error
	AssignPermissionsToRole(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error

	// Permission management
	ListPermissions(ctx context.Context, module *string) ([]Permission, error)
	GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]Permission, error)

	// Workload
	GetAgentWorkload(ctx context.Context) ([]WorkloadItem, error)
}

// usecase implements Usecase interface
type usecase struct {
	repo   Repository
	logger *zap.Logger
}

// NewUsecase creates a new agent usecase
func NewUsecase(repo Repository, logger *zap.Logger) Usecase {
	return &usecase{
		repo:   repo,
		logger: logger,
	}
}

// InviteAgent creates a new agent and assigns roles
func (u *usecase) InviteAgent(ctx context.Context, req *CreateUserRequest) (*User, error) {
	// Check if user already exists
	existingUser, _ := u.repo.GetUserByEmail(ctx, req.Email)
	if existingUser != nil {
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	// Create user
	user := &User{
		ID:        uuid.New(),
		Email:     req.Email,
		FullName:  req.FullName,
		Status:    UserStatusOffline,
		CreatedAt: time.Now(),
	}

	if err := u.repo.CreateUser(ctx, user); err != nil {
		u.logger.Error("failed to create user", zap.Error(err))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Assign roles if provided
	for _, roleIDStr := range req.RoleIDs {
		roleID, err := uuid.Parse(roleIDStr)
		if err != nil {
			u.logger.Warn("invalid role ID", zap.String("role_id", roleIDStr))
			continue
		}

		if err := u.repo.AssignRoleToUser(ctx, user.ID, roleID); err != nil {
			u.logger.Error("failed to assign role to user",
				zap.String("user_id", user.ID.String()),
				zap.String("role_id", roleID.String()),
				zap.Error(err),
			)
		}
	}

	u.logger.Info("agent invited successfully",
		zap.String("user_id", user.ID.String()),
		zap.String("email", user.Email),
	)

	return user, nil
}

// GetAgent retrieves an agent with their roles
func (u *usecase) GetAgent(ctx context.Context, id uuid.UUID) (*UserWithRoles, error) {
	userWithRoles, err := u.repo.GetUserWithRoles(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return userWithRoles, nil
}

// ListAgents lists all agents, optionally filtered by status
func (u *usecase) ListAgents(ctx context.Context, status *UserStatus) ([]User, error) {
	// Validate status if provided
	if status != nil && !status.IsValid() {
		return nil, fmt.Errorf("invalid status: %s", *status)
	}

	users, err := u.repo.ListUsers(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	return users, nil
}

// UpdateAgentStatus updates an agent's status
func (u *usecase) UpdateAgentStatus(ctx context.Context, id uuid.UUID, status UserStatus) error {
	// Validate status
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %s", status)
	}

	if err := u.repo.UpdateUserStatus(ctx, id, status); err != nil {
		u.logger.Error("failed to update agent status",
			zap.String("user_id", id.String()),
			zap.String("status", string(status)),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update agent status: %w", err)
	}

	u.logger.Info("agent status updated",
		zap.String("user_id", id.String()),
		zap.String("status", string(status)),
	)

	return nil
}

// DeleteAgent deletes an agent
func (u *usecase) DeleteAgent(ctx context.Context, id uuid.UUID) error {
	if err := u.repo.DeleteUser(ctx, id); err != nil {
		u.logger.Error("failed to delete agent",
			zap.String("user_id", id.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	u.logger.Info("agent deleted", zap.String("user_id", id.String()))

	return nil
}

// CreateRole creates a new role with permissions
func (u *usecase) CreateRole(ctx context.Context, req *CreateRoleRequest) (*Role, error) {
	// Check if role with slug already exists
	existingRole, _ := u.repo.GetRoleBySlug(ctx, req.Slug)
	if existingRole != nil {
		return nil, fmt.Errorf("role with slug %s already exists", req.Slug)
	}

	// Create role
	role := &Role{
		ID:        uuid.New(),
		Name:      req.Name,
		Slug:      req.Slug,
		IsSystem:  false,
		CreatedAt: time.Now(),
	}

	if err := u.repo.CreateRole(ctx, role); err != nil {
		u.logger.Error("failed to create role", zap.Error(err))
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	// Assign permissions if provided
	for _, permIDStr := range req.PermissionIDs {
		permID, err := uuid.Parse(permIDStr)
		if err != nil {
			u.logger.Warn("invalid permission ID", zap.String("permission_id", permIDStr))
			continue
		}

		if err := u.repo.AssignPermissionToRole(ctx, role.ID, permID); err != nil {
			u.logger.Error("failed to assign permission to role",
				zap.String("role_id", role.ID.String()),
				zap.String("permission_id", permID.String()),
				zap.Error(err),
			)
		}
	}

	u.logger.Info("role created successfully",
		zap.String("role_id", role.ID.String()),
		zap.String("slug", role.Slug),
	)

	return role, nil
}

// GetRole retrieves a role with its permissions
func (u *usecase) GetRole(ctx context.Context, id uuid.UUID) (*RoleWithPermissions, error) {
	roleWithPerms, err := u.repo.GetRoleWithPermissions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	return roleWithPerms, nil
}

// ListRoles lists all roles
func (u *usecase) ListRoles(ctx context.Context) ([]Role, error) {
	roles, err := u.repo.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	return roles, nil
}

// DeleteRole deletes a role (only non-system roles)
func (u *usecase) DeleteRole(ctx context.Context, id uuid.UUID) error {
	// Check if role exists and is not system
	role, err := u.repo.GetRoleByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get role: %w", err)
	}

	if role.IsSystem {
		return fmt.Errorf("cannot delete system role")
	}

	if err := u.repo.DeleteRole(ctx, id); err != nil {
		u.logger.Error("failed to delete role",
			zap.String("role_id", id.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete role: %w", err)
	}

	u.logger.Info("role deleted", zap.String("role_id", id.String()))

	return nil
}

// AssignPermissionsToRole assigns multiple permissions to a role
func (u *usecase) AssignPermissionsToRole(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	// Verify role exists
	_, err := u.repo.GetRoleByID(ctx, roleID)
	if err != nil {
		return fmt.Errorf("role not found: %w", err)
	}

	// Assign each permission
	for _, permID := range permissionIDs {
		if err := u.repo.AssignPermissionToRole(ctx, roleID, permID); err != nil {
			u.logger.Error("failed to assign permission to role",
				zap.String("role_id", roleID.String()),
				zap.String("permission_id", permID.String()),
				zap.Error(err),
			)
			return fmt.Errorf("failed to assign permission: %w", err)
		}
	}

	u.logger.Info("permissions assigned to role",
		zap.String("role_id", roleID.String()),
		zap.Int("count", len(permissionIDs)),
	)

	return nil
}

// ListPermissions lists all permissions, optionally filtered by module
func (u *usecase) ListPermissions(ctx context.Context, module *string) ([]Permission, error) {
	permissions, err := u.repo.ListPermissions(ctx, module)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	return permissions, nil
}

// GetUserPermissions retrieves all permissions for a user
func (u *usecase) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]Permission, error) {
	permissions, err := u.repo.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}

	return permissions, nil
}

// GetAgentWorkload retrieves the workload of all agents
func (u *usecase) GetAgentWorkload(ctx context.Context) ([]WorkloadItem, error) {
	workload, err := u.repo.GetAgentWorkload(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent workload: %w", err)
	}

	return workload, nil
}
