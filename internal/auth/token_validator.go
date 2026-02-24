package auth

import "github.com/google/uuid"

// TokenValidatorAdapter adapts the auth Usecase to the middleware.TokenValidator interface
type TokenValidatorAdapter struct {
	usecase Usecase
}

// NewTokenValidator creates a new TokenValidatorAdapter
func NewTokenValidator(usecase Usecase) *TokenValidatorAdapter {
	return &TokenValidatorAdapter{usecase: usecase}
}

// ValidateAccessToken validates a JWT access token and returns claims
func (v *TokenValidatorAdapter) ValidateAccessToken(tokenString string) (uuid.UUID, string, []string, error) {
	claims, err := v.usecase.ValidateAccessToken(tokenString)
	if err != nil {
		return uuid.Nil, "", nil, err
	}

	return claims.UserID, claims.DeviceID, claims.Roles, nil
}
