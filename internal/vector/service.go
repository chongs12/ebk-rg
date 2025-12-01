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
// 用途：
// - 针对包含中文的文本，按照句子结束符进行语义分块，尽可能保持句子完整性
// 参数：
// - text：原始文本
// - maxChars：单块最大字符数（按 rune 计数），超过时会进行强制切分
// 返回：
// - []string：分块后的文本切片
func splitChineseText(text string, maxChars int) []string {
	if strings.TrimSpace(text) == "" {
		return []string{}
	}

	// 按句子结束符分割：中文句号、感叹号、问号、分号、换行等
	// 匹配常见中文/英文句子结束符号以及换行符
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

		// 极端情况：单句超长，强制切分（按固定长度截断，避免超出向量维度限制）
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

		// 追加当前句（在块内使用空格衔接，避免过度粘连）
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

// TextChunk 相关逻辑说明：
// - 业务侧以 `models.TextChunk` 作为分块实体，记录文本片段与位置等信息
// - 向量存储在 Milvus 中，数据库仅存文本与必要元数据（embedding 以字节保留可选）

// ChunkText 将文档内容拆分为易管理的文本块
// 用途：
// - 根据指定的 chunkSize 对文本进行中文语义分块
// 参数：
// - documentIDStr：文档 ID（字符串）
// - content：原始文本内容
// - chunkSize：每块最大字符数，<=0 时使用默认 200
// 返回：
// - []*models.TextChunk：生成的分块列表（包含索引与统计信息）
// - error：失败时返回错误
func (s *VectorService) ChunkText(ctx context.Context, documentIDStr string, content string, chunkSize int) ([]*models.TextChunk, error) {
	if chunkSize <= 0 {
		chunkSize = 200 // Default: 200 characters (suitable for Chinese)
	}
	documentID, err := uuid.Parse(documentIDStr)
	if err != nil {
		return nil, err
	}

	// 使用中文语义分块（按句子与最大长度进行整合）
	chunkTexts := splitChineseText(content, chunkSize)

	var chunks []*models.TextChunk
	for idx, ct := range chunkTexts {
		if strings.TrimSpace(ct) == "" {
			continue
		}
		runes := []rune(ct)
		wc := len(runes) // 此处 wordCount 以字符计数，适配中英文统一处理

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

// GenerateEmbeddings 为文本分块生成向量并入库
// 用途：
// - 批量调用 Embedder 生成向量
// - 将分块与向量写入关系库与 Milvus
// 返回：
// - error：失败时返回错误
func (s *VectorService) GenerateEmbeddings(ctx context.Context, chunks []*models.TextChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Step 1: Collect contents
	contents := make([]string, len(chunks))
	for i, c := range chunks {
		contents[i] = c.Content
	}

	// Step 2: 生成向量（批量）
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

		// 保存至关系库（记录分块与向量字节，可选）
		if err := s.storeChunk(ctx, chunk); err != nil {
			return err
		}
		fmt.Printf("save chunk, row = %d\n", i)
	}
	fmt.Printf("you have done it %d chunks\n", len(chunks))
	fmt.Printf("s.store type: %T\n", s.store)

	// Step 4: 通过 MilvusStore 直接插入向量（高效批量列插入）
	return s.store.(*MilvusStore).InsertChunks(ctx, chunks, embeddings)
}

// generateSimpleEmbedding creates a simple embedding for demonstration
// In production, this would call an AI service
/* removed */

// sqrt 简单的平方根函数（牛顿法）
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

// storeChunk 将分块记录保存到数据库
func (s *VectorService) storeChunk(ctx context.Context, chunk *models.TextChunk) error {
	if err := s.db.Create(chunk).Error; err != nil {
		return fmt.Errorf("failed to create chunk: %w", err)
	}
	return nil
}

// SearchSimilarChunks 执行语义检索，返回相似分块
// 用途：
// - 通过向量检索命中分块 ID，再回查数据库组装完整分块对象
// - 支持 Redis 缓存命中以降低后端压力
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
	// 语义检索（由 store 决定底层召回实现）
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

// Search performs semantic search and returns chunks with scores (distance)
func (s *VectorService) Search(ctx context.Context, query string, limit int) ([]models.TextChunkWithDistance, error) {
	if limit <= 0 {
		limit = 10
	}
	
	hits, err := s.store.Retrieve(ctx, query, limit, 0)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}

	// Map ID -> Score
	scoreMap := make(map[string]float32)
	ids := make([]uuid.UUID, 0, len(hits))
	for _, h := range hits {
		id, err := uuid.Parse(h.ID)
		if err == nil {
			ids = append(ids, id)
			scoreMap[h.ID] = h.Score
		}
	}

	var dbChunks []models.TextChunk
	if len(ids) > 0 {
		if err := s.db.Where("id IN ?", ids).Find(&dbChunks).Error; err != nil {
			return nil, err
		}
	}

	chunkMap := make(map[string]models.TextChunk)
	for _, c := range dbChunks {
		chunkMap[c.ID.String()] = c
	}

	result := make([]models.TextChunkWithDistance, 0, len(ids))
	for _, id := range ids {
		idStr := id.String()
		if chunk, ok := chunkMap[idStr]; ok {
			result = append(result, models.TextChunkWithDistance{
				TextChunk: chunk,
				Distance:  scoreMap[idStr],
			})
		}
	}

	return result, nil
}

// cosineSimilarity 计算两个向量的余弦相似度
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

// GetDocumentChunks 获取指定文档的全部分块
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

// DeleteDocumentChunks 删除指定文档的全部分块（含 Milvus 与数据库）
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

// hash 生成缓存键（前缀+文本短哈希+limit）
func (s *VectorService) hash(prefix string, text string, limit int) string {
	h := sha256.Sum256([]byte(text))
	return prefix + ":" + hex.EncodeToString(h[:8]) + ":" + fmt.Sprintf("%d", limit)
}
