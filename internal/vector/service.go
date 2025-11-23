package vector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chongs12/enterprise-knowledge-base/internal/common/models"
	"github.com/chongs12/enterprise-knowledge-base/pkg/database"
	"github.com/chongs12/enterprise-knowledge-base/pkg/logger"
	"github.com/google/uuid"
)

type VectorService struct {
	db *database.Database
}

func NewVectorService(db *database.Database) *VectorService {
	return &VectorService{
		db: db,
	}
}

// TextChunk represents a chunk of text with metadata
type TextChunk struct {
	ID         string  `json:"id"`
	DocumentID string  `json:"document_id"`
	Content    string  `json:"content"`
	ChunkIndex int     `json:"chunk_index"`
	StartPos   int     `json:"start_pos"`
	EndPos     int     `json:"end_pos"`
	WordCount  int     `json:"word_count"`
	Embedding  []float64 `json:"embedding,omitempty"`
}

// ChunkText breaks down document content into manageable chunks
func (s *VectorService) ChunkText(ctx context.Context, documentID string, content string, chunkSize int) ([]*TextChunk, error) {
	if chunkSize <= 0 {
		chunkSize = 500 // Default chunk size (words)
	}

	words := strings.Fields(content)
	var chunks []*TextChunk
	
	for i := 0; i < len(words); i += chunkSize {
		end := i + chunkSize
		if end > len(words) {
			end = len(words)
		}
		
		chunkContent := strings.Join(words[i:end], " ")
		chunk := &TextChunk{
			ID:         uuid.New().String(),
			DocumentID: documentID,
			Content:    chunkContent,
			ChunkIndex: len(chunks),
			StartPos:   i,
			EndPos:     end,
			WordCount:  end - i,
		}
		
		chunks = append(chunks, chunk)
	}
	
	logger.Info(ctx, "Text chunking completed", "document_id", documentID, "chunks", len(chunks))
	return chunks, nil
}

// GenerateEmbeddings creates vector embeddings for text chunks
func (s *VectorService) GenerateEmbeddings(ctx context.Context, chunks []*TextChunk) error {
	// For now, we'll use a simple TF-IDF based embedding
	// In production, you'd integrate with an AI service like OpenAI, Hugging Face, etc.
	
	for _, chunk := range chunks {
		// Generate a simple embedding (in production, replace with actual AI service)
		embedding := s.generateSimpleEmbedding(chunk.Content)
		chunk.Embedding = embedding
		
		// Store chunk in database
		if err := s.storeChunk(ctx, chunk); err != nil {
			logger.Error(ctx, "Failed to store chunk", "chunk_id", chunk.ID, "error", err.Error())
			return fmt.Errorf("failed to store chunk: %w", err)
		}
	}
	
	logger.Info(ctx, "Embeddings generated and stored", "chunks", len(chunks))
	return nil
}

// generateSimpleEmbedding creates a simple embedding for demonstration
// In production, this would call an AI service
func (s *VectorService) generateSimpleEmbedding(text string) []float64 {
	// Simple word-based embedding for demonstration
	// In production, integrate with OpenAI, Hugging Face, or similar service
	words := strings.Fields(strings.ToLower(text))
	embedding := make([]float64, 384) // 384-dimensional embedding
	
	// Simple hash-based embedding for demonstration
	for i, word := range words {
		if i >= 100 { // Limit to first 100 words
			break
		}
		// Simple hash to create embedding values
		for j := 0; j < len(embedding); j++ {
			embedding[j] += float64(len(word)*i + j*int(word[0])) / 1000.0
		}
	}
	
	// Normalize embedding
	magnitude := 0.0
	for _, val := range embedding {
		magnitude += val * val
	}
	magnitude = sqrt(magnitude)
	
	if magnitude > 0 {
		for i := range embedding {
			embedding[i] /= magnitude
		}
	}
	
	return embedding
}

// sqrt is a simple square root function
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// storeChunk saves a text chunk to the database
func (s *VectorService) storeChunk(ctx context.Context, chunk *TextChunk) error {
	embeddingJSON, err := json.Marshal(chunk.Embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}
	
	docUUID, err := uuid.Parse(chunk.DocumentID)
	if err != nil {
		return fmt.Errorf("invalid document ID: %w", err)
	}
	
	dbChunk := &models.TextChunk{
		ID:          uuid.MustParse(chunk.ID),
		DocumentID:  docUUID,
		Content:     chunk.Content,
		ChunkIndex:  chunk.ChunkIndex,
		StartPos:    chunk.StartPos,
		EndPos:      chunk.EndPos,
		WordCount:   chunk.WordCount,
		Embedding:   embeddingJSON,
	}
	
	if err := s.db.Create(dbChunk).Error; err != nil {
		return fmt.Errorf("failed to create chunk: %w", err)
	}
	
	return nil
}

