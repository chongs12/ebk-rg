package vector

import (
	"context"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/chongs12/enterprise-knowledge-base/internal/common/models"
	"github.com/chongs12/enterprise-knowledge-base/pkg/logger"
	arkext "github.com/cloudwego/eino-ext/components/embedding/ark"
	milindex "github.com/cloudwego/eino-ext/components/indexer/milvus"
	milret "github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/schema"
	milvus "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type MilvusStore struct {
	client      milvus.Client
	indexer     *milindex.Indexer
	retriever   *milret.Retriever
	collection  string
	vectorField string
	vectorDim   int
	vectorType  string
}

type milvusSearchParam struct {
	params map[string]interface{}
}

func (sp *milvusSearchParam) Params() map[string]interface{} {
	p := make(map[string]interface{}, len(sp.params))
	for k, v := range sp.params {
		p[k] = v
	}
	return p
}

func (sp *milvusSearchParam) AddRadius(radius float64) {
	sp.params["radius"] = radius
}

func (sp *milvusSearchParam) AddRangeFilter(rangeFilter float64) {
	sp.params["range_filter"] = rangeFilter
}

func NewMilvusSearchParam(params map[string]interface{}) entity.SearchParam {
	p := make(map[string]interface{}, len(params))
	for k, v := range params {
		p[k] = v
	}
	return &milvusSearchParam{params: p}
}

