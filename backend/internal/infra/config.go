// Package infra 负责配置加载与各基础设施连接的初始化（Postgres/Redis/RabbitMQ/MinIO/Manticore）。
package infra

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 全局配置聚合（viper 加载 YAML + NW_ 前缀环境变量覆盖，启动期 fail fast 校验）。
type Config struct {
	Server    ServerConfig    `yaml:"server" mapstructure:"server"`       // HTTP 服务
	JWT       JWTConfig       `yaml:"jwt" mapstructure:"jwt"`             // 令牌
	Postgres  PostgresConfig  `yaml:"postgres" mapstructure:"postgres"`   // 数据库
	Redis     RedisConfig     `yaml:"redis" mapstructure:"redis"`         // 缓存/令牌黑名单
	RabbitMQ  RabbitMQConfig  `yaml:"rabbitmq" mapstructure:"rabbitmq"`   // 消息队列
	MinIO     MinIOConfig     `yaml:"minio" mapstructure:"minio"`         // 对象存储
	Manticore ManticoreConfig `yaml:"manticore" mapstructure:"manticore"` // 全文检索
	AI        AIConfig        `yaml:"ai" mapstructure:"ai"`               // 模型网关
	App       AppConfig       `yaml:"app" mapstructure:"app"`             // 业务参数
}

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	Host            string `yaml:"host" mapstructure:"host"`                           // 监听地址
	Port            int    `yaml:"port" mapstructure:"port"`                           // 监听端口
	Mode            string `yaml:"mode" mapstructure:"mode"`                           // gin 运行模式
	ReadTimeoutSec  int    `yaml:"read_timeout_sec" mapstructure:"read_timeout_sec"`   // 读超时（秒）
	WriteTimeoutSec int    `yaml:"write_timeout_sec" mapstructure:"write_timeout_sec"` // 写超时（秒），0 为不限（SSE）
}

// JWTConfig 令牌配置。
type JWTConfig struct {
	Secret      string `yaml:"secret" mapstructure:"secret"`             // 签名密钥
	ExpireHours int    `yaml:"expire_hours" mapstructure:"expire_hours"` // 有效期（小时）
}

// PostgresConfig 数据库连接配置。
type PostgresConfig struct {
	Host               string `yaml:"host" mapstructure:"host"`                                   // 主机
	Port               int    `yaml:"port" mapstructure:"port"`                                   // 端口
	User               string `yaml:"user" mapstructure:"user"`                                   // 用户
	Password           string `yaml:"password" mapstructure:"password"`                           // 密码
	DBName             string `yaml:"dbname" mapstructure:"dbname"`                               // 库名
	SSLMode            string `yaml:"sslmode" mapstructure:"sslmode"`                             // SSL 模式
	MaxOpenConns       int    `yaml:"max_open_conns" mapstructure:"max_open_conns"`               // 最大打开连接
	MaxIdleConns       int    `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`               // 最大空闲连接
	ConnMaxLifetimeMin int    `yaml:"conn_max_lifetime_min" mapstructure:"conn_max_lifetime_min"` // 连接最长存活（分钟）
}

// RedisConfig 缓存配置。
type RedisConfig struct {
	Addr     string `yaml:"addr" mapstructure:"addr"`         // 地址
	Password string `yaml:"password" mapstructure:"password"` // 密码
	DB       int    `yaml:"db" mapstructure:"db"`             // 库号
}

// RabbitMQConfig 消息队列配置。
type RabbitMQConfig struct {
	URL           string `yaml:"url" mapstructure:"url"`                         // amqp 连接串
	QueueDocIndex string `yaml:"queue_doc_index" mapstructure:"queue_doc_index"` // 文档索引队列名
}

// MinIOConfig 对象存储配置。
type MinIOConfig struct {
	Endpoint  string `yaml:"endpoint" mapstructure:"endpoint"`     // S3 端点
	AccessKey string `yaml:"access_key" mapstructure:"access_key"` // AccessKey
	SecretKey string `yaml:"secret_key" mapstructure:"secret_key"` // SecretKey
	Bucket    string `yaml:"bucket" mapstructure:"bucket"`         // 桶名
	UseSSL    bool   `yaml:"use_ssl" mapstructure:"use_ssl"`       // 是否 SSL
}

// ManticoreConfig 全文检索配置。
type ManticoreConfig struct {
	BaseURL     string `yaml:"base_url" mapstructure:"base_url"`         // HTTP JSON API 地址
	TableChunks string `yaml:"table_chunks" mapstructure:"table_chunks"` // 切片全文表名
}

// AIConfig 模型网关配置（仅作为供应商表的出厂默认：为空时播种一条默认供应商）。
type AIConfig struct {
	ProtocolCode string  `yaml:"protocol_code" mapstructure:"protocol_code"` // 协议：openai/anthropic
	BaseURL      string  `yaml:"base_url" mapstructure:"base_url"`           // 网关地址（OpenAI 含 /v1；Anthropic 为根地址）
	APIKey       string  `yaml:"api_key" mapstructure:"api_key"`             // API Key
	ChatModel    string  `yaml:"chat_model" mapstructure:"chat_model"`       // 对话模型名
	Temperature  float64 `yaml:"temperature" mapstructure:"temperature"`     // 采样温度
	MaxTokens    int     `yaml:"max_tokens" mapstructure:"max_tokens"`       // 单次输出上限
	TimeoutSec   int     `yaml:"timeout_sec" mapstructure:"timeout_sec"`     // 调用超时（秒）
	HistoryCount int     `yaml:"history_count" mapstructure:"history_count"` // 历史消息注入默认条数
}

// AppConfig 业务参数配置。
type AppConfig struct {
	UploadMaxMB   int    `yaml:"upload_max_mb" mapstructure:"upload_max_mb"`   // 上传大小上限（MB）
	ChunkSize     int    `yaml:"chunk_size" mapstructure:"chunk_size"`         // 知识库切片字符数
	ChunkOverlap  int    `yaml:"chunk_overlap" mapstructure:"chunk_overlap"`   // 切片重叠字符数
	RAGTopK       int    `yaml:"rag_top_k" mapstructure:"rag_top_k"`           // RAG 默认召回条数
	AdminPassword string `yaml:"admin_password" mapstructure:"admin_password"` // 引导管理员初始密码（仅用户表为空时生效）
	BuildVersion  string `yaml:"build_version" mapstructure:"build_version"`   // 构建版本（发布时注入，缺省 dev）
}

// LoadConfig 加载 YAML 配置并施加 NW_ 前缀环境变量覆盖（如 postgres.host → NW_POSTGRES_HOST）。
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "configs/config.yaml"
	}
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetEnvPrefix("NW")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", configPath, err)
	}
	// 显式绑定全部键，使环境变量参与 Unmarshal（AutomaticEnv 与 AllSettings 合并存在盲区）
	for _, key := range v.AllKeys() {
		_ = v.BindEnv(key)
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", configPath, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validate 启动期校验关键字段，缺失即失败（fail fast）；补齐业务默认值。
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
	if c.AI.ProtocolCode == "" {
		c.AI.ProtocolCode = "openai" // 默认 OpenAI 兼容协议
	}
	if c.App.BuildVersion == "" {
		c.App.BuildVersion = "dev" // 未注入构建版本时的兜底
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
	if c.App.AdminPassword == "" {
		c.App.AdminPassword = "Admin@123456" // 引导默认密码，生产必须覆盖
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
