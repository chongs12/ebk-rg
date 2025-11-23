# Enterprise Knowledge Base (EKB)

企业级智能知识库系统，基于微服务架构和RAG（Retrieval-Augmented Generation）技术构建。

## 🚀 核心特性

- **微服务架构**: 采用Go语言和现代云原生技术栈
- **RAG智能问答**: 基于向量检索和LLM的自然语言问答
- **多文档支持**: 支持PDF、Word、TXT等多种文档格式
- **权限控制**: 细粒度的文档访问权限管理
- **向量检索**: 基于MySQL 8.0+向量函数的高效相似度搜索
- **JWT认证**: 无状态用户认证和授权
- **分布式追踪**: 完整的链路追踪和监控

## 🏗️ 系统架构

### 微服务组件

1. **Auth Service** (端口: 8081) - 用户认证和授权
2. **Document Service** (端口: 8082) - 文档上传和处理
3. **Vector Service** (端口: 8083) - 文本向量化和存储
4. **Query Service** (端口: 8084) - RAG查询和智能问答
5. **Gateway Service** (端口: 8080) - API网关和路由

### 技术栈

- **后端**: Go 1.21+, Gin框架
- **AI框架**: Eino (字节跳动开源Go AI框架)
- **数据库**: MySQL 8.0+ (支持向量函数)
- **缓存**: Redis
- **认证**: JWT
- **部署**: Docker + Kubernetes
- **监控**: Jaeger, Prometheus, Grafana

## 🛠️ 快速开始

### 环境要求

- Go 1.21+
- MySQL 8.0+
- Redis 6.0+
- Docker (可选)

### 1. 克隆项目

```bash
git clone https://github.com/your-org/enterprise-knowledge-base.git
cd enterprise-knowledge-base
```

### 2. 配置环境变量

```bash
cp .env.example .env
# 编辑 .env 文件，配置数据库和API密钥
```

### 3. 安装依赖

```bash
go mod download
```

### 4. 数据库初始化

```bash
# 运行数据库迁移
mysql -u root -p < migrations/001_create_users_table.sql
mysql -u root -p < migrations/002_create_documents_tables.sql
mysql -u root -p < migrations/003_create_queries_tables.sql
```

### 5. 启动服务

#### 启动认证服务
```bash
cd cmd/auth
go run main.go
```

#### 启动文档服务
```bash
cd cmd/document
go run main.go
```

#### 启动向量服务
```bash
cd cmd/vector
go run main.go
```

#### 启动查询服务
```bash
cd cmd/query
go run main.go
```

#### 启动网关服务
```bash
cd cmd/gateway
go run main.go
```

## 📖 API文档

### 认证API

#### 用户注册
```http
POST /api/v1/auth/register
Content-Type: application/json

{
  "username": "john_doe",
  "email": "john@example.com",
  "password": "securepassword123",
  "first_name": "John",
  "last_name": "Doe"
}
```

#### 用户登录
```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "john_doe",
  "password": "securepassword123"
}
```

#### 刷新令牌
```http
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "your-refresh-token"
}
```

### 文档API

#### 上传文档
```http
POST /api/v1/documents
Authorization: Bearer your-jwt-token
Content-Type: multipart/form-data

file: @document.pdf
title: "Company Policy Manual"
description: "Employee handbook for 2024"
is_public: false
```

#### 查询文档
```http
POST /api/v1/query
Authorization: Bearer your-jwt-token
Content-Type: application/json

{
  "query": "What is the company vacation policy?",
  "max_results": 3
}
```

## 🔧 开发指南

### 项目结构

```
enterprise-knowledge-base/
├── cmd/                    # 服务入口
│   ├── auth/              # 认证服务
│   ├── document/          # 文档服务
│   ├── vector/            # 向量服务
│   ├── query/             # 查询服务
│   └── gateway/           # 网关服务
├── internal/              # 内部包
│   ├── auth/              # 认证逻辑
│   ├── document/          # 文档处理
│   ├── vector/            # 向量化处理
│   ├── query/             # 查询处理
│   └── common/            # 公共模型
├── pkg/                   # 公共包
│   ├── config/            # 配置管理
│   ├── database/          # 数据库连接
│   ├── logger/            # 日志系统
│   ├── middleware/        # 中间件
│   └── utils/             # 工具函数
├── api/                   # API定义
│   ├── proto/             # gRPC协议
│   └── rest/              # REST API
├── migrations/            # 数据库迁移
├── deployments/           # 部署配置
│   ├── docker/            # Docker配置
│   └── kubernetes/        # K8s配置
└── tests/                 # 测试文件
```

### 代码规范

- 使用Go 1.21+的现代特性
- 遵循Clean Architecture原则
- 实现完整的错误处理
- 添加适当的日志记录
- 编写单元测试和集成测试

## 🧪 测试

```bash
# 运行所有测试
go test ./...

# 运行指定服务测试
go test ./internal/auth/...

# 生成测试覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 📊 监控和日志

### 分布式追踪
- Jaeger UI: http://localhost:16686
- 服务名称: enterprise-knowledge-base

### 性能监控
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000

### 日志查看
```bash
# 查看服务日志
docker logs auth-service
docker logs document-service
docker logs vector-service
docker logs query-service
```

## 🚀 部署

### Docker部署

```bash
# 构建所有服务镜像
docker-compose build

# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps
```

### Kubernetes部署

```bash
# 应用Kubernetes配置
kubectl apply -f deployments/kubernetes/

# 查看服务状态
kubectl get pods
kubectl get services
```

## 🤝 贡献

1. Fork项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建Pull Request

## 📄 许可证

本项目采用MIT许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🆘 支持

如有问题或建议，请提交Issue或联系我们。

## 🏆 技术亮点

- **高性能**: Go语言的并发特性，支持高并发处理
- **可扩展**: 微服务架构，支持水平扩展
- **高可用**: 服务发现和负载均衡
- **安全性**: JWT认证，RBAC权限控制
- **可观测性**: 完整的监控、日志和追踪体系
- **现代化**: 使用最新的Go语言特性和云原生技术