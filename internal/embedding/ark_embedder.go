package embedding

// ArkEmbedder 是针对火山引擎 Ark 的向量嵌入适配器。
//
// 功能说明：
// - 对外暴露统一的 Embed 接口，生成文本的向量表示（embedding）。
// - 封装 Ark 的初始化与配置细节，简化业务调用。
//
// 设计要点：
// - 使用 Eino 扩展组件的 Ark Embedder，以获得稳定的 SDK 能力。
// - 支持可选的 BaseURL 与 Region，以便在不同区域与私有化部署中复用。
//
// 注意事项：
// - 需要在运行环境提供有效的 API Key（或 AK/SK），并确保网络可达。
// - 向量维度由模型决定，后续在存储端需与 Milvus 集合字段一致。
import (
	"context"

	arkext "github.com/cloudwego/eino-ext/components/embedding/ark"
)

// ArkEmbedder 封装 Ark 的嵌入器实例
// 参数含义：
// - emb：Eino Ark 的嵌入器实现，用于实际生成向量
// 返回值：
// - 通过方法 Embed 返回二维浮点切片，表示批量文本的向量集合
type ArkEmbedder struct {
	emb arkext.Embedder
}

// 新建 Ark 向量嵌入器（使用火山引擎 Ark）
// 用途：
// - 根据提供的 apiKey/model/baseURL/region 创建并初始化 Ark 嵌入器
// 参数：
// - apiKey：Ark 的访问密钥
// - model：Ark 嵌入模型标识
// - baseURL：可选，自定义服务地址
// - region：可选，服务区域
// 返回：
// - *ArkEmbedder 成功时返回实例；error 失败时返回错误信息
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

// Embed 生成文本向量
// 用途：
// - 对输入的字符串数组批量生成向量表示
// 参数：
// - ctx：请求上下文，用于超时与取消控制
// - inputs：待嵌入的文本切片
// 返回：
// - [][]float64：每个文本对应的向量数组
// - error：失败时返回错误
func (a *ArkEmbedder) Embed(ctx context.Context, inputs []string) ([][]float64, error) {
	return a.emb.EmbedStrings(ctx, inputs)
}
