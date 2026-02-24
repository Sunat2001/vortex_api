package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/voronka/backend/internal/shared/config"
)

// Usecase defines business logic for auth operations
type Usecase interface {
	Login(ctx context.Context, req *LoginRequest, ipAddress string) (*TokenPair, error)
	Register(ctx context.Context, req *RegisterRequest, ipAddress string) (*TokenPair, error)
	RefreshTokens(ctx context.Context, req *RefreshRequest, ipAddress string) (*TokenPair, error)
	Logout(ctx context.Context, userID uuid.UUID, deviceID string) error
	LogoutAll(ctx context.Context, userID uuid.UUID) error
	GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]DeviceSession, error)
	RevokeSession(ctx context.Context, userID uuid.UUID, deviceID string) error
	ValidateAccessToken(tokenString string) (*AccessClaims, error)
}

// usecase implements Usecase interface
type usecase struct {
	repo   Repository
	cfg    *config.JWTConfig
	logger *zap.Logger
}

// NewUsecase creates a new auth usecase
func NewUsecase(repo Repository, cfg *config.JWTConfig, logger *zap.Logger) Usecase {
	return &usecase{
		repo:   repo,
		cfg:    cfg,
		logger: logger,
	}
}

// Login authenticates a user and returns a token pair
func (u *usecase) Login(ctx context.Context, req *LoginRequest, ipAddress string) (*TokenPair, error) {
	// Get user by email
	user, err := u.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Revoke existing tokens for this device
	_ = u.repo.RevokeUserDeviceTokens(ctx, user.ID, req.DeviceID)

	// Generate token pair
	tokenPair, err := u.generateTokenPair(ctx, user.ID, req.DeviceID, req.DeviceName, ipAddress)
	if err != nil {
		u.logger.Error("failed to generate token pair", zap.Error(err))
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	u.logger.Info("user logged in",
		zap.String("user_id", user.ID.String()),
		zap.String("device_id", req.DeviceID),
	)

	return tokenPair, nil
}

// Register creates a new user and returns a token pair
func (u *usecase) Register(ctx context.Context, req *RegisterRequest, ipAddress string) (*TokenPair, error) {
	// Check if user already exists
	existingUser, _ := u.repo.GetUserByEmail(ctx, req.Email)
	if existingUser != nil {
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &AuthUser{
		ID:           uuid.New(),
		Email:        req.Email,
		FullName:     req.FullName,
		PasswordHash: string(passwordHash),
		Status:       "offline",
		CreatedAt:    time.Now(),
	}

	if err := u.repo.CreateUser(ctx, user); err != nil {
		u.logger.Error("failed to create user", zap.Error(err))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate token pair
	tokenPair, err := u.generateTokenPair(ctx, user.ID, req.DeviceID, req.DeviceName, ipAddress)
	if err != nil {
		u.logger.Error("failed to generate token pair", zap.Error(err))
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	u.logger.Info("user registered",
		zap.String("user_id", user.ID.String()),
		zap.String("email", user.Email),
	)

	return tokenPair, nil
}

// RefreshTokens validates the refresh token, rotates it, and returns a new pair
func (u *usecase) RefreshTokens(ctx context.Context, req *RefreshRequest, ipAddress string) (*TokenPair, error) {
	// Parse refresh token claims
	claims, err := u.parseRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Verify device_id matches
	if claims.DeviceID != req.DeviceID {
		u.logger.Warn("device_id mismatch on token refresh",
			zap.String("user_id", claims.UserID.String()),
			zap.String("expected_device", claims.DeviceID),
			zap.String("actual_device", req.DeviceID),
		)
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Get stored token record
	storedToken, err := u.repo.GetRefreshToken(ctx, claims.TokenID)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Check if token is revoked
	if storedToken.RevokedAt != nil {
		// Possible token reuse attack - revoke all tokens for this user
		u.logger.Warn("refresh token reuse detected, revoking all user tokens",
			zap.String("user_id", claims.UserID.String()),
			zap.String("token_id", claims.TokenID.String()),
		)
		_ = u.repo.RevokeAllUserTokens(ctx, claims.UserID)
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Check if token is expired
	if storedToken.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("refresh token expired")
	}

	// Verify token hash
	tokenHash := hashToken(req.RefreshToken)
	if tokenHash != storedToken.TokenHash {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Revoke old refresh token (rotation)
	if err := u.repo.RevokeRefreshToken(ctx, claims.TokenID); err != nil {
		u.logger.Error("failed to revoke old refresh token", zap.Error(err))
		return nil, fmt.Errorf("failed to refresh tokens: %w", err)
	}

	// Generate new token pair
	tokenPair, err := u.generateTokenPair(ctx, claims.UserID, req.DeviceID, storedToken.DeviceName, ipAddress)
	if err != nil {
		u.logger.Error("failed to generate token pair", zap.Error(err))
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	u.logger.Info("tokens refreshed",
		zap.String("user_id", claims.UserID.String()),
		zap.String("device_id", req.DeviceID),
	)

	return tokenPair, nil
}

// Logout revokes all tokens for a specific device
func (u *usecase) Logout(ctx context.Context, userID uuid.UUID, deviceID string) error {
	if err := u.repo.RevokeUserDeviceTokens(ctx, userID, deviceID); err != nil {
		u.logger.Error("failed to logout",
			zap.String("user_id", userID.String()),
			zap.String("device_id", deviceID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to logout: %w", err)
	}

	u.logger.Info("user logged out",
		zap.String("user_id", userID.String()),
		zap.String("device_id", deviceID),
	)

	return nil
}

// LogoutAll revokes all tokens for a user
func (u *usecase) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	if err := u.repo.RevokeAllUserTokens(ctx, userID); err != nil {
		u.logger.Error("failed to logout all sessions",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to logout all sessions: %w", err)
	}

	u.logger.Info("user logged out from all sessions",
		zap.String("user_id", userID.String()),
	)

	return nil
}

// GetActiveSessions retrieves all active sessions for a user
func (u *usecase) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]DeviceSession, error) {
	sessions, err := u.repo.GetUserSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	return sessions, nil
}

// RevokeSession revokes all tokens for a specific device
func (u *usecase) RevokeSession(ctx context.Context, userID uuid.UUID, deviceID string) error {
	if err := u.repo.RevokeUserDeviceTokens(ctx, userID, deviceID); err != nil {
		u.logger.Error("failed to revoke session",
			zap.String("user_id", userID.String()),
			zap.String("device_id", deviceID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	u.logger.Info("session revoked",
		zap.String("user_id", userID.String()),
		zap.String("device_id", deviceID),
	)

	return nil
}

// ValidateAccessToken parses and validates an access token
func (u *usecase) ValidateAccessToken(tokenString string) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(u.cfg.AccessSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid access token: %w", err)
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid access token claims")
	}

	// Verify issuer
	if iss, _ := claims.GetIssuer(); iss != u.cfg.Issuer {
		return nil, fmt.Errorf("invalid token issuer")
	}

	return claims, nil
}

// generateTokenPair creates a new access/refresh token pair
func (u *usecase) generateTokenPair(ctx context.Context, userID uuid.UUID, deviceID, deviceName, ipAddress string) (*TokenPair, error) {
	// Get user roles for access token
	roles, err := u.repo.GetUserRoleSlugs(ctx, userID)
	if err != nil {
		u.logger.Warn("failed to get user roles, proceeding with empty roles",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		roles = []string{}
	}

	now := time.Now()
	accessExpiry := now.Add(u.cfg.AccessTTL)
	refreshExpiry := now.Add(u.cfg.RefreshTTL)
	tokenID := uuid.New()

	// Generate access token
	accessClaims := &AccessClaims{
		UserID:   userID,
		DeviceID: deviceID,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    u.cfg.Issuer,
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(u.cfg.AccessSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token
	refreshClaims := &RefreshClaims{
		UserID:   userID,
		DeviceID: deviceID,
		TokenID:  tokenID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    u.cfg.Issuer,
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(refreshExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(u.cfg.RefreshSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	// Store refresh token hash in database
	record := &RefreshTokenRecord{
		ID:         tokenID,
		UserID:     userID,
		TokenHash:  hashToken(refreshTokenString),
		DeviceID:   deviceID,
		DeviceName: deviceName,
		IPAddress:  ipAddress,
		ExpiresAt:  refreshExpiry,
		CreatedAt:  now,
	}

	if err := u.repo.StoreRefreshToken(ctx, record); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    accessExpiry.Unix(),
	}, nil
}

// parseRefreshToken parses and validates a refresh token
func (u *usecase) parseRefreshToken(tokenString string) (*RefreshClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(u.cfg.RefreshSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid refresh token claims")
	}

	if iss, _ := claims.GetIssuer(); iss != u.cfg.Issuer {
		return nil, fmt.Errorf("invalid token issuer")
	}

	return claims, nil
}

// hashToken creates a SHA-256 hash of a token string
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
