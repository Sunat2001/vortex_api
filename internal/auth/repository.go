package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the interface for auth-related database operations
type Repository interface {
	// User operations
	CreateUser(ctx context.Context, user *AuthUser) error
	GetUserByEmail(ctx context.Context, email string) (*AuthUser, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*AuthUser, error)

	// Refresh token operations
	StoreRefreshToken(ctx context.Context, token *RefreshTokenRecord) error
	GetRefreshToken(ctx context.Context, tokenID uuid.UUID) (*RefreshTokenRecord, error)
	RevokeRefreshToken(ctx context.Context, tokenID uuid.UUID) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
	RevokeUserDeviceTokens(ctx context.Context, userID uuid.UUID, deviceID string) error

	// Session operations
	GetUserSessions(ctx context.Context, userID uuid.UUID) ([]DeviceSession, error)

	// Role operations (for token claims)
	GetUserRoleSlugs(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// postgresRepository implements Repository interface using pgx
type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL repository
func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

// CreateUser creates a new user with password hash
func (r *postgresRepository) CreateUser(ctx context.Context, user *AuthUser) error {
	query := `
		INSERT INTO users (id, email, full_name, password_hash, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query, user.ID, user.Email, user.FullName, user.PasswordHash, user.Status, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByEmail retrieves a user by email including password hash
func (r *postgresRepository) GetUserByEmail(ctx context.Context, email string) (*AuthUser, error) {
	query := `SELECT id, email, full_name, password_hash, status, created_at FROM users WHERE email = $1`

	var user AuthUser
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.FullName, &user.PasswordHash, &user.Status, &user.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID including password hash
func (r *postgresRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*AuthUser, error) {
	query := `SELECT id, email, full_name, password_hash, status, created_at FROM users WHERE id = $1`

	var user AuthUser
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.FullName, &user.PasswordHash, &user.Status, &user.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// StoreRefreshToken stores a refresh token record
func (r *postgresRepository) StoreRefreshToken(ctx context.Context, token *RefreshTokenRecord) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, device_id, device_name, ip_address, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(ctx, query,
		token.ID, token.UserID, token.TokenHash, token.DeviceID, token.DeviceName, token.IPAddress, token.ExpiresAt, token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}
	return nil
}

// GetRefreshToken retrieves a refresh token by ID
func (r *postgresRepository) GetRefreshToken(ctx context.Context, tokenID uuid.UUID) (*RefreshTokenRecord, error) {
	query := `
		SELECT id, user_id, token_hash, device_id, device_name, ip_address, expires_at, created_at, revoked_at
		FROM refresh_tokens WHERE id = $1
	`

	var token RefreshTokenRecord
	err := r.pool.QueryRow(ctx, query, tokenID).Scan(
		&token.ID, &token.UserID, &token.TokenHash, &token.DeviceID, &token.DeviceName,
		&token.IPAddress, &token.ExpiresAt, &token.CreatedAt, &token.RevokedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("refresh token not found")
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	return &token, nil
}

// RevokeRefreshToken revokes a specific refresh token
func (r *postgresRepository) RevokeRefreshToken(ctx context.Context, tokenID uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked_at = $1 WHERE id = $2 AND revoked_at IS NULL`

	result, err := r.pool.Exec(ctx, query, time.Now(), tokenID)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("refresh token not found or already revoked")
	}

	return nil
}

// RevokeAllUserTokens revokes all refresh tokens for a user
func (r *postgresRepository) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`

	_, err := r.pool.Exec(ctx, query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all user tokens: %w", err)
	}

	return nil
}

// RevokeUserDeviceTokens revokes all refresh tokens for a user on a specific device
func (r *postgresRepository) RevokeUserDeviceTokens(ctx context.Context, userID uuid.UUID, deviceID string) error {
	query := `UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND device_id = $3 AND revoked_at IS NULL`

	_, err := r.pool.Exec(ctx, query, time.Now(), userID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to revoke user device tokens: %w", err)
	}

	return nil
}

// GetUserSessions retrieves all active sessions for a user
func (r *postgresRepository) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]DeviceSession, error) {
	query := `
		SELECT DISTINCT ON (device_id) device_id, device_name, ip_address, created_at
		FROM refresh_tokens
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
		ORDER BY device_id, created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	defer rows.Close()

	var sessions []DeviceSession
	for rows.Next() {
		var session DeviceSession
		if err := rows.Scan(&session.DeviceID, &session.DeviceName, &session.IPAddress, &session.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetUserRoleSlugs retrieves role slugs for a user (used for JWT claims)
func (r *postgresRepository) GetUserRoleSlugs(ctx context.Context, userID uuid.UUID) ([]string, error) {
	query := `
		SELECT r.slug
		FROM roles r
		INNER JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.slug
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role slugs: %w", err)
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("failed to scan role slug: %w", err)
		}
		slugs = append(slugs, slug)
	}

	return slugs, nil
}
