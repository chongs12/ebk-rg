package rag_query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	arkmodel "github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/chongs12/enterprise-knowledge-base/internal/vector"
	"github.com/chongs12/enterprise-knowledge-base/pkg/database"
)

// RAGQueryRequest 同步与异步查询的通用入参
type RAGQueryRequest struct {
	Query       string
	Limit       int
	Temperature float32
	MaxTokens   int
	SessionID   string
}

// RAGQueryResult 查询结果结构，包含答案与来源
type RAGQueryResult struct {
	Answer  string
	Sources []map[string]any
	Usage   map[string]int
}

// RAGQueryService 提供检索增强生成的核心能力
type RAGQueryService struct {
	db    *database.Database
	redis *redis.Client
	vs    *vector.VectorService
	chat  *arkmodel.ChatModel
}

// NewRAGQueryService 创建服务实例
func NewRAGQueryService(db *database.Database, redis *redis.Client, vs *vector.VectorService, chat *arkmodel.ChatModel) *RAGQueryService {
	return &RAGQueryService{db: db, redis: redis, vs: vs, chat: chat}
}

// AskSync 执行同步查询并返回完整答案
func (s *RAGQueryService) AskSync(ctx context.Context, userID uuid.UUID, req *RAGQueryRequest) (*RAGQueryResult, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("query is empty")
	}

	// 结果缓存：按 query+limit 做缓存键
	ckey := s.cacheKey("rag", req.Query, req.Limit)
	if s.redis != nil {
		if val, err := s.redis.Get(ctx, ckey).Result(); err == nil && val != "" {
			return &RAGQueryResult{Answer: val, Sources: nil, Usage: map[string]int{}}, nil
		}
	}

	// 语义检索：使用向量服务，严格遵守 limit
	chunks, err := s.vs.SearchSimilarChunks(ctx, req.Query, req.Limit)
	if err != nil {
		return nil, err
	}

	// 来源组装与上下文构建
	var sb strings.Builder
	sources := make([]map[string]any, 0, len(chunks))
	for _, ch := range chunks {
		preview := ch.Content
		if len(preview) > 200 {
			preview = preview[:200]
		}
		sb.WriteString("\n[chunk#")
		sb.WriteString(fmt.Sprintf("%d", ch.ChunkIndex))
		sb.WriteString("] ")
		sb.WriteString(ch.Content)
		sources = append(sources, map[string]any{
			"id":              ch.ID.String(),
			"document_id":     ch.DocumentID.String(),
			"chunk_index":     ch.ChunkIndex,
			"content_excerpt": preview,
		})
	}

	// 构造消息：system+user（含上下文与对话记忆）
	history := s.loadHistory(ctx, userID, req.SessionID)
	msgs := make([]*schema.Message, 0, len(history)+2)
	msgs = append(msgs, &schema.Message{Role: schema.System, Content: "你是企业知识库的检索增强问答助手。严格依据提供的上下文回答，无法回答时请明确说明。"})
	msgs = append(msgs, history...)
	userContent := fmt.Sprintf("问题：%s\n上下文：%s", req.Query, sb.String())
	msgs = append(msgs, &schema.Message{Role: schema.User, Content: userContent})

	// 生成调用
	resp, err := s.chat.Generate(ctx, msgs) // 运行时参数（温度与最大token）
	// 这里使用 Ark ChatModel 的配置方式：通过 ChatModelConfig 设定全局，或在 Generate 的 opts 中设定（若支持）。

	if err != nil {
		return nil, err
	}

	answer := resp.Content
	usage := map[string]int{}
	if resp.ResponseMeta != nil {
		if resp.ResponseMeta.Usage != nil {
			usage["prompt_tokens"] = resp.ResponseMeta.Usage.PromptTokens
			usage["completion_tokens"] = resp.ResponseMeta.Usage.CompletionTokens
			usage["total_tokens"] = resp.ResponseMeta.Usage.TotalTokens
		}
	}

	// 写入对话记忆
	s.saveHistory(ctx, userID, req.SessionID, &schema.Message{Role: schema.User, Content: req.Query})
	s.saveHistory(ctx, userID, req.SessionID, &schema.Message{Role: schema.Assistant, Content: answer})

	// 写入缓存
	if s.redis != nil {
		_ = s.redis.Set(ctx, ckey, answer, 60*time.Second).Err()
	}

	return &RAGQueryResult{Answer: answer, Sources: sources, Usage: usage}, nil
}