// 初始化 Milvus 存储（包含索引与检索器）
func NewMilvusStore(ctx context.Context, cli milvus.Client, collection string, arkAPIKey, arkModel string, arkBaseURL, arkRegion string, vectorField string, vectorDim int, vectorType string) (*MilvusStore, error) {
	emb, err := arkext.NewEmbedder(ctx, &arkext.EmbeddingConfig{APIKey: arkAPIKey, Model: arkModel, BaseURL: arkBaseURL, Region: arkRegion})
	if err != nil {
		return nil, err
	}
	fields := buildFields(vectorField, vectorDim, vectorType)

	// 👇 新增：根据 vectorType 推导 indexer 所需的 MetricType
	var indexerMetric milindex.MetricType
	if strings.ToLower(vectorType) == "binary" {
		indexerMetric = "HAMMING"
	} else {
		indexerMetric = "COSINE" // 或 "L2", "IP"，根据你的 embedding 模型选择
	}

	// ✅ 新增：自定义 DocumentConverter，避免默认 converter 将 float64 向量转成 []byte
	customConverter := func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error) {
		rows := make([]interface{}, len(docs))
		for i, doc := range docs {
			metaBytes, err := sonic.Marshal(doc.MetaData)
			if err != nil {
				return nil, fmt.Errorf("marshal metadata: %w", err)
			}

			// 将 []float64 转为 Milvus 要求的 []float32
			vec32 := make([]float32, len(vectors[i]))
			for j, v := range vectors[i] {
				vec32[j] = float32(v)
			}

			// 仅提供向量与元数据列，避免与 Indexer 默认列（id/content）重复
			rows[i] = map[string]interface{}{
				vectorField: vec32,
				"metadata":  metaBytes,
			}
			_ = doc
		}
		return rows, nil
	}

	// 👇 修改：传入 MetricType 和自定义 DocumentConverter
	idx, err := milindex.NewIndexer(ctx, &milindex.IndexerConfig{
		Client:            cli,
		Collection:        collection,
		Embedding:         emb,
		Fields:            fields,
		MetricType:        indexerMetric,
		DocumentConverter: customConverter, // ← 关键修复！
	})
	if err != nil {
		return nil, err
	}

	// Retriever 的 MetricType 保持不变（你已正确设置）
	var retrieverMetric entity.MetricType
	if strings.ToLower(vectorType) == "binary" {
		retrieverMetric = entity.HAMMING
	} else {
		retrieverMetric = entity.COSINE
	}
	customVectorConverter := func(ctx context.Context, vectors [][]float64) ([]entity.Vector, error) {
		if len(vectors) == 0 || len(vectors[0]) == 0 {
			return nil, fmt.Errorf("empty vectors")
		}
		vec32 := make([]float32, len(vectors[0]))
		for i, v := range vectors[0] {
			vec32[i] = float32(v)
		}
		return []entity.Vector{entity.FloatVector(vec32)}, nil
	}
	searchParam := NewMilvusSearchParam(map[string]interface{}{
		"ef": 64,
	})

	customRetrieverConverter := func(ctx context.Context, result milvus.SearchResult) ([]*schema.Document, error) {
		logger.Info(ctx, "Actual type of result", "type", fmt.Sprintf("%T", result))
		logger.Info(ctx, "Actual type of result.Fields", "type", fmt.Sprintf("%T", result.Fields))
		var docs []*schema.Document
		n := len(result.Scores)
		if n == 0 {
			return docs, nil
		}

		// ID 字段
		idCol, ok := result.IDs.(*entity.ColumnVarChar)
		if !ok {
			return nil, fmt.Errorf("ID is not VarChar")
		}

		// content 字段（在不同 SDK 版本中，Fields 可能是切片）
		// 找 content 字段
		var contentCol *entity.ColumnVarChar
		for _, col := range result.Fields {
			if col.Name() == "content" {
				if c, ok := col.(*entity.ColumnVarChar); ok {
					contentCol = c
					break
				}
			}
		}
		if contentCol == nil {
			// 更详细的错误信息，便于排查
			fieldNames := make([]string, len(result.Fields))
			for i, col := range result.Fields {
				fieldNames[i] = col.Name()
			}
			return nil, fmt.Errorf("content field not in output_fields; available fields: %v", fieldNames)
		}

		// metadata 字段（JSON）
		var metaBytes [][]byte
		var metaCol entity.Column
		for _, col := range result.Fields {
			if col.Name() == "metadata" {
				metaCol = col
				break
			}
		}
		if metaCol != nil {
			if mb, ok := metaCol.(*entity.ColumnJSONBytes); ok {
				metaBytes = mb.Data()
			} else if c, ok := metaCol.(interface{ Data() [][]byte }); ok {
				metaBytes = c.Data()
			}
			// 否则留空
		}

		for i := 0; i < n; i++ {
			if i >= len(idCol.Data()) || i >= len(contentCol.Data()) {
				continue
			}

			doc := &schema.Document{
				ID:      idCol.Data()[i],
				Content: contentCol.Data()[i],
				MetaData: map[string]any{
					"score": result.Scores[i], // ← 关键：注入真实分数！
				},
			}

			// 合并原始 metadata
			if i < len(metaBytes) && metaBytes[i] != nil {
				var m map[string]any
				if err = sonic.Unmarshal(metaBytes[i], &m); err == nil {
					for k, v := range m {
						doc.MetaData[k] = v
					}
				}
			}

			docs = append(docs, doc)
		}
		return docs, nil
	}

	ret, err := milret.NewRetriever(ctx, &milret.RetrieverConfig{
		Client:            cli,
		Collection:        collection,
		Embedding:         emb,
		TopK:              10,
		VectorField:       vectorField,
		MetricType:        retrieverMetric,
		VectorConverter:   customVectorConverter,
		Sp:                searchParam,
		ScoreThreshold:    0,
		DocumentConverter: customRetrieverConverter,

		// 👇 关键新增：明确指定要返回哪些字段！
		OutputFields: []string{"id", "content", "metadata"},
	})
	if err != nil {
		return nil, err
	}

	return &MilvusStore{
		client:      cli,
		indexer:     idx,
		retriever:   ret,
		collection:  collection,
		vectorField: vectorField,
		vectorDim:   vectorDim,
		vectorType:  vectorType,
	}, nil
}

// 其余代码保持不变（从 EnsureCollection 开始往下都不需要改）
func (s *MilvusStore) EnsureCollection(ctx context.Context) error { return nil }

func (s *MilvusStore) IndexChunks(ctx context.Context, chunks []*models.TextChunk) error {
	logger.Info(ctx, "Index chunks", "collection", s.collection, "field", s.vectorField, "type", s.vectorType, "dim", s.vectorDim, "count", len(chunks))
	docs := make([]*schema.Document, 0, len(chunks))
	for _, c := range chunks {
		docs = append(docs, &schema.Document{ID: c.ID.String(), Content: c.Content, MetaData: map[string]any{"document_id": c.DocumentID.String(), "chunk_index": c.ChunkIndex, "start_pos": c.StartPos, "end_pos": c.EndPos, "word_count": c.WordCount}})
	}
	_, err := s.indexer.Store(ctx, docs)
	return err
}

type Hit struct {
	ID    string
	Score float32
}

