package vector

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/enterprise-knowledge-base/ekb/pkg/logger"
	"github.com/enterprise-knowledge-base/ekb/pkg/middleware"
)

type Handler struct {
	service *VectorService
}

func NewHandler(service *VectorService) *Handler {
	return &Handler{
		service: service,
	}
}

// ChunkDocumentRequest represents a request to chunk a document
type ChunkDocumentRequest struct {
	DocumentID string `json:"document_id" binding:"required"`
	Content    string `json:"content" binding:"required"`
	ChunkSize  int    `json:"chunk_size"`
}

// SearchSimilarRequest represents a request to search for similar chunks
type SearchSimilarRequest struct {
	Query string `json:"query" binding:"required"`
	Limit int    `json:"limit"`
}

// ChunkDocument chunks a document into text segments
func (h *Handler) ChunkDocument(c *gin.Context) {
	ctx := c.Request.Context()
	
	_, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	
	var req ChunkDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Chunk the document content
	chunks, err := h.service.ChunkText(ctx, req.DocumentID, req.Content, req.ChunkSize)
	if err != nil {
		logger.Error(ctx, "Failed to chunk document", "error", err.Error(), "document_id", req.DocumentID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to chunk document"})
		return
	}
	
	// Generate embeddings for chunks
	if err := h.service.GenerateEmbeddings(ctx, chunks); err != nil {
		logger.Error(ctx, "Failed to generate embeddings", "error", err.Error(), "document_id", req.DocumentID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate embeddings"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"document_id": req.DocumentID,
		"chunks":      chunks,
		"message":     "document chunked and embedded successfully",
	})
}

// SearchSimilar searches for similar text chunks
func (h *Handler) SearchSimilar(c *gin.Context) {
	ctx := c.Request.Context()
	
	_, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	
	var req SearchSimilarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Search for similar chunks
	chunks, err := h.service.SearchSimilarChunks(ctx, req.Query, req.Limit)
	if err != nil {
		logger.Error(ctx, "Failed to search similar chunks", "error", err.Error(), "query", req.Query)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search similar chunks"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"query":   req.Query,
		"chunks":  chunks,
		"count":   len(chunks),
	})
}

// GetDocumentChunks retrieves all chunks for a document
func (h *Handler) GetDocumentChunks(c *gin.Context) {
	ctx := c.Request.Context()
	
	_, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	
	documentID := c.Param("documentId")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document ID is required"})
		return
	}
	
	// Get document chunks
	chunks, err := h.service.GetDocumentChunks(ctx, documentID)
	if err != nil {
		logger.Error(ctx, "Failed to get document chunks", "error", err.Error(), "document_id", documentID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get document chunks"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"document_id": documentID,
		"chunks":      chunks,
		"count":       len(chunks),
	})
}

// DeleteDocumentChunks removes all chunks for a document
func (h *Handler) DeleteDocumentChunks(c *gin.Context) {
	ctx := c.Request.Context()
	
	_, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	
	documentID := c.Param("documentId")
	if documentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document ID is required"})
		return
	}
	
	// Delete document chunks
	if err := h.service.DeleteDocumentChunks(ctx, documentID); err != nil {
		logger.Error(ctx, "Failed to delete document chunks", "error", err.Error(), "document_id", documentID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete document chunks"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"document_id": documentID,
		"message":     "document chunks deleted successfully",
	})
}

// SetupRoutes configures the vector service routes
func (h *Handler) SetupRoutes(router *gin.Engine, authMiddleware *middleware.AuthMiddleware) {
	vectors := router.Group("/api/v1/vectors")
	vectors.Use(authMiddleware.RequireAuth())
	
	// Vector operations
	vectors.POST("/chunk", h.ChunkDocument)
	vectors.POST("/search", h.SearchSimilar)
	
	// Document-specific vector operations
	vectors.GET("/documents/:documentId/chunks", h.GetDocumentChunks)
	vectors.DELETE("/documents/:documentId/chunks", h.DeleteDocumentChunks)
}