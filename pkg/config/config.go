package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	AI       AIConfig
	Ark      ArkConfig
	Milvus   MilvusConfig
	Storage  StorageConfig
	Tracing  TracingConfig
	RagQuery RagQueryConfig
	Gateway  GatewayConfig
	RabbitMQ RabbitMQConfig
}

type RabbitMQConfig struct {
	URL   string
	Queue string
}

type ServerConfig struct {
	Port         string
	Mode         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Host         string
	Port         string
	Username     string
	Password     string
	Database     string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret     string
	ExpireTime time.Duration
	Issuer     string
}

type AIConfig struct {
	OpenAIAPIKey   string
	EmbeddingModel string
	ChatModel      string
	MaxTokens      int
	Temperature    float64
}

type ArkConfig struct {
	APIKey    string
	AccessKey string
	SecretKey string
	Model     string
	BaseURL   string
	Region    string
}

type RagQueryParameters struct {
	Temperature float64
	MaxTokens   int
}

type RagQueryConfig struct {
	Model      string
	Parameters RagQueryParameters
}

type GatewayConfig struct {
	AuthBaseURL     string
	DocumentBaseURL string
	VectorBaseURL   string
	QueryBaseURL    string
	EntryBaseURL    string
}

type MilvusConfig struct {
	Addr        string
	Username    string
	Password    string
	Collection  string
	VectorField string
	VectorDim   int
	VectorType  string
}

type StorageConfig struct {
	UploadPath   string
	MaxFileSize  int64
	AllowedTypes []string
}

type TracingConfig struct {
	Enabled        bool
	JaegerEndpoint string
	ServiceName    string
}

var cfg *Config

func Load() (*Config, error) {
	if cfg != nil {
		return cfg, nil
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/ekb/")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("EKB")

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, using environment variables or defaults")
	}

	cfg = &Config{
		Server: ServerConfig{
			Port:         getEnvOrDefault("SERVER_PORT", viper.GetString("server.port")),
			Mode:         getEnvOrDefault("SERVER_MODE", viper.GetString("server.mode")),
			ReadTimeout:  viper.GetDuration("server.read_timeout"),
			WriteTimeout: viper.GetDuration("server.write_timeout"),
		},
		Database: DatabaseConfig{
			Host:         getEnvOrDefault("DB_HOST", viper.GetString("database.host")),
			Port:         getEnvOrDefault("DB_PORT", viper.GetString("database.port")),
			Username:     getEnvOrDefault("DB_USERNAME", viper.GetString("database.username")),
			Password:     getEnvOrDefault("DB_PASSWORD", viper.GetString("database.password")),
			Database:     getEnvOrDefault("DB_NAME", viper.GetString("database.database")),
			MaxOpenConns: viper.GetInt("database.max_open_conns"),
			MaxIdleConns: viper.GetInt("database.max_idle_conns"),
			MaxLifetime:  viper.GetDuration("database.max_lifetime"),
		},
		Redis: RedisConfig{
			Host:     getEnvOrDefault("REDIS_HOST", viper.GetString("redis.host")),
			Port:     getEnvOrDefault("REDIS_PORT", viper.GetString("redis.port")),
			Password: getEnvOrDefault("REDIS_PASSWORD", viper.GetString("redis.password")),
			DB:       getEnvAsIntOrDefault("REDIS_DB", viper.GetInt("redis.db")),
		},
		JWT: JWTConfig{
			Secret:     getEnvOrDefault("JWT_SECRET", viper.GetString("jwt.secret")),
			ExpireTime: viper.GetDuration("jwt.expire_time"),
			Issuer:     getEnvOrDefault("JWT_ISSUER", viper.GetString("jwt.issuer")),
		},
		AI: AIConfig{
			OpenAIAPIKey:   getEnvOrDefault("OPENAI_API_KEY", viper.GetString("ai.openai_api_key")),
			EmbeddingModel: getEnvOrDefault("EMBEDDING_MODEL", viper.GetString("ai.embedding_model")),
			ChatModel:      getEnvOrDefault("CHAT_MODEL", viper.GetString("ai.chat_model")),
			MaxTokens:      viper.GetInt("ai.max_tokens"),
			Temperature:    viper.GetFloat64("ai.temperature"),
		},
		Ark: ArkConfig{
			APIKey:    getEnvOrDefault("ARK_API_KEY", viper.GetString("ark.api_key")),
			AccessKey: getEnvOrDefault("ARK_ACCESS_KEY", viper.GetString("ark.access_key")),
			SecretKey: getEnvOrDefault("ARK_SECRET_KEY", viper.GetString("ark.secret_key")),
			Model:     getEnvOrDefault("ARK_MODEL", viper.GetString("ark.model")),
			BaseURL:   getEnvOrDefault("ARK_BASE_URL", viper.GetString("ark.base_url")),
			Region:    getEnvOrDefault("ARK_REGION", viper.GetString("ark.region")),
		},
		Milvus: MilvusConfig{
			Addr:        getEnvOrDefault("MILVUS_ADDR", viper.GetString("milvus.addr")),
			Username:    getEnvOrDefault("MILVUS_USERNAME", viper.GetString("milvus.username")),
			Password:    getEnvOrDefault("MILVUS_PASSWORD", viper.GetString("milvus.password")),
			Collection:  getEnvOrDefault("MILVUS_COLLECTION", viper.GetString("milvus.collection")),
			VectorField: getEnvOrDefault("MILVUS_VECTOR_FIELD", viper.GetString("milvus.vector_field")),
			VectorDim:   getEnvAsIntOrDefault("MILVUS_VECTOR_DIM", viper.GetInt("milvus.vector_dim")),
			VectorType:  getEnvOrDefault("MILVUS_VECTOR_TYPE", viper.GetString("milvus.vector_type")),
		},
		Storage: StorageConfig{
			UploadPath:   getEnvOrDefault("UPLOAD_PATH", viper.GetString("storage.upload_path")),
			MaxFileSize:  getEnvAsInt64OrDefault("MAX_FILE_SIZE", viper.GetInt64("storage.max_file_size")),
			AllowedTypes: viper.GetStringSlice("storage.allowed_types"),
		},
		Tracing: TracingConfig{
			Enabled:        getEnvAsBoolOrDefault("TRACING_ENABLED", viper.GetBool("tracing.enabled")),
			JaegerEndpoint: getEnvOrDefault("JAEGER_ENDPOINT", viper.GetString("tracing.jaeger_endpoint")),
			ServiceName:    getEnvOrDefault("SERVICE_NAME", viper.GetString("tracing.service_name")),
		},
		RagQuery: RagQueryConfig{
			Model: getEnvOrDefault("RAG_MODEL", viper.GetString("rag_query.model")),
			Parameters: RagQueryParameters{
				Temperature: viper.GetFloat64("rag_query.parameters.temperature"),
				MaxTokens:   viper.GetInt("rag_query.parameters.max_tokens"),
			},
		},
		Gateway: GatewayConfig{
			AuthBaseURL:     getEnvOrDefault("GW_AUTH_BASE_URL", viper.GetString("gateway.auth_base_url")),
			DocumentBaseURL: getEnvOrDefault("GW_DOCUMENT_BASE_URL", viper.GetString("gateway.document_base_url")),
			VectorBaseURL:   getEnvOrDefault("GW_VECTOR_BASE_URL", viper.GetString("gateway.vector_base_url")),
			QueryBaseURL:    getEnvOrDefault("GW_QUERY_BASE_URL", viper.GetString("gateway.query_base_url")),
			EntryBaseURL:    getEnvOrDefault("GW_ENTRY_BASE_URL", viper.GetString("gateway.entry_base_url")),
		},
		RabbitMQ: RabbitMQConfig{
			URL:   getEnvOrDefault("RABBITMQ_URL", viper.GetString("rabbitmq.url")),
			Queue: getEnvOrDefault("RABBITMQ_QUEUE", viper.GetString("rabbitmq.queue")),
		},
	}

	return cfg, nil
}

