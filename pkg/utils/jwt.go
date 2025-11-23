package utils

import (
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTClaims struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type JWTTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type JWTManager struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	issuer        string
}

func NewJWTManager(secret string, accessExpiry, refreshExpiry time.Duration, issuer string) *JWTManager {
	return &JWTManager{
		secret:        []byte(secret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
		issuer:        issuer,
	}
}

func (j *JWTManager) GenerateTokens(userID, email, role, username string) (*JWTTokens, error) {
	accessToken, err := j.generateToken(userID, email, role, username, j.accessExpiry, "access")
	if err != nil {
		return nil, err
	}

	refreshToken, err := j.generateToken(userID, email, role, username, j.refreshExpiry, "refresh")
	if err != nil {
		return nil, err
	}

	return &JWTTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(j.accessExpiry.Seconds()),
	}, nil
}

func (j *JWTManager) generateToken(userID, email, role, username string, expiry time.Duration, tokenType string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:   userID,
		Email:    email,
		Role:     role,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   tokenType,
			Audience:  jwt.ClaimStrings{"enterprise-knowledge-base"},
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func (j *JWTManager) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return j.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func (j *JWTManager) RefreshToken(refreshToken string) (*JWTTokens, error) {
	claims, err := j.ValidateToken(refreshToken)
	if err != nil {
		return nil, err
	}

	if claims.Subject != "refresh" {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return j.GenerateTokens(claims.UserID, claims.Email, claims.Role, claims.Username)
}

func ExtractTokenFromHeader(authHeader string) string {
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}