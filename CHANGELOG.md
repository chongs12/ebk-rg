# 变更日志（CHANGELOG）

## 1.1.0 — 2025-11-26

### 新增
- 向量服务重构为 Ark + Milvus 架构（Eino 组件）：
  - 集成 `ArkEmbedder` 生成文本向量
  - 集成 `MilvusStore`（Indexer/Retriever）进行索引与检索
  - 向量批量列式插入，提升写入性能
- 为核心代码新增中文注释（函数头注释、关键逻辑行内注释），提高可读性与维护性
- 单元测试更新：调整分块用例以匹配按字符分块逻辑，并添加中文说明

### 修复
- 统一向量类型转换（`float64 -> float32`）避免与 Milvus 字段类型不一致导致的插入/检索异常
- 检索结果字段读取健壮性提升：确保 `id/content/metadata` 均可正确解析

### 重大变更
- 向量后端由 Qdrant 迁移至 Milvus；内部实现改用 CloudWeGo Eino 组件生态
  - API 兼容：`/api/v1/vectors/*` 路由与请求/响应保持不变
  - 存储变更：向量不再依赖旧后端，需部署 Milvus 服务

### 兼容性注意事项
- 需要在运行环境配置以下变量：
  - `MILVUS_ADDR`, `MILVUS_USERNAME`, `MILVUS_PASSWORD`, `MILVUS_COLLECTION`
  - `ARK_API_KEY`, `ARK_MODEL`（可选 `ARK_BASE_URL`, `ARK_REGION`）
- 确认 Milvus 集合的向量维度与 Ark 模型输出维度一致，否则写入或检索可能失败
- 旧数据如需迁移，建议重嵌入向量后再写入新集合，避免维度不一致问题

---

本变更遵循语义化版本（SemVer）。如需回滚，请将向量服务切回旧后端配置，并停止对新集合的写入。

