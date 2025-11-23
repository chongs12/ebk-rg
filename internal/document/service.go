package document

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/enterprise-knowledge-base/ekb/internal/common/models"
	"github.com/enterprise-knowledge-base/ekb/pkg/database"
	"github.com/enterprise-knowledge-base/ekb/pkg/logger"
	"github.com/enterprise-knowledge-base/ekb/pkg/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DocumentService struct {
	db           *database.Database
	uploadPath   string
	maxFileSize  int64
	allowedTypes []string
}

type UploadRequest struct {
	File        multipart.File
	Header      *multipart.FileHeader
	Title       string
	Description string
	IsPublic    bool
	Category    string
	Tags        []string
	UserID      string
}

type DocumentResponse struct {
	Document *models.Document `json:"document"`
	Message  string           `json:"message"`
}

type DocumentListResponse struct {
	Documents []*models.Document `json:"documents"`
	Total     int64              `json:"total"`
	Page      int                `json:"page"`
	PageSize  int                `json:"page_size"`
}

func NewDocumentService(db *database.Database, uploadPath string, maxFileSize int64, allowedTypes []string) *DocumentService {
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		logger.Fatalf("Failed to create upload directory: %v", err)
	}

	return &DocumentService{
		db:           db,
		uploadPath:   uploadPath,
		maxFileSize:  maxFileSize,
		allowedTypes: allowedTypes,
	}
}

func (s *DocumentService) UploadDocument(ctx context.Context, req *UploadRequest) (*DocumentResponse, error) {
	logger.WithFields(map[string]interface{}{
		"filename": req.Header.Filename,
		"size":     req.Header.Size,
		"user_id":  req.UserID,
	}).Info("Uploading document")

	if req.Header.Size > s.maxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", s.maxFileSize)
	}

	ext := strings.ToLower(utils.GetFileExtension(req.Header.Filename))
	if !utils.Contains(s.allowedTypes, ext) {
		return nil, fmt.Errorf("file type %s is not allowed. Allowed types: %v", ext, s.allowedTypes)
	}

	content, err := io.ReadAll(req.File)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	hash := sha256.Sum256(content)
	checksum := hex.EncodeToString(hash[:])

	var existingDoc models.Document
	err = s.db.WithContext(ctx).Where("checksum = ?", checksum).First(&existingDoc).Error
	if err == nil {
		return nil, fmt.Errorf("document with same content already exists")
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("database error: %w", err)
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	docID := uuid.New()
	filename := fmt.Sprintf("%s%s", docID.String(), ext)
	filePath := filepath.Join(s.uploadPath, filename)

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	doc := &models.Document{
		ID:          docID,
		Title:       req.Title,
		Description: req.Description,
		FileName:    req.Header.Filename,
		FilePath:    filePath,
		FileSize:    req.Header.Size,
		FileType:    ext,
		MimeType:    req.Header.Header.Get("Content-Type"),
		Checksum:    checksum,
		IsPublic:    req.IsPublic,
		OwnerID:     userID,
		Category:    req.Category,
		Tags:        strings.Join(req.Tags, ","),
		Status:      models.DocumentStatusPending.String(),
	}

	textContent, wordCount, pageCount, language, err := s.extractTextContent(filePath, ext)
	if err != nil {
		logger.WithError(err).Error("Failed to extract text content")
		doc.Status = models.DocumentStatusFailed.String()
	} else {
		doc.Content = textContent
		doc.WordCount = wordCount
		doc.PageCount = pageCount
		doc.Language = language
		doc.Status = models.DocumentStatusCompleted.String()
		doc.IsProcessed = true
	}

	keywords := utils.ExtractKeywords(textContent, 10)
	doc.Keywords = strings.Join(keywords, ",")

	if err := s.db.WithContext(ctx).Create(doc).Error; err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save document metadata: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"document_id": doc.ID,
		"status":      doc.Status,
	}).Info("Document uploaded successfully")

	return &DocumentResponse{
		Document: doc,
		Message:  "Document uploaded successfully",
	}, nil
}

func (s *DocumentService) extractTextContent(filePath string, fileType string) (string, int, int, string, error) {
	switch fileType {
	case ".pdf":
		return s.extractPDFContent(filePath)
	case ".txt":
		return s.extractPlainTextContent(filePath)
	default:
		return "", 0, 0, "unknown", fmt.Errorf("unsupported file type: %s", fileType)
	}
}

func (s *DocumentService) extractPDFContent(filePath string) (string, int, int, string, error) {
	// Open PDF file
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("failed to open PDF file: %w", err)
	}
	defer file.Close()

	// Extract text from PDF using simple approach
	// For now, we'll read the file as text and extract what we can
	// This is a basic implementation - for production, use a proper PDF library
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("failed to read PDF file: %w", err)
	}

	// Simple page counting - default to 1 for now
	totalPages := 1

	textContent := utils.NormalizeText(string(content))
	wordCount := len(strings.Fields(textContent))
	language := "en"

	return textContent, wordCount, totalPages, language, nil
}

