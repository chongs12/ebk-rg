package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Document struct {
    ID          uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
    Title       string    `gorm:"type:varchar(255);not null" json:"title"`
    Description string    `gorm:"type:text" json:"description"`
    FileName    string    `gorm:"type:varchar(255);not null" json:"file_name"`
    FilePath    string    `gorm:"type:varchar(500);not null" json:"file_path"`
    FileSize    int64     `gorm:"not null" json:"file_size"`
    FileType    string    `gorm:"type:varchar(50);not null" json:"file_type"`
    MimeType    string    `gorm:"type:varchar(100);not null" json:"mime_type"`
    Checksum    string    `gorm:"type:varchar(64);not null" json:"checksum"`
    Content     string    `gorm:"type:longtext" json:"content"`
    Language    string    `gorm:"type:varchar(10);default:'en'" json:"language"`
    WordCount   int       `gorm:"default:0" json:"word_count"`
    PageCount   int       `gorm:"default:0" json:"page_count"`
    IsPublic    bool      `gorm:"default:false" json:"is_public"`
    IsProcessed bool      `gorm:"default:false" json:"is_processed"`
    Visibility  string    `gorm:"type:varchar(20);default:'public'" json:"visibility"`
    DepartmentID string   `gorm:"type:varchar(100)" json:"department_id"`
    Status      string    `gorm:"type:varchar(20);default:'pending'" json:"status"`
    OwnerID     uuid.UUID `gorm:"type:char(36);not null" json:"owner_id"`
    Owner       User      `gorm:"foreignKey:OwnerID" json:"owner"`
    Tags        string    `gorm:"type:text" json:"tags"`
    Keywords    string    `gorm:"type:text" json:"keywords"`
    Category    string    `gorm:"type:varchar(100)" json:"category"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

func (d *Document) TableName() string {
	return "documents"
}

type DocumentChunk struct {
	ID         uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	DocumentID uuid.UUID `gorm:"type:char(36);not null;index" json:"document_id"`
	Document   Document  `gorm:"foreignKey:DocumentID" json:"document"`
	ChunkIndex int       `gorm:"not null" json:"chunk_index"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	StartPos   int       `gorm:"not null" json:"start_pos"`
	EndPos     int       `gorm:"not null" json:"end_pos"`
	WordCount  int       `gorm:"default:0" json:"word_count"`
	CharCount  int       `gorm:"default:0" json:"char_count"`
	Vector     string    `gorm:"type:text" json:"vector"`
	Embedding  []float64 `gorm:"-" json:"embedding"`
	Keywords   string    `gorm:"type:text" json:"keywords"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (dc *DocumentChunk) BeforeCreate(tx *gorm.DB) error {
	if dc.ID == uuid.Nil {
		dc.ID = uuid.New()
	}
	return nil
}

func (dc *DocumentChunk) TableName() string {
	return "document_chunks"
}

type DocumentPermission struct {
	ID         uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	DocumentID uuid.UUID `gorm:"type:char(36);not null;index" json:"document_id"`
	Document   Document  `gorm:"foreignKey:DocumentID" json:"document"`
	UserID     uuid.UUID `gorm:"type:char(36);not null;index" json:"user_id"`
	User       User      `gorm:"foreignKey:UserID" json:"user"`
	Permission string    `gorm:"type:varchar(20);not null" json:"permission"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (dp *DocumentPermission) BeforeCreate(tx *gorm.DB) error {
	if dp.ID == uuid.Nil {
		dp.ID = uuid.New()
	}
	return nil
}

func (dp *DocumentPermission) TableName() string {
	return "document_permissions"
}

type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "pending"
	DocumentStatusProcessing DocumentStatus = "processing"
	DocumentStatusCompleted  DocumentStatus = "completed"
	DocumentStatusFailed     DocumentStatus = "failed"
)

func (s DocumentStatus) String() string {
	return string(s)
}

func (s DocumentStatus) IsValid() bool {
	switch s {
	case DocumentStatusPending, DocumentStatusProcessing, DocumentStatusCompleted, DocumentStatusFailed:
		return true
	}
	return false
}

type DocumentPermissionType string

const (
	PermissionRead   DocumentPermissionType = "read"
	PermissionWrite  DocumentPermissionType = "write"
	PermissionDelete DocumentPermissionType = "delete"
	PermissionShare  DocumentPermissionType = "share"
)

func (p DocumentPermissionType) String() string {
	return string(p)
}

func (p DocumentPermissionType) IsValid() bool {
	switch p {
	case PermissionRead, PermissionWrite, PermissionDelete, PermissionShare:
		return true
	}
	return false
}

// TextChunk represents a text chunk with vector embedding
type TextChunk struct {
	ID         uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	DocumentID uuid.UUID `gorm:"type:char(36);not null;index" json:"document_id"`
	Document   Document  `gorm:"foreignKey:DocumentID" json:"document"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	ChunkIndex int       `gorm:"not null" json:"chunk_index"`
	StartPos   int       `gorm:"not null" json:"start_pos"`
	EndPos     int       `gorm:"not null" json:"end_pos"`
	WordCount  int       `gorm:"default:0" json:"word_count"`
	Embedding  []byte    `gorm:"type:blob" json:"embedding"` // JSON-encoded embedding vector
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (tc *TextChunk) BeforeCreate(tx *gorm.DB) error {
	if tc.ID == uuid.Nil {
		tc.ID = uuid.New()
	}
	return nil
}

func (tc *TextChunk) TableName() string {
	return "text_chunks"
}

type TextChunkWithDistance struct {
	TextChunk
	Distance float32 `json:"distance"`
}
