package embedding

import (
	"context"

	arkext "github.com/cloudwego/eino-ext/components/embedding/ark"
)

type ArkEmbedder struct {
	emb arkext.Embedder
}

// 新建 Ark 向量嵌入器（使用火山引擎 Ark）
func NewArkEmbedder(apiKey, model, baseURL, region string) (*ArkEmbedder, error) {
	cfg := &arkext.EmbeddingConfig{
		APIKey: apiKey,
		Model:  model,
	}
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	if region != "" {
		cfg.Region = region
	}
	emb, err := arkext.NewEmbedder(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	return &ArkEmbedder{emb: *emb}, nil
}

// 生成文本向量
func (a *ArkEmbedder) Embed(ctx context.Context, inputs []string) ([][]float64, error) {
	return a.emb.EmbedStrings(ctx, inputs)
}
