package agent

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the interface for agent-related database operations
type Repository interface {
	// User operations
	CreateUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	ListUsers(ctx context.Context, status *UserStatus) ([]User, error)
	UpdateUserStatus(ctx context.Context, id uuid.UUID, status UserStatus) error
	DeleteUser(ctx context.Context, id uuid.UUID) error

	// Role operations
	CreateRole(ctx context.Context, role *Role) error
	GetRoleByID(ctx context.Context, id uuid.UUID) (*Role, error)
	GetRoleBySlug(ctx context.Context, slug string) (*Role, error)
	ListRoles(ctx context.Context) ([]Role, error)
	UpdateRole(ctx context.Context, role *Role) error
	DeleteRole(ctx context.Context, id uuid.UUID) error

	// Permission operations
	CreatePermission(ctx context.Context, permission *Permission) error
	GetPermissionByID(ctx context.Context, id uuid.UUID) (*Permission, error)
	GetPermissionBySlug(ctx context.Context, slug string) (*Permission, error)
	ListPermissions(ctx context.Context, module *string) ([]Permission, error)

	// User-Role assignments
	AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error
	RemoveRoleFromUser(ctx context.Context, userID, roleID uuid.UUID) error
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]Role, error)
	GetUserWithRoles(ctx context.Context, userID uuid.UUID) (*UserWithRoles, error)

	// Role-Permission assignments
	AssignPermissionToRole(ctx context.Context, roleID, permissionID uuid.UUID) error
	RemovePermissionFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error
	GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]Permission, error)
	GetRoleWithPermissions(ctx context.Context, roleID uuid.UUID) (*RoleWithPermissions, error)

	// RBAC checks
	HasPermission(ctx context.Context, userID uuid.UUID, permissionSlug string) (bool, error)
	GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]Permission, error)

	// Workload
	GetAgentWorkload(ctx context.Context) ([]WorkloadItem, error)
}

// postgresRepository implements Repository interface using pgx
type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL repository
func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