// AskStream 执行异步流式查询，返回增量答案片段
func (s *RAGQueryService) AskStream(ctx context.Context, userID uuid.UUID, req *RAGQueryRequest) (<-chan string, <-chan error) {
	out := make(chan string, 16)
	errs := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errs)

		chunks, err := s.vs.SearchSimilarChunks(ctx, req.Query, req.Limit)
		if err != nil {
			errs <- err
			return
		}
		var sb strings.Builder
		for _, ch := range chunks {
			sb.WriteString("\n[chunk#")
			sb.WriteString(fmt.Sprintf("%d", ch.ChunkIndex))
			sb.WriteString("] ")
			sb.WriteString(ch.Content)
		}
		history := s.loadHistory(ctx, userID, req.SessionID)
		msgs := make([]*schema.Message, 0, len(history)+2)
		msgs = append(msgs, &schema.Message{Role: schema.System, Content: "你是企业知识库的检索增强问答助手。严格依据提供的上下文回答，无法回答时请明确说明。"})
		msgs = append(msgs, history...)
		msgs = append(msgs, &schema.Message{Role: schema.User, Content: fmt.Sprintf("问题：%s\n上下文：%s", req.Query, sb.String())})

		sr, err := s.chat.Stream(ctx, msgs)
		if err != nil {
			errs <- err
			return
		}
		var final strings.Builder
		for {
			m, e := sr.Recv()
			if e != nil {
				if e.Error() == "EOF" || strings.Contains(e.Error(), "EOF") {
					break
				}
				errs <- e
				return
			}
			if m != nil {
				out <- m.Content
				final.WriteString(m.Content)
			}
		}

		// 保存对话记忆
		s.saveHistory(ctx, userID, req.SessionID, &schema.Message{Role: schema.User, Content: req.Query})
		s.saveHistory(ctx, userID, req.SessionID, &schema.Message{Role: schema.Assistant, Content: final.String()})
	}()

	return out, errs
}

// loadHistory 读取多轮对话记忆
func (s *RAGQueryService) loadHistory(ctx context.Context, userID uuid.UUID, sessionID string) []*schema.Message {
	if s.redis == nil || sessionID == "" {
		return nil
	}
	key := fmt.Sprintf("rag:hist:%s:%s", userID.String(), sessionID)
	vals, err := s.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil || len(vals) == 0 {
		return nil
	}
	msgs := make([]*schema.Message, 0, len(vals))
	for _, v := range vals {
		// 约定格式："role|content"
		parts := strings.SplitN(v, "|", 2)
		if len(parts) != 2 {
			continue
		}
		role := parts[0]
		content := parts[1]
		var r schema.RoleType
		switch role {
		case "system":
			r = schema.System
		case "assistant":
			r = schema.Assistant
		default:
			r = schema.User
		}
		msgs = append(msgs, &schema.Message{Role: r, Content: content})
	}
	return msgs
}

// saveHistory 写入多轮对话记忆
func (s *RAGQueryService) saveHistory(ctx context.Context, userID uuid.UUID, sessionID string, msg *schema.Message) {
	if s.redis == nil || sessionID == "" || msg == nil {
		return
	}
	key := fmt.Sprintf("rag:hist:%s:%s", userID.String(), sessionID)
	role := "user"
	if msg.Role == schema.System {
		role = "system"
	}
	if msg.Role == schema.Assistant {
		role = "assistant"
	}
	_ = s.redis.RPush(ctx, key, role+"|"+msg.Content).Err()
	_ = s.redis.Expire(ctx, key, 24*time.Hour).Err()
}

// cacheKey 生成缓存键
func (s *RAGQueryService) cacheKey(prefix string, text string, limit int) string {
	h := sha256.Sum256([]byte(text))
	return prefix + ":" + hex.EncodeToString(h[:8]) + ":" + fmt.Sprintf("%d", limit)
}
