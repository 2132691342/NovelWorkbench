// Package infra 负责配置加载与各基础设施连接的初始化（Postgres/Redis/RabbitMQ/MinIO/Manticore）。
package infra

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 全局配置聚合（YAML 加载 + 环境变量覆盖，启动期 fail fast 校验）。
type Config struct {
	Server   ServerConfig   `yaml:"server"`   // HTTP 服务
	JWT      JWTConfig      `yaml:"jwt"`      // 令牌
	Postgres PostgresConfig `yaml:"postgres"` // 数据库
	Redis    RedisConfig    `yaml:"redis"`    // 缓存/令牌黑名单
	RabbitMQ RabbitMQConfig `yaml:"rabbitmq"` // 消息队列
	MinIO    MinIOConfig    `yaml:"minio"`    // 对象存储
	Manticore ManticoreConfig `yaml:"manticore"` // 全文检索
	AI       AIConfig       `yaml:"ai"`       // 模型网关
	App      AppConfig      `yaml:"app"`      // 业务参数
}

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	Host            string `yaml:"host"`              // 监听地址
	Port            int    `yaml:"port"`              // 监听端口
	Mode            string `yaml:"mode"`              // gin 运行模式
	ReadTimeoutSec  int    `yaml:"read_timeout_sec"`  // 读超时（秒）
	WriteTimeoutSec int    `yaml:"write_timeout_sec"` // 写超时（秒），0 为不限（SSE）
}

// JWTConfig 令牌配置。
type JWTConfig struct {
	Secret      string `yaml:"secret"`       // 签名密钥
	ExpireHours int    `yaml:"expire_hours"` // 有效期（小时）
}

// PostgresConfig 数据库连接配置。
type PostgresConfig struct {
	Host              string `yaml:"host"`                 // 主机
	Port              int    `yaml:"port"`                 // 端口
	User              string `yaml:"user"`                 // 用户
	Password          string `yaml:"password"`             // 密码
	DBName            string `yaml:"dbname"`               // 库名
	SSLMode           string `yaml:"sslmode"`              // SSL 模式
	MaxOpenConns      int    `yaml:"max_open_conns"`       // 最大打开连接
	MaxIdleConns      int    `yaml:"max_idle_conns"`       // 最大空闲连接
	ConnMaxLifetimeMin int   `yaml:"conn_max_lifetime_min"` // 连接最长存活（分钟）
}

// RedisConfig 缓存配置。
type RedisConfig struct {
	Addr     string `yaml:"addr"`     // 地址
	Password string `yaml:"password"` // 密码
	DB       int    `yaml:"db"`       // 库号
}

// RabbitMQConfig 消息队列配置。
type RabbitMQConfig struct {
	URL           string `yaml:"url"`             // amqp 连接串
	QueueDocIndex string `yaml:"queue_doc_index"` // 文档索引队列名
}

// MinIOConfig 对象存储配置。
type MinIOConfig struct {
	Endpoint  string `yaml:"endpoint"`   // S3 端点
	AccessKey string `yaml:"access_key"` // AccessKey
	SecretKey string `yaml:"secret_key"` // SecretKey
	Bucket    string `yaml:"bucket"`     // 桶名
	UseSSL    bool   `yaml:"use_ssl"`    // 是否 SSL
}

// ManticoreConfig 全文检索配置。
type ManticoreConfig struct {
	BaseURL    string `yaml:"base_url"`    // HTTP JSON API 地址
	TableChunks string `yaml:"table_chunks"` // 切片全文表名
}

// AIConfig 模型网关配置（OpenAI 兼容）。
type AIConfig struct {
	BaseURL        string  `yaml:"base_url"`        // OpenAI 兼容网关地址
	APIKey         string  `yaml:"api_key"`         // API Key
	ChatModel      string  `yaml:"chat_model"`      // 对话模型名
	EmbeddingModel string  `yaml:"embedding_model"` // 向量模型名
	EmbeddingDim   int     `yaml:"embedding_dim"`   // 向量维度
	Temperature    float64 `yaml:"temperature"`     // 采样温度
	MaxTokens      int     `yaml:"max_tokens"`      // 单次输出上限
	TimeoutSec     int     `yaml:"timeout_sec"`     // 调用超时（秒）
	HistoryCount   int     `yaml:"history_count"`   // 历史消息注入默认条数
}

