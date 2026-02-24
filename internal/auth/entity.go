package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenPair represents an access/refresh token pair
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// AccessClaims represents JWT access token claims
type AccessClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	DeviceID string    `json:"device_id"`
	Roles    []string  `json:"roles"`
	jwt.RegisteredClaims
}

// RefreshClaims represents JWT refresh token claims
type RefreshClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	DeviceID string    `json:"device_id"`
	TokenID  uuid.UUID `json:"token_id"`
	jwt.RegisteredClaims
}

// RefreshTokenRecord represents a refresh token stored in the database
type RefreshTokenRecord struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	TokenHash  string     `json:"-" db:"token_hash"`
	DeviceID   string     `json:"device_id" db:"device_id"`
	DeviceName string     `json:"device_name" db:"device_name"`
	IPAddress  string     `json:"ip_address" db:"ip_address"`
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// DeviceSession represents an active device session for a user
type DeviceSession struct {
	DeviceID   string    `json:"device_id" db:"device_id"`
	DeviceName string    `json:"device_name" db:"device_name"`
	IPAddress  string    `json:"ip_address" db:"ip_address"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=6"`
	DeviceID   string `json:"device_id" binding:"required"`
	DeviceName string `json:"device_name" binding:"required"`
}

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=8"`
	FullName   string `json:"full_name" binding:"required"`
	DeviceID   string `json:"device_id" binding:"required"`
	DeviceName string `json:"device_name" binding:"required"`
}

// RefreshRequest represents the token refresh request body
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	DeviceID     string `json:"device_id" binding:"required"`
}

// AuthUser represents a user with password hash for authentication
type AuthUser struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	FullName     string    `json:"full_name" db:"full_name"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Status       string    `json:"status" db:"status"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}
