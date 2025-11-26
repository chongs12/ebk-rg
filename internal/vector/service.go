package vector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chongs12/enterprise-knowledge-base/internal/common/models"
	"github.com/chongs12/enterprise-knowledge-base/pkg/database"
	"github.com/chongs12/enterprise-knowledge-base/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float64, error)
}
type Store interface {
	IndexChunks(ctx context.Context, chunks []*models.TextChunk) error
	Retrieve(ctx context.Context, query string, limit int, scoreThreshold float32) ([]Hit, error)
	DeleteByIDs(ctx context.Context, ids []string) error
	InsertChunks(ctx context.Context, chunks []*models.TextChunk, embeddings [][]float64) error
}

type VectorService struct {
	db    *database.Database
	embed Embedder
	store Store
	redis *redis.Client
}

// splitChineseText 按中文语义切分文本为 chunks
func splitChineseText(text string, maxChars int) []string {
	if strings.TrimSpace(text) == "" {
		return []string{}
	}

	// 按句子结束符分割：中文句号、感叹号、问号、分号、换行等
	sentenceRegex := regexp.MustCompile(`[。！？；;!?。\n\r]+`)
	sentences := sentenceRegex.Split(text, -1)

	var chunks []string
	var current strings.Builder
	currentLen := 0

	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if sent == "" {
			continue
		}

		sentRunes := []rune(sent)
		sentLen := len(sentRunes)

		// 极端情况：单句超长，强制切分
		if sentLen > maxChars {
			// 按 maxChars 切分
			for i := 0; i < sentLen; i += maxChars {
				end := i + maxChars
				if end > sentLen {
					end = sentLen
				}
				chunks = append(chunks, string(sentRunes[i:end]))
			}
			continue
		}

		// 如果加入当前句会超出限制，且当前 chunk 非空 → 提交
		if currentLen > 0 && currentLen+sentLen > maxChars {
			chunks = append(chunks, current.String())
			current.Reset()
			currentLen = 0
		}

		// 追加当前句
		if currentLen > 0 {
			current.WriteString(" ") // 可选：加空格或直接拼接
		}
		current.WriteString(sent)
		currentLen += sentLen
	}

	// 处理最后一块
	if currentLen > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}

func NewVectorService(db *database.Database, embed Embedder, store Store, redis *redis.Client) *VectorService {
	return &VectorService{db: db, embed: embed, store: store, redis: redis}
}

// TextChunk represents a chunk of text with metadata

// ChunkText breaks down document content into manageable chunks
func (s *VectorService) ChunkText(ctx context.Context, documentIDStr string, content string, chunkSize int) ([]*models.TextChunk, error) {
	if chunkSize <= 0 {
		chunkSize = 200 // Default: 200 characters (suitable for Chinese)
	}
	documentID, err := uuid.Parse(documentIDStr)
	if err != nil {
		return nil, err
	}

	// 使用中文语义分块
	chunkTexts := splitChineseText(content, chunkSize)

	var chunks []*models.TextChunk
	for idx, ct := range chunkTexts {
		if strings.TrimSpace(ct) == "" {
			continue
		}
		runes := []rune(ct)
		wc := len(runes)

		chunks = append(chunks, &models.TextChunk{
			ID:         uuid.New(),
			DocumentID: documentID,
			Content:    ct,
			ChunkIndex: idx,
			StartPos:   0,
			EndPos:     wc,
			WordCount:  wc,
		})
	}
	return chunks, nil
}

// GenerateEmbeddings creates vector embeddings for text chunks
// GenerateEmbeddings creates vector embeddings for text chunks and stores them
func (s *VectorService) GenerateEmbeddings(ctx context.Context, chunks []*models.TextChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Step 1: Collect contents
	contents := make([]string, len(chunks))
	for i, c := range chunks {
		contents[i] = c.Content
	}

	// Step 2: Generate embeddings
	embeddings, err := s.embed.Embed(ctx, contents)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Step 3: Save to DB and Milvus
	for i, chunk := range chunks {
		chunk.Embedding, err = Float64SliceToBytes(embeddings[i])
		if err != nil {
			return fmt.Errorf("failed to convert embedding to bytes: %w", err)
		}

		// Save to PostgreSQL
		if err := s.storeChunk(ctx, chunk); err != nil {
			return err
		}
		fmt.Printf("save chunk, row = %d\n", i)
	}
	fmt.Printf("you have done it %d chunks\n", len(chunks))
	fmt.Printf("s.store type: %T\n", s.store)

	// Step 4: Insert into Milvus directly via MilvusStore
	return s.store.(*MilvusStore).InsertChunks(ctx, chunks, embeddings)
}