// CreateUser creates a new user
func (r *postgresRepository) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (id, email, full_name, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, user.ID, user.Email, user.FullName, user.Status, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by ID
func (r *postgresRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `SELECT id, email, full_name, status, created_at FROM users WHERE id = $1`

	var user User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.FullName, &user.Status, &user.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (r *postgresRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT id, email, full_name, status, created_at FROM users WHERE email = $1`

	var user User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.FullName, &user.Status, &user.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// ListUsers lists all users, optionally filtered by status
func (r *postgresRepository) ListUsers(ctx context.Context, status *UserStatus) ([]User, error) {
	query := `SELECT id, email, full_name, status, created_at FROM users`
	args := []interface{}{}

	if status != nil {
		query += ` WHERE status = $1`
		args = append(args, *status)
	}

	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.FullName, &user.Status, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}

// UpdateUserStatus updates a user's status
func (r *postgresRepository) UpdateUserStatus(ctx context.Context, id uuid.UUID, status UserStatus) error {
	query := `UPDATE users SET status = $1 WHERE id = $2`

	result, err := r.pool.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUser deletes a user
func (r *postgresRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// CreateRole creates a new role
func (r *postgresRepository) CreateRole(ctx context.Context, role *Role) error {
	query := `
		INSERT INTO roles (id, name, slug, is_system, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, role.ID, role.Name, role.Slug, role.IsSystem, role.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}
	return nil
}

// GetRoleByID retrieves a role by ID
func (r *postgresRepository) GetRoleByID(ctx context.Context, id uuid.UUID) (*Role, error) {
	query := `SELECT id, name, slug, is_system, created_at FROM roles WHERE id = $1`

	var role Role
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&role.ID, &role.Name, &role.Slug, &role.IsSystem, &role.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("role not found")
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	return &role, nil
}

// GetRoleBySlug retrieves a role by slug
func (r *postgresRepository) GetRoleBySlug(ctx context.Context, slug string) (*Role, error) {
	query := `SELECT id, name, slug, is_system, created_at FROM roles WHERE slug = $1`

	var role Role
	err := r.pool.QueryRow(ctx, query, slug).Scan(
		&role.ID, &role.Name, &role.Slug, &role.IsSystem, &role.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("role not found")
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	return &role, nil
}

// ListRoles lists all roles
func (r *postgresRepository) ListRoles(ctx context.Context) ([]Role, error) {
	query := `SELECT id, name, slug, is_system, created_at FROM roles ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Slug, &role.IsSystem, &role.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, role)
	}

	return roles, nil
}

// UpdateRole updates a role
func (r *postgresRepository) UpdateRole(ctx context.Context, role *Role) error {
	query := `UPDATE roles SET name = $1, slug = $2 WHERE id = $3 AND is_system = false`

	result, err := r.pool.Exec(ctx, query, role.Name, role.Slug, role.ID)
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("role not found or is a system role")
	}

	return nil
}

// DeleteRole deletes a role
func (r *postgresRepository) DeleteRole(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM roles WHERE id = $1 AND is_system = false`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("role not found or is a system role")
	}

	return nil
}

// CreatePermission creates a new permission
func (r *postgresRepository) CreatePermission(ctx context.Context, permission *Permission) error {
	query := `
		INSERT INTO permissions (id, slug, module, description, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, permission.ID, permission.Slug, permission.Module, permission.Description, permission.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create permission: %w", err)
	}
	return nil
}

// GetPermissionByID retrieves a permission by ID
func (r *postgresRepository) GetPermissionByID(ctx context.Context, id uuid.UUID) (*Permission, error) {
	query := `SELECT id, slug, module, description, created_at FROM permissions WHERE id = $1`

	var permission Permission
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&permission.ID, &permission.Slug, &permission.Module, &permission.Description, &permission.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("permission not found")
		}
		return nil, fmt.Errorf("failed to get permission: %w", err)
	}

	return &permission, nil
}

// GetPermissionBySlug retrieves a permission by slug
func (r *postgresRepository) GetPermissionBySlug(ctx context.Context, slug string) (*Permission, error) {
	query := `SELECT id, slug, module, description, created_at FROM permissions WHERE slug = $1`

	var permission Permission
	err := r.pool.QueryRow(ctx, query, slug).Scan(
		&permission.ID, &permission.Slug, &permission.Module, &permission.Description, &permission.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("permission not found")
		}
		return nil, fmt.Errorf("failed to get permission: %w", err)
	}

	return &permission, nil
}

// ListPermissions lists all permissions, optionally filtered by module
func (r *postgresRepository) ListPermissions(ctx context.Context, module *string) ([]Permission, error) {
	query := `SELECT id, slug, module, description, created_at FROM permissions`
	args := []interface{}{}

	if module != nil {
		query += ` WHERE module = $1`
		args = append(args, *module)
	}

	query += ` ORDER BY module, slug`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var permission Permission
		if err := rows.Scan(&permission.ID, &permission.Slug, &permission.Module, &permission.Description, &permission.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

// AssignRoleToUser assigns a role to a user
func (r *postgresRepository) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	query := `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`

	_, err := r.pool.Exec(ctx, query, userID, roleID)
	if err != nil {
		return fmt.Errorf("failed to assign role to user: %w", err)
	}

	return nil
}

// RemoveRoleFromUser removes a role from a user
func (r *postgresRepository) RemoveRoleFromUser(ctx context.Context, userID, roleID uuid.UUID) error {
	query := `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`

	result, err := r.pool.Exec(ctx, query, userID, roleID)
	if err != nil {
		return fmt.Errorf("failed to remove role from user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user-role assignment not found")
	}

	return nil
}

// GetUserRoles retrieves all roles assigned to a user
func (r *postgresRepository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]Role, error) {
	query := `
		SELECT r.id, r.name, r.slug, r.is_system, r.created_at
		FROM roles r
		INNER JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.name
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Slug, &role.IsSystem, &role.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, role)
	}

	return roles, nil
}