// AppConfig 业务参数配置。
type AppConfig struct {
	UploadMaxMB  int `yaml:"upload_max_mb"`  // 上传大小上限（MB）
	ChunkSize    int `yaml:"chunk_size"`     // 知识库切片字符数
	ChunkOverlap int `yaml:"chunk_overlap"`  // 切片重叠字符数
	RAGTopK      int `yaml:"rag_top_k"`      // RAG 默认召回条数
}

// LoadConfig 加载 YAML 配置并施加环境变量覆盖；configPath 为空时使用 configs/config.yaml。
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "configs/config.yaml"
	}
	rawBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", configPath, err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(rawBytes, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", configPath, err)
	}
	cfg.applyEnvOverride()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applyEnvOverride 以 NW_ 前缀环境变量覆盖敏感/部署相关配置（集中一处，禁止散落 Getenv）。
func (c *Config) applyEnvOverride() {
	setStr := func(key string, target *string) {
		if value := os.Getenv(key); value != "" {
			*target = value
		}
	}
	setStr("NW_JWT_SECRET", &c.JWT.Secret)
	setStr("NW_POSTGRES_HOST", &c.Postgres.Host)
	setStr("NW_POSTGRES_PASSWORD", &c.Postgres.Password)
	setStr("NW_REDIS_ADDR", &c.Redis.Addr)
	setStr("NW_REDIS_PASSWORD", &c.Redis.Password)
	setStr("NW_RABBITMQ_URL", &c.RabbitMQ.URL)
	setStr("NW_MINIO_ENDPOINT", &c.MinIO.Endpoint)
	setStr("NW_MINIO_ACCESS_KEY", &c.MinIO.AccessKey)
	setStr("NW_MINIO_SECRET_KEY", &c.MinIO.SecretKey)
	setStr("NW_MANTICORE_BASE_URL", &c.Manticore.BaseURL)
	setStr("NW_AI_BASE_URL", &c.AI.BaseURL)
	setStr("NW_AI_API_KEY", &c.AI.APIKey)
	setStr("NW_AI_CHAT_MODEL", &c.AI.ChatModel)
	setStr("NW_AI_EMBEDDING_MODEL", &c.AI.EmbeddingModel)
	if value := os.Getenv("NW_SERVER_PORT"); value != "" {
		if port, err := strconv.Atoi(value); err == nil {
			c.Server.Port = port
		}
	}
}

// validate 启动期校验关键字段，缺失即失败（fail fast）。
func (c *Config) validate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret is required")
	}
	if c.Postgres.Host == "" || c.Postgres.DBName == "" {
		return fmt.Errorf("postgres.host/dbname is required")
	}
	if c.AI.BaseURL == "" || c.AI.ChatModel == "" {
		return fmt.Errorf("ai.base_url/chat_model is required")
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080 // 默认端口
	}
	if c.AI.HistoryCount <= 0 {
		c.AI.HistoryCount = 5 // 默认注入最近 5 条历史
	}
	if c.App.RAGTopK <= 0 {
		c.App.RAGTopK = 6 // 默认召回 6 条
	}
	return nil
}

// ServerAddr 返回监听地址（host:port，IPv6 自动加方括号）。
func (c *Config) ServerAddr() string {
	return net.JoinHostPort(c.Server.Host, strconv.Itoa(c.Server.Port))
}

// JWTExpireTTL 返回令牌有效期。
func (c *Config) JWTExpireTTL() time.Duration {
	return time.Duration(c.JWT.ExpireHours) * time.Hour
}
