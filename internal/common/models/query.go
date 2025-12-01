package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Query struct {
	ID             uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	UserID         uuid.UUID `gorm:"type:char(36);not null;index" json:"user_id"`
	User           User      `gorm:"foreignKey:UserID" json:"user"`
	QueryText      string    `gorm:"type:text;not null" json:"query_text"`
	QueryType      string    `gorm:"type:varchar(50);default:'rag'" json:"query_type"`
	Response       string    `gorm:"type:longtext" json:"response"`
	IsAnswered     bool      `gorm:"default:false" json:"is_answered"`
	Confidence     float64   `gorm:"default:0" json:"confidence"`
	SourceCount    int       `gorm:"default:0" json:"source_count"`
	ProcessingTime int64     `gorm:"default:0" json:"processing_time"`
	Status         string    `gorm:"type:varchar(20);default:'pending'" json:"status"`
	Error          string    `gorm:"type:text" json:"error"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (q *Query) BeforeCreate(tx *gorm.DB) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return nil
}

func (q *Query) TableName() string {
	return "queries"
}

type QuerySource struct {
	ID             uuid.UUID `gorm:"type:char(36);primary_key" json:"id"`
	QueryID        uuid.UUID `gorm:"type:char(36);not null;index" json:"query_id"`
	Query          Query     `gorm:"foreignKey:QueryID" json:"query"`
	DocumentID     uuid.UUID `gorm:"type:char(36);not null;index" json:"document_id"`
	Document       Document  `gorm:"foreignKey:DocumentID" json:"document"`
	ChunkID        uuid.UUID `gorm:"type:char(36);not null;index" json:"chunk_id"`
	Chunk          TextChunk `gorm:"foreignKey:ChunkID" json:"chunk"`
	RelevanceScore float64   `gorm:"default:0" json:"relevance_score"`
	Excerpt        string    `gorm:"type:text" json:"excerpt"`
	Position       int       `gorm:"default:0" json:"position"`
	CreatedAt      time.Time `json:"created_at"`
}

func (qs *QuerySource) BeforeCreate(tx *gorm.DB) error {
	if qs.ID == uuid.Nil {
		qs.ID = uuid.New()
	}
	return nil
}

func (qs *QuerySource) TableName() string {
	return "query_sources"
}

type QueryType string

const (
	QueryTypeRAG    QueryType = "rag"
	QueryTypeSearch QueryType = "search"
	QueryTypeChat   QueryType = "chat"
)

func (t QueryType) String() string {
	return string(t)
}

func (t QueryType) IsValid() bool {
	switch t {
	case QueryTypeRAG, QueryTypeSearch, QueryTypeChat:
		return true
	}
	return false
}

type QueryStatus string

const (
	QueryStatusPending    QueryStatus = "pending"
	QueryStatusProcessing QueryStatus = "processing"
	QueryStatusCompleted  QueryStatus = "completed"
	QueryStatusFailed     QueryStatus = "failed"
)

func (s QueryStatus) String() string {
	return string(s)
}

func (s QueryStatus) IsValid() bool {
	switch s {
	case QueryStatusPending, QueryStatusProcessing, QueryStatusCompleted, QueryStatusFailed:
		return true
	}
	return false
}