// generateSimpleEmbedding creates a simple embedding for demonstration
// In production, this would call an AI service
/* removed */

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
func (s *VectorService) storeChunk(ctx context.Context, chunk *models.TextChunk) error {
	if err := s.db.Create(chunk).Error; err != nil {
		return fmt.Errorf("failed to create chunk: %w", err)
	}
	return nil
}

// SearchSimilarChunks finds chunks similar to a query
func (s *VectorService) SearchSimilarChunks(ctx context.Context, query string, limit int) ([]*models.TextChunk, error) {
	if limit <= 0 {
		limit = 10
	}
	key := s.hash("srch", query, limit)
	if s.redis != nil {
		v, err := s.redis.Get(ctx, key).Result()
		if err == nil && v != "" {
			var cached []*models.TextChunk
			if json.Unmarshal([]byte(v), &cached) == nil {
				return cached, nil
			}
		}
	}
	// 语义检索
	hits, err := s.store.Retrieve(ctx, query, limit, 0)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	ids := make([]uuid.UUID, 0, len(hits))
	for _, h := range hits {
		id, err := uuid.Parse(h.ID)
		if err == nil {
			ids = append(ids, id)
		}
	}
	var dbChunks []models.TextChunk
	if len(ids) > 0 {
		if err := s.db.Where("id IN ?", ids).Find(&dbChunks).Error; err != nil {
			return nil, err
		}
	}
	if limit > 0 && len(dbChunks) > limit {
		dbChunks = dbChunks[:limit]
	}
	result := make([]*models.TextChunk, 0, len(dbChunks))
	for i := range dbChunks {
		result = append(result, &dbChunks[i])
	}
	if s.redis != nil {
		b, _ := json.Marshal(result)
		s.redis.Set(ctx, key, string(b), 60*time.Second)
	}
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
func (s *VectorService) GetDocumentChunks(ctx context.Context, documentID string) ([]*models.TextChunk, error) {
	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document ID: %w", err)
	}

	var dbChunks []models.TextChunk
	if err := s.db.Where("document_id = ?", docUUID).Find(&dbChunks).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch document chunks: %w", err)
	}

	var chunks []*models.TextChunk
	for i := range dbChunks {
		chunks = append(chunks, &dbChunks[i])
	}

	return chunks, nil
}

// DeleteDocumentChunks removes all chunks for a document
func (s *VectorService) DeleteDocumentChunks(ctx context.Context, documentID string) error {
	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return fmt.Errorf("invalid document ID: %w", err)
	}
	var ids []models.TextChunk
	if err := s.db.Select("id").Where("document_id = ?", docUUID).Find(&ids).Error; err != nil {
		return fmt.Errorf("failed to fetch ids: %w", err)
	}
	qids := make([]string, 0, len(ids))
	for _, c := range ids {
		qids = append(qids, c.ID.String())
	}
	if len(qids) > 0 {
		_ = s.store.DeleteByIDs(ctx, qids)
	}
	if err := s.db.Where("document_id = ?", docUUID).Delete(&models.TextChunk{}).Error; err != nil {
		return fmt.Errorf("failed to delete document chunks: %w", err)
	}

	logger.Info(ctx, "Document chunks deleted", "document_id", documentID)
	return nil
}

func (s *VectorService) hash(prefix string, text string, limit int) string {
	h := sha256.Sum256([]byte(text))
	return prefix + ":" + hex.EncodeToString(h[:8]) + ":" + fmt.Sprintf("%d", limit)
}