// SearchSimilarChunks finds chunks similar to a query
func (s *VectorService) SearchSimilarChunks(ctx context.Context, query string, limit int) ([]*TextChunk, error) {
	if limit <= 0 {
		limit = 10
	}
	
	// Generate embedding for query
	queryEmbedding := s.generateSimpleEmbedding(query)
	
	// Get all chunks from database
	var dbChunks []models.TextChunk
	if err := s.db.Find(&dbChunks).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch chunks: %w", err)
	}
	
	// Calculate similarities and find top matches
	var similarities []struct {
		chunk     *TextChunk
		similarity float64
	}
	
	for _, dbChunk := range dbChunks {
		var embedding []float64
		if err := json.Unmarshal(dbChunk.Embedding, &embedding); err != nil {
			logger.Warn(ctx, "Failed to unmarshal embedding", "chunk_id", dbChunk.ID.String(), "error", err.Error())
			continue
		}
		
		similarity := cosineSimilarity(queryEmbedding, embedding)
		
		chunk := &TextChunk{
			ID:         dbChunk.ID.String(),
			DocumentID: dbChunk.DocumentID.String(),
			Content:    dbChunk.Content,
			ChunkIndex: dbChunk.ChunkIndex,
			StartPos:   dbChunk.StartPos,
			EndPos:     dbChunk.EndPos,
			WordCount:  dbChunk.WordCount,
			Embedding:  embedding,
		}
		
		similarities = append(similarities, struct {
			chunk     *TextChunk
			similarity float64
		}{chunk, similarity})
	}
	
	// Sort by similarity (descending)
	for i := 0; i < len(similarities); i++ {
		for j := i + 1; j < len(similarities); j++ {
			if similarities[j].similarity > similarities[i].similarity {
				similarities[i], similarities[j] = similarities[j], similarities[i]
			}
		}
	}
	
	// Return top matches
	var result []*TextChunk
	for i := 0; i < len(similarities) && i < limit; i++ {
		result = append(result, similarities[i].chunk)
	}
	
	logger.Info(ctx, "Similarity search completed", "query", query, "matches", len(result))
	return result, nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	
	dotProduct := 0.0
	normA := 0.0
	normB := 0.0
	
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	if normA == 0 || normB == 0 {
		return 0
	}
	
	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// GetDocumentChunks retrieves all chunks for a document
func (s *VectorService) GetDocumentChunks(ctx context.Context, documentID string) ([]*TextChunk, error) {
	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document ID: %w", err)
	}
	
	var dbChunks []models.TextChunk
	if err := s.db.Where("document_id = ?", docUUID).Find(&dbChunks).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch document chunks: %w", err)
	}
	
	var chunks []*TextChunk
	for _, dbChunk := range dbChunks {
		var embedding []float64
		if err := json.Unmarshal(dbChunk.Embedding, &embedding); err != nil {
			logger.Warn(ctx, "Failed to unmarshal embedding", "chunk_id", dbChunk.ID.String(), "error", err.Error())
			embedding = nil
		}
		
		chunk := &TextChunk{
			ID:         dbChunk.ID.String(),
			DocumentID: dbChunk.DocumentID.String(),
			Content:    dbChunk.Content,
			ChunkIndex: dbChunk.ChunkIndex,
			StartPos:   dbChunk.StartPos,
			EndPos:     dbChunk.EndPos,
			WordCount:  dbChunk.WordCount,
			Embedding:  embedding,
		}
		chunks = append(chunks, chunk)
	}
	
	return chunks, nil
}

// DeleteDocumentChunks removes all chunks for a document
func (s *VectorService) DeleteDocumentChunks(ctx context.Context, documentID string) error {
	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return fmt.Errorf("invalid document ID: %w", err)
	}
	
	if err := s.db.Where("document_id = ?", docUUID).Delete(&models.TextChunk{}).Error; err != nil {
		return fmt.Errorf("failed to delete document chunks: %w", err)
	}
	
	logger.Info(ctx, "Document chunks deleted", "document_id", documentID)
	return nil
}