// GetUserWithRoles retrieves a user with their roles
func (r *postgresRepository) GetUserWithRoles(ctx context.Context, userID uuid.UUID) (*UserWithRoles, error) {
	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	roles, err := r.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &UserWithRoles{
		User:  *user,
		Roles: roles,
	}, nil
}

// AssignPermissionToRole assigns a permission to a role
func (r *postgresRepository) AssignPermissionToRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	query := `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`

	_, err := r.pool.Exec(ctx, query, roleID, permissionID)
	if err != nil {
		return fmt.Errorf("failed to assign permission to role: %w", err)
	}

	return nil
}

// RemovePermissionFromRole removes a permission from a role
func (r *postgresRepository) RemovePermissionFromRole(ctx context.Context, roleID, permissionID uuid.UUID) error {
	query := `DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`

	result, err := r.pool.Exec(ctx, query, roleID, permissionID)
	if err != nil {
		return fmt.Errorf("failed to remove permission from role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("role-permission assignment not found")
	}

	return nil
}

// GetRolePermissions retrieves all permissions assigned to a role
func (r *postgresRepository) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]Permission, error) {
	query := `
		SELECT p.id, p.slug, p.module, p.description, p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.module, p.slug
	`

	rows, err := r.pool.Query(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var permission Permission
		if err := rows.Scan(&permission.ID, &permission.Slug, &permission.Module, &permission.Description, &permission.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

// GetRoleWithPermissions retrieves a role with its permissions
func (r *postgresRepository) GetRoleWithPermissions(ctx context.Context, roleID uuid.UUID) (*RoleWithPermissions, error) {
	role, err := r.GetRoleByID(ctx, roleID)
	if err != nil {
		return nil, err
	}

	permissions, err := r.GetRolePermissions(ctx, roleID)
	if err != nil {
		return nil, err
	}

	return &RoleWithPermissions{
		Role:        *role,
		Permissions: permissions,
	}, nil
}

// HasPermission checks if a user has a specific permission
func (r *postgresRepository) HasPermission(ctx context.Context, userID uuid.UUID, permissionSlug string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM permissions p
			INNER JOIN role_permissions rp ON p.id = rp.permission_id
			INNER JOIN user_roles ur ON rp.role_id = ur.role_id
			WHERE ur.user_id = $1 AND p.slug = $2
		)
	`

	var hasPermission bool
	err := r.pool.QueryRow(ctx, query, userID, permissionSlug).Scan(&hasPermission)
	if err != nil {
		return false, fmt.Errorf("failed to check permission: %w", err)
	}

	return hasPermission, nil
}

// GetUserPermissions retrieves all permissions for a user (through their roles)
func (r *postgresRepository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]Permission, error) {
	query := `
		SELECT DISTINCT p.id, p.slug, p.module, p.description, p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		INNER JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY p.module, p.slug
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var permission Permission
		if err := rows.Scan(&permission.ID, &permission.Slug, &permission.Module, &permission.Description, &permission.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

// GetAgentWorkload retrieves the workload of all agents (number of active chats)
func (r *postgresRepository) GetAgentWorkload(ctx context.Context) ([]WorkloadItem, error) {
	query := `
		SELECT
			u.id,
			u.email,
			u.full_name,
			u.status,
			COALESCE(COUNT(d.id), 0) as active_chats_count
		FROM users u
		LEFT JOIN dialogs d ON d.current_agent_id = u.id AND d.status IN ('open', 'pending')
		GROUP BY u.id, u.email, u.full_name, u.status
		ORDER BY active_chats_count DESC, u.full_name
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent workload: %w", err)
	}
	defer rows.Close()

	var workload []WorkloadItem
	for rows.Next() {
		var item WorkloadItem
		if err := rows.Scan(&item.UserID, &item.Email, &item.FullName, &item.Status, &item.ActiveChatsCount); err != nil {
			return nil, fmt.Errorf("failed to scan workload item: %w", err)
		}
		workload = append(workload, item)
	}

	return workload, nil
}