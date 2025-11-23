package document

import (
	"context"
	"fmt"
	"os"

	"github.com/chongs12/enterprise-knowledge-base/internal/common/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *DocumentService) GetDocumentPermissions(ctx context.Context, documentID string, userID string) ([]*models.DocumentPermission, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document ID: %w", err)
	}

	// Check if user has access to the document
	hasAccess, err := s.hasDocumentAccess(ctx, docUUID, userUUID, "read")
	if err != nil {
		return nil, fmt.Errorf("failed to check access: %w", err)
	}
	if !hasAccess {
		return nil, gorm.ErrRecordNotFound
	}

	var permissions []*models.DocumentPermission
	err = s.db.WithContext(ctx).
		Where("document_id = ?", docUUID).
		Preload("User").
		Find(&permissions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	return permissions, nil
}

func (s *DocumentService) DownloadDocument(ctx context.Context, documentID string, userID string) (*models.Document, []byte, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid user ID: %w", err)
	}

	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid document ID: %w", err)
	}

	// Check if user has access to the document
	hasAccess, err := s.hasDocumentAccess(ctx, docUUID, userUUID, "read")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check access: %w", err)
	}
	if !hasAccess {
		return nil, nil, gorm.ErrRecordNotFound
	}

	var doc models.Document
	err = s.db.WithContext(ctx).Where("id = ?", docUUID).First(&doc).Error
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Read file content
	fileData, err := os.ReadFile(doc.FilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &doc, fileData, nil
}

func (s *DocumentService) SearchDocuments(ctx context.Context, userID string, query string, page, pageSize int) (*DocumentListResponse, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	offset := (page - 1) * pageSize

	var documents []*models.Document
	var total int64

	// Build query to search documents accessible by user
	db := s.db.WithContext(ctx).
		Table("documents d").
		Select("d.*").
		Joins("LEFT JOIN document_permissions dp ON d.id = dp.document_id AND dp.user_id = ?", userUUID).
		Where("d.owner_id = ? OR dp.permission IN (?) OR d.is_public = ?", userUUID, []string{"read", "write"}, true).
		Where("d.status = ?", models.DocumentStatusCompleted.String()).
		Where("(d.title LIKE ? OR d.content LIKE ? OR d.keywords LIKE ?)", "%"+query+"%", "%"+query+"%", "%"+query+"%").
		Group("d.id")

	// Get total count
	db.Count(&total)

	// Get paginated results
	err = db.Offset(offset).Limit(pageSize).
		Order("d.created_at DESC").
		Find(&documents).Error
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	return &DocumentListResponse{
		Documents: documents,
		Total:     total,
	}, nil
}

func (s *DocumentService) hasDocumentAccess(ctx context.Context, documentID, userID uuid.UUID, permission string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("documents d").
		Joins("LEFT JOIN document_permissions dp ON d.id = dp.document_id AND dp.user_id = ?", userID).
		Where("d.id = ? AND (d.owner_id = ? OR dp.permission = ? OR d.is_public = ?)",
			documentID, userID, permission, true).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}