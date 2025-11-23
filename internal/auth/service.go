package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/chongs12/enterprise-knowledge-base/internal/common/models"
	"github.com/chongs12/enterprise-knowledge-base/pkg/database"
	"github.com/chongs12/enterprise-knowledge-base/pkg/logger"
	"github.com/chongs12/enterprise-knowledge-base/pkg/utils"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db         *database.Database
	jwtManager *utils.JWTManager
}

type RegisterRequest struct {
	Username  string `json:"username" binding:"required,min=3,max=50"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
	FirstName string `json:"first_name" binding:"max=50"`
	LastName  string `json:"last_name" binding:"max=50"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	User         *models.User `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func NewAuthService(db *database.Database, jwtSecret string, accessExpiry, refreshExpiry time.Duration, issuer string) *AuthService {
	return &AuthService{
		db:         db,
		jwtManager: utils.NewJWTManager(jwtSecret, accessExpiry, refreshExpiry, issuer),
	}
}

func (s *AuthService) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	logger.WithFields(logrus.Fields{
		"username": req.Username,
		"email":    req.Email,
	}).Info("Registering new user")

	if !utils.IsValidEmail(req.Email) {
		return nil, fmt.Errorf("invalid email format")
	}

	var existingUser models.User
	err := s.db.WithContext(ctx).Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error
	if err == nil {
		return nil, fmt.Errorf("username or email already exists")
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("database error: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Username:  req.Username,
		Email:     req.Email,
		Password:  string(hashedPassword),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      models.RoleUser.String(),
		IsActive:  true,
	}

	if err = s.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	tokens, err := s.jwtManager.GenerateTokens(user.ID.String(), user.Email, user.Role, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	user.Password = ""

	logger.WithFields(logrus.Fields{
		"user_id": user.ID,
	}).Info("User registered successfully")

	return &AuthResponse{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	logger.WithFields(logrus.Fields{
		"username": req.Username,
	}).Info("User login attempt")

	var user models.User
	err := s.db.WithContext(ctx).Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invalid username or password")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user account is deactivated")
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	now := time.Now()
	user.LastLogin = &now
	if err = s.db.WithContext(ctx).Save(&user).Error; err != nil {
		logger.WithError(err).Error("Failed to update last login")
	}

	tokens, err := s.jwtManager.GenerateTokens(user.ID.String(), user.Email, user.Role, user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	user.Password = ""

	logger.WithFields(logrus.Fields{
		"user_id": user.ID,
	}).Info("User logged in successfully")

	return &AuthResponse{
		User:         &user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, req *RefreshTokenRequest) (*AuthResponse, error) {
	logger.Info("Refreshing access token")

	tokens, err := s.jwtManager.RefreshToken(req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	claims, err := s.jwtManager.ValidateToken(tokens.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to validate new token: %w", err)
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID in token: %w", err)
	}

	var user models.User
	err = s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user account is deactivated")
	}

	user.Password = ""

	logger.WithFields(logrus.Fields{
		"user_id": user.ID,
	}).Info("Token refreshed successfully")

	return &AuthResponse{
		User:         &user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	}, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	user.Password = ""
	return &user, nil
}

func (s *AuthService) UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	if password, ok := updates["password"].(string); ok {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		updates["password"] = string(hashedPassword)
	}

	if err := s.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	user.Password = ""
	return &user, nil
}

func (s *AuthService) DeactivateUser(ctx context.Context, userID string) error {
	var user models.User
	err := s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user not found")
		}
		return fmt.Errorf("database error: %w", err)
	}

	user.IsActive = false
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return fmt.Errorf("failed to deactivate user: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"user_id": userID,
	}).Info("User deactivated successfully")

	return nil
}