func setDefaults() {
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "development")
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")

	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "3306")
	viper.SetDefault("database.username", "root")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.database", "ekb")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.max_lifetime", "5m")

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	viper.SetDefault("jwt.secret", "your-secret-key")
	viper.SetDefault("jwt.expire_time", "24h")
	viper.SetDefault("jwt.issuer", "enterprise-knowledge-base")

	viper.SetDefault("ai.embedding_model", "text-embedding-ada-002")
	viper.SetDefault("ai.chat_model", "gpt-3.5-turbo")
	viper.SetDefault("ai.max_tokens", 1000)
	viper.SetDefault("ai.temperature", 0.7)

	viper.SetDefault("ark.api_key", "")
	viper.SetDefault("ark.access_key", "")
	viper.SetDefault("ark.secret_key", "")
	viper.SetDefault("ark.model", "")
	viper.SetDefault("ark.base_url", "https://ark.cn-beijing.volces.com/api/v3")
	viper.SetDefault("ark.region", "cn-beijing")

	viper.SetDefault("milvus.addr", "localhost:19530")
	viper.SetDefault("milvus.username", "")
	viper.SetDefault("milvus.password", "")
	viper.SetDefault("milvus.collection", "eino_collection")
	viper.SetDefault("milvus.vector_field", "vector")
	viper.SetDefault("milvus.vector_dim", 1024)
	viper.SetDefault("milvus.vector_type", "float")

	viper.SetDefault("storage.upload_path", "./uploads")
	viper.SetDefault("storage.max_file_size", 10485760) // 10MB
	viper.SetDefault("storage.allowed_types", []string{".pdf", ".txt", ".doc", ".docx"})

	viper.SetDefault("tracing.enabled", false)
	viper.SetDefault("tracing.jaeger_endpoint", "http://localhost:14268/api/traces")
	viper.SetDefault("tracing.service_name", "enterprise-knowledge-base")

	viper.SetDefault("rag_query.model", "doubao-seed-1-6-251015")
	viper.SetDefault("rag_query.parameters.temperature", 0.7)
	viper.SetDefault("rag_query.parameters.max_tokens", 1024)

	viper.SetDefault("gateway.auth_base_url", "http://localhost:8081")
	viper.SetDefault("gateway.document_base_url", "http://localhost:8082")
	viper.SetDefault("gateway.vector_base_url", "http://localhost:8083")
	viper.SetDefault("gateway.query_base_url", "http://localhost:8084")
	viper.SetDefault("gateway.entry_base_url", "http://localhost:8080")

	viper.SetDefault("rabbitmq.url", "amqp://guest:guest@localhost:5672/")
	viper.SetDefault("rabbitmq.queue", "vector_processing_queue")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsInt64OrDefault(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func Get() *Config {
	if cfg == nil {
		config, err := Load()
		if err != nil {
			panic(fmt.Sprintf("failed to load config: %v", err))
		}
		cfg = config
	}
	return cfg
}