func (s *DocumentService) extractPlainTextContent(filePath string) (string, int, int, string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("failed to read file: %w", err)
	}

	text := utils.NormalizeText(string(content))
	wordCount := len(strings.Fields(text))
	language := "en"

	return text, wordCount, 1, language, nil
}

func (s *DocumentService) GetDocument(ctx context.Context, documentID string, userID string) (*models.Document, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document ID: %w", err)
	}

	var doc models.Document
	err = s.db.WithContext(ctx).Where("id = ?", docUUID).First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("document not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	if !doc.IsPublic && doc.OwnerID != userUUID {
		var permission models.DocumentPermission
		err = s.db.WithContext(ctx).Where("document_id = ? AND user_id = ? AND permission = ?", 
			docUUID, userUUID, models.PermissionRead.String()).First(&permission).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("access denied: insufficient permissions")
			}
			return nil, fmt.Errorf("database error: %w", err)
		}
	}

	return &doc, nil
}

func (s *DocumentService) ListDocuments(ctx context.Context, userID string, page, pageSize int, filters map[string]interface{}) (*DocumentListResponse, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	offset := (page - 1) * pageSize

	query := s.db.WithContext(ctx).Model(&models.Document{})

	query = query.Where("owner_id = ? OR is_public = ?", userUUID, true)

	if category, ok := filters["category"].(string); ok && category != "" {
		query = query.Where("category = ?", category)
	}

	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}

	if search, ok := filters["search"].(string); ok && search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("title LIKE ? OR description LIKE ? OR keywords LIKE ?", 
			searchPattern, searchPattern, searchPattern)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count documents: %w", err)
	}

	var documents []*models.Document
	if err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&documents).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch documents: %w", err)
	}

	return &DocumentListResponse{
		Documents: documents,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	}, nil
}

func (s *DocumentService) UpdateDocument(ctx context.Context, documentID string, userID string, updates map[string]interface{}) (*models.Document, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document ID: %w", err)
	}

	var doc models.Document
	err = s.db.WithContext(ctx).Where("id = ? AND owner_id = ?", docUUID, userUUID).First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("document not found or access denied")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	allowedFields := []string{"title", "description", "category", "tags", "is_public"}
	filteredUpdates := make(map[string]interface{})
	for _, field := range allowedFields {
		if value, ok := updates[field]; ok {
			if field == "tags" {
				if tags, ok := value.([]string); ok {
					filteredUpdates[field] = strings.Join(tags, ",")
				}
			} else {
				filteredUpdates[field] = value
			}
		}
	}

	if len(filteredUpdates) == 0 {
		return nil, fmt.Errorf("no valid fields to update")
	}

	if err := s.db.WithContext(ctx).Model(&doc).Updates(filteredUpdates).Error; err != nil {
		return nil, fmt.Errorf("failed to update document: %w", err)
	}

	return &doc, nil
}

func (s *DocumentService) DeleteDocument(ctx context.Context, documentID string, userID string) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return fmt.Errorf("invalid document ID: %w", err)
	}

	var doc models.Document
	err = s.db.WithContext(ctx).Where("id = ? AND owner_id = ?", docUUID, userUUID).First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("document not found or access denied")
		}
		return fmt.Errorf("database error: %w", err)
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer tx.Rollback()

	if err := tx.Where("document_id = ?", docUUID).Delete(&models.DocumentChunk{}).Error; err != nil {
		return fmt.Errorf("failed to delete document chunks: %w", err)
	}

	if err := tx.Where("document_id = ?", docUUID).Delete(&models.DocumentPermission{}).Error; err != nil {
		return fmt.Errorf("failed to delete document permissions: %w", err)
	}

	if err := tx.Delete(&doc).Error; err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	if err := os.Remove(doc.FilePath); err != nil && !os.IsNotExist(err) {
		logger.WithError(err).Warn("Failed to remove file from disk")
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"document_id": doc.ID,
	}).Info("Document deleted successfully")

	return nil
}

func (s *DocumentService) ShareDocument(ctx context.Context, documentID string, ownerID string, userID string, permission string) error {
	ownerUUID, err := uuid.Parse(ownerID)
	if err != nil {
		return fmt.Errorf("invalid owner ID: %w", err)
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return fmt.Errorf("invalid document ID: %w", err)
	}

	var doc models.Document
	err = s.db.WithContext(ctx).Where("id = ? AND owner_id = ?", docUUID, ownerUUID).First(&doc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("document not found or access denied")
		}
		return fmt.Errorf("database error: %w", err)
	}

	if !models.DocumentPermissionType(permission).IsValid() {
		return fmt.Errorf("invalid permission type: %s", permission)
	}

	permissionRecord := &models.DocumentPermission{
		DocumentID: docUUID,
		UserID:     userUUID,
		Permission: permission,
	}

	err = s.db.WithContext(ctx).Where("document_id = ? AND user_id = ? AND permission = ?",
		docUUID, userUUID, permission).FirstOrCreate(permissionRecord).Error
	if err != nil {
		return fmt.Errorf("failed to create permission: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"document_id": documentID,
		"user_id":     userID,
		"permission":  permission,
	}).Info("Document shared successfully")

	return nil
}