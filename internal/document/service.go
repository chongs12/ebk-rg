package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chongs12/enterprise-knowledge-base/internal/common/models"
	"github.com/chongs12/enterprise-knowledge-base/pkg/database"
	"github.com/chongs12/enterprise-knowledge-base/pkg/logger"
	"github.com/chongs12/enterprise-knowledge-base/pkg/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DocumentService struct {
	db           *database.Database
	uploadPath   string
	maxFileSize  int64
	allowedTypes []string
	gatewayBase  string
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
	AuthToken   string
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

// NewDocumentService 创建文档服务
// 参数：
// - db：数据库连接
// - uploadPath：上传目录
// - maxFileSize：文件大小上限
// - allowedTypes：允许的扩展名列表
// - gatewayBase：网关基础地址，用于触发向量化
// 返回：
// - *DocumentService 实例
func NewDocumentService(db *database.Database, uploadPath string, maxFileSize int64, allowedTypes []string, gatewayBase string) *DocumentService {
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		logger.Fatalf("Failed to create upload directory: %v", err)
	}

	return &DocumentService{
		db:           db,
		uploadPath:   uploadPath,
		maxFileSize:  maxFileSize,
		allowedTypes: allowedTypes,
		gatewayBase:  strings.TrimRight(gatewayBase, "/"),
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

	if err = os.WriteFile(filePath, content, 0644); err != nil {
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

	// 触发异步向量化流水线（通过 Gateway 调用 Vector 服务）
	go func(token string) {
		if err := s.TriggerVectorization(context.Background(), doc, token); err != nil {
			logger.WithError(err).Warn("Vectorization trigger failed")
		}
	}(req.AuthToken)

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
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("failed to read PDF file: %w", err)
	}
	text := utils.NormalizeText(string(content))
	wordCount := len(strings.Fields(text))
	return text, wordCount, 1, "en", nil
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

// TriggerVectorization 触发向量化（异步）
// 用途：
// - 调用 Gateway 的 `/api/v1/vectors/chunk` 接口，将文档内容分块并生成向量
// 参数：
// - ctx：上下文
// - doc：文档对象（需包含 Content）
// - authToken：鉴权令牌（可选，若为空表示由网关放行内部调用策略）
// 返回：
// - error：失败时返回错误
func (s *DocumentService) TriggerVectorization(ctx context.Context, doc *models.Document, authToken string) error {
	if s.gatewayBase == "" || doc == nil {
		return nil
	}
	payload := map[string]interface{}{
		"document_id": doc.ID.String(),
		"content":     doc.Content,
		"chunk_size":  200,
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.gatewayBase+"/api/v1/vectors/chunk", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(authToken) != "" {
		req.Header.Set("Authorization", authToken)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vectorization upstream status=%d", resp.StatusCode)
	}
	return nil
}
