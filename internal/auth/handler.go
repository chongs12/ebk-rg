package auth

import (
	"net/http"

	"github.com/chongs12/enterprise-knowledge-base/pkg/logger"
	"github.com/chongs12/enterprise-knowledge-base/pkg/middleware"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service      *AuthService
	authMiddleware *middleware.AuthMiddleware
}

func NewAuthHandler(service *AuthService, authMiddleware *middleware.AuthMiddleware) *AuthHandler {
	return &AuthHandler{
		service:        service,
		authMiddleware: authMiddleware,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	resp, err := h.service.Register(c.Request.Context(), &req)
	if err != nil {
		logger.WithError(err).Error("Registration failed")
		
		statusCode := http.StatusInternalServerError
		if err.Error() == "username or email already exists" {
			statusCode = http.StatusConflict
		} else if err.Error() == "invalid email format" {
			statusCode = http.StatusBadRequest
		}
		
		c.JSON(statusCode, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"data":    resp,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	resp, err := h.service.Login(c.Request.Context(), &req)
	if err != nil {
		logger.WithError(err).Error("Login failed")
		
		statusCode := http.StatusUnauthorized
		if err.Error() == "user account is deactivated" {
			statusCode = http.StatusForbidden
		}
		
		c.JSON(statusCode, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"data":    resp,
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	resp, err := h.service.RefreshToken(c.Request.Context(), &req)
	if err != nil {
		logger.WithError(err).Error("Token refresh failed")
		
		statusCode := http.StatusUnauthorized
		if err.Error() == "user not found" || err.Error() == "user account is deactivated" {
			statusCode = http.StatusForbidden
		}
		
		c.JSON(statusCode, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Token refreshed successfully",
		"data":    resp,
	})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	user, err := h.service.GetUserByID(c.Request.Context(), userID.(string))
	if err != nil {
		logger.WithError(err).Error("Failed to get user profile")
		
		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}
		
		c.JSON(statusCode, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile retrieved successfully",
		"data":    user,
	})
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	allowedFields := []string{"first_name", "last_name", "avatar", "department"}
	filteredUpdates := make(map[string]interface{})
	for _, field := range allowedFields {
		if value, ok := updates[field]; ok {
			filteredUpdates[field] = value
		}
	}

	if len(filteredUpdates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No valid fields to update",
		})
		return
	}

	user, err := h.service.UpdateUser(c.Request.Context(), userID.(string), filteredUpdates)
	if err != nil {
		logger.WithError(err).Error("Failed to update user profile")
		
		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}
		
		c.JSON(statusCode, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
		"data":    user,
	})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	updates := map[string]interface{}{
		"password": req.NewPassword,
	}

	user, err := h.service.UpdateUser(c.Request.Context(), userID.(string), updates)
	if err != nil {
		logger.WithError(err).Error("Failed to change password")
		
		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}
		
		c.JSON(statusCode, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password changed successfully",
		"data":    user,
	})
}

func (h *AuthHandler) DeactivateAccount(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	if err := h.service.DeactivateUser(c.Request.Context(), userID.(string)); err != nil {
		logger.WithError(err).Error("Failed to deactivate user account")
		
		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}
		
		c.JSON(statusCode, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Account deactivated successfully",
	})
}

func (h *AuthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"service": "auth",
		"timestamp": gin.H{
			"unix":  gin.H{"$date": "2024-01-01T00:00:00Z"},
		},
	})
}