func (s *MilvusStore) Retrieve(ctx context.Context, query string, limit int, scoreThreshold float32) ([]Hit, error) {
	docs, err := s.retriever.Retrieve(ctx, query)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	var hits []Hit
	for _, d := range docs {
		var score float64
		if d.MetaData != nil {
			if raw, ok := d.MetaData["score"]; ok {
				switch v := raw.(type) {
				case float64:
					score = v
				case float32:
					score = float64(v)
				case int, int32, int64:
					score = float64(v.(int64))
				default:
					score = 0
				}
			}
		}

		// ✅ 安全截断：按 rune 而非 byte
		preview := TruncateToRunes(d.Content, 50)
		logger.Info(ctx, "Doc score", "id", d.ID, "content_preview", preview, "score", score)

		if scoreThreshold > 0 && score < float64(scoreThreshold) {
			continue
		}
		hits = append(hits, Hit{ID: d.ID, Score: float32(score)})
		if len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

func (s *MilvusStore) DeleteByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	expr := fmt.Sprintf(`id in ["%s"]`, strings.Join(ids, `","`))
	return s.client.Delete(ctx, s.collection, "", expr)
}

// InsertChunks inserts pre-chunked and pre-embedded data into Milvus
func (s *MilvusStore) InsertChunks(ctx context.Context, chunks []*models.TextChunk, embeddings [][]float64) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("chunks and embeddings length mismatch")
	}

	n := len(chunks)

	// Pre-allocate slices for each column
	ids := make([]string, n)
	contents := make([]string, n)
	vectors := make([][]float32, n)
	metadatas := make([][]byte, n)

	for i, chunk := range chunks {
		// ID
		ids[i] = chunk.ID.String()

		// Content
		contents[i] = chunk.Content

		// Vector: []float64 -> []float32
		vec32 := make([]float32, len(embeddings[i]))
		for j, v := range embeddings[i] {
			vec32[j] = float32(v)
		}
		vectors[i] = vec32

		// Metadata
		meta := map[string]interface{}{
			"document_id": chunk.DocumentID.String(),
			"chunk_index": chunk.ChunkIndex,
			"start_pos":   chunk.StartPos,
			"end_pos":     chunk.EndPos,
			"word_count":  chunk.WordCount,
		}
		metaBytes, err := sonic.Marshal(meta)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		metadatas[i] = metaBytes
	}

	// 构造正确的 columns：每个字段一个 Column，包含所有数据
	columns := []entity.Column{
		entity.NewColumnVarChar("id", ids),
		entity.NewColumnVarChar("content", contents),
		entity.NewColumnFloatVector(s.vectorField, s.vectorDim, vectors),
		entity.NewColumnJSONBytes("metadata", metadatas),
	}

	_, err := s.client.Insert(ctx, s.collection, "", columns...)
	return err
}

func buildFields(vectorField string, vectorDim int, vectorType string) []*entity.Field {
	id := &entity.Field{Name: "id", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "256"}, PrimaryKey: true}
	content := &entity.Field{Name: "content", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "8192"}}
	metadata := &entity.Field{Name: "metadata", DataType: entity.FieldTypeJSON}
	var vector *entity.Field
	if strings.ToLower(vectorType) == "binary" {
		vector = &entity.Field{Name: vectorField, DataType: entity.FieldTypeBinaryVector, TypeParams: map[string]string{"dim": fmt.Sprintf("%d", vectorDim)}}
	} else {
		vector = &entity.Field{Name: vectorField, DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": fmt.Sprintf("%d", vectorDim)}}
	}
	return []*entity.Field{id, vector, content, metadata}
}

func (s *MilvusStore) LogDiagnostics(ctx context.Context) error {
	coll, err := s.client.DescribeCollection(ctx, s.collection)
	if err != nil {
		logger.Warn(ctx, "DescribeCollection failed", "collection", s.collection, "error", err.Error())
		return err
	}
	infos := make([]string, 0, len(coll.Schema.Fields))
	for _, f := range coll.Schema.Fields {
		tp := ""
		if f.TypeParams != nil {
			if dim, ok := f.TypeParams["dim"]; ok {
				tp = fmt.Sprintf("dim=%s", dim)
			}
			if ml, ok := f.TypeParams["max_length"]; ok {
				if tp == "" {
					tp = fmt.Sprintf("max_length=%s", ml)
				} else {
					tp = tp + ",max_length=" + ml
				}
			}
		}
		infos = append(infos, fmt.Sprintf("%s:%s(%s)", f.Name, f.DataType.String(), tp))
	}
	logger.Info(ctx, "Milvus collection schema", "collection", s.collection, "fields", strings.Join(infos, "; "))
	return nil
}
