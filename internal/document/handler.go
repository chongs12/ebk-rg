package document

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/enterprise-knowledge-base/ekb/pkg/logger"
	"github.com/enterprise-knowledge-base/ekb/pkg/middleware"
)

type Handler struct {
	service *DocumentService
}

func NewHandler(service *DocumentService) *Handler {
	return &Handler{service: service}
}

// UploadDocument handles document upload
func (h *Handler) UploadDocument(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	// Validate file size (max 50MB)
	if header.Size > 50*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file size exceeds 50MB limit"})
		return
	}

	// Get title from form data
	title := c.PostForm("title")
	if title == "" {
		title = header.Filename
	}

	description := c.PostForm("description")

	ctx := c.Request.Context()
	uploadReq := &UploadRequest{
		File:        file,
		Header:      header,
		Title:       title,
		Description: description,
		UserID:      userID.(string),
	}
	doc, err := h.service.UploadDocument(ctx, uploadReq)
	if err != nil {
		logger.Error(ctx, "Failed to upload document", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload document"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "document uploaded successfully",
		"document": doc,
	})
}

// GetDocument retrieves a document by ID
func (h *Handler) GetDocument(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document ID is required"})
		return
	}

	ctx := c.Request.Context()
	doc, err := h.service.GetDocument(ctx, documentID, userID.(string))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		logger.Error(ctx, "Failed to get document", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get document"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"document": doc})
}

// ListDocuments retrieves documents with pagination
func (h *Handler) ListDocuments(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	ctx := c.Request.Context()
	filters := make(map[string]interface{})
	response, err := h.service.ListDocuments(ctx, userID.(string), page, pageSize, filters)
	if err != nil {
		logger.Error(ctx, "Failed to list documents", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list documents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": response.Documents,
		"total":     response.Total,
		"page":      page,
		"page_size": pageSize,
	})
}

// UpdateDocument updates document metadata
func (h *Handler) UpdateDocument(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document ID is required"})
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	ctx := c.Request.Context()
	updates := map[string]interface{}{
		"title":       req.Title,
		"description": req.Description,
	}
	doc, err := h.service.UpdateDocument(ctx, documentID, userID.(string), updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		logger.Error(ctx, "Failed to update document", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update document"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "document updated successfully",
		"document": doc,
	})
}

// DeleteDocument deletes a document
func (h *Handler) DeleteDocument(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document ID is required"})
		return
	}

	ctx := c.Request.Context()
	if err := h.service.DeleteDocument(ctx, documentID, userID.(string)); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		logger.Error(ctx, "Failed to delete document", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete document"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "document deleted successfully"})
}

// ShareDocument shares a document with other users
func (h *Handler) ShareDocument(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document ID is required"})
		return
	}

	var req struct {
		UserIDs    []string `json:"user_ids"`
		Permission string   `json:"permission"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Validate permission
	if req.Permission != "read" && req.Permission != "write" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "permission must be 'read' or 'write'"})
		return
	}

	ctx := c.Request.Context()
	// Share document with multiple users
	for _, targetUserID := range req.UserIDs {
		if err := h.service.ShareDocument(ctx, documentID, userID.(string), targetUserID, req.Permission); err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
				return
			}
			logger.Error(ctx, "Failed to share document", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to share document"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "document shared successfully"})
}

// GetDocumentPermissions retrieves document permissions
func (h *Handler) GetDocumentPermissions(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document ID is required"})
		return
	}

	ctx := c.Request.Context()
	permissions, err := h.service.GetDocumentPermissions(ctx, documentID, userID.(string))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		logger.Error(ctx, "Failed to get document permissions", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get document permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"permissions": permissions})
}

// DownloadDocument downloads a document file
func (h *Handler) DownloadDocument(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	documentID := c.Param("id")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document ID is required"})
		return
	}

	ctx := c.Request.Context()
	doc, fileData, err := h.service.DownloadDocument(ctx, documentID, userID.(string))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		logger.Error(ctx, "Failed to download document", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to download document"})
		return
	}

	// Set appropriate headers for file download
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", doc.FileName))
	c.Header("Content-Length", strconv.FormatInt(doc.FileSize, 10))

	c.Data(http.StatusOK, "application/octet-stream", fileData)
}

// SearchDocuments searches documents by title and content
func (h *Handler) SearchDocuments(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "search query is required"})
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	ctx := c.Request.Context()
	documents, err := h.service.SearchDocuments(ctx, userID.(string), query, page, pageSize)
	if err != nil {
		logger.Error(ctx, "Failed to search documents", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search documents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": documents.Documents,
		"total":     documents.Total,
		"page":      page,
		"page_size": pageSize,
	})
}

// SetupRoutes configures the document service routes
func (h *Handler) SetupRoutes(router *gin.Engine, authMiddleware *middleware.AuthMiddleware) {
	// Document routes (require authentication)
	documents := router.Group("/api/v1/documents")
	documents.Use(authMiddleware.RequireAuth())
	{
		documents.POST("", h.UploadDocument)
		documents.GET("", h.ListDocuments)
		documents.GET("/search", h.SearchDocuments)
		documents.GET("/:id", h.GetDocument)
		documents.PUT("/:id", h.UpdateDocument)
		documents.DELETE("/:id", h.DeleteDocument)
		documents.GET("/:id/download", h.DownloadDocument)
		documents.POST("/:id/share", h.ShareDocument)
		documents.GET("/:id/permissions", h.GetDocumentPermissions)
	}
}