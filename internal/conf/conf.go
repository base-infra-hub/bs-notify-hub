package conf

import (
	"crypto/rsa"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"bs-notify-hub/pkg/jwtutil"

	"gopkg.in/yaml.v3"
)

// Config 全局配置结构体
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Dispatcher DispatcherConfig `yaml:"dispatcher"`
	System     SystemConfig     `yaml:"system"`
	Database   DatabaseConfig   `yaml:"database"`
	Redis      RedisConfig      `yaml:"redis"`
	Ticket     TicketConfig     `yaml:"ticket"`
	Auth       AuthConfig       `yaml:"auth"`

	// 运行时缓存，不参与 yaml 解析
	rsaPublicKey *rsa.PublicKey
}

// AuthConfig 鉴权相关配置
type AuthConfig struct {
	// RSAPublicKey RSA 公钥，支持完整 PEM（含头尾）和裸 Base64 两种格式
	RSAPublicKey string `yaml:"rsa_public_key"`
	// ServiceTag JWT 中 tag 字段必须与此值一致，防止跨服务令牌复用
	// 默认值：BS-Notify-Hub
	ServiceTag string      `yaml:"service_tag"`
	Admin      AdminConfig `yaml:"admin"`
}

// AdminConfig 管理员 Web 控制台登录会话配置
type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// SessionTTLSeconds 登录状态 Session 有效秒数（默认 7200 = 2 小时）
	SessionTTLSeconds int `yaml:"session_ttl_seconds"`
}

// GetRSAPublicKey 获取已解析的 RSA 公钥（启动时初始化）
func (c *Config) GetRSAPublicKey() *rsa.PublicKey {
	return c.rsaPublicKey
}

// SystemConfig 系统默认参数配置
type SystemConfig struct {
	TenantNotifyTTLSeconds int64 `yaml:"tenant_notify_ttl_seconds"`
	UserNotifyTTLSeconds   int64 `yaml:"user_notify_ttl_seconds"`
	EnableNotifyCleanup    bool  `yaml:"enable_notify_cleanup"`
}

// ServerConfig 服务端口相关配置
type ServerConfig struct {
	HTTPPort int `yaml:"http_port"`
	GRPCPort int `yaml:"grpc_port"`
}

// DispatcherConfig 调度器配置
// 当前仅支持 local 模式；cluster 模式预留，后续扩展
type DispatcherConfig struct {
	Mode string `yaml:"mode"` // 目前固定 local
}

// DatabaseConfig 数据库预留配置
type DatabaseConfig struct {
	Postgres PostgresConfig `yaml:"postgres"`
}

// PostgresConfig Postgres 详细配置
type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

func (c PostgresConfig) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.DBName,
	}

	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()

	return u.String()
}

// RedisConfig Redis 预留配置
type RedisConfig struct {
	Addrs    []string `yaml:"addrs"`
	Password string   `yaml:"password"`
	DB       int      `yaml:"db"`
}

type TicketConfig struct {
	ExpireSeconds int    `yaml:"expire_seconds"`
	AseKey        string `yaml:"ase_key"`
}

var (
	globalConfig *Config
	configOnce   sync.Once
)

// LoadConfig 加载配置，启动时调用，返回错误
func LoadConfig() (*Config, error) {
	var loadErr error
	configOnce.Do(func() {
		globalConfig = &Config{}
		if err := globalConfig.load(); err != nil {
			loadErr = err
		}
	})
	if loadErr != nil {
		return nil, loadErr
	}
	return globalConfig, nil
}

// PrintConfig 打印当前配置（敏感字段脱敏）
func (c *Config) PrintConfig() {
	log.Println("┌─────────────────────────── 当前配置 ───────────────────────────")
	log.Printf("│ server.http_port           : %d", c.Server.HTTPPort)
	log.Printf("│ server.grpc_port           : %d", c.Server.GRPCPort)
	log.Println("│")
	log.Printf("│ system.tenant_notify_ttl   : %d", c.System.TenantNotifyTTLSeconds)
	log.Printf("│ system.user_notify_ttl     : %d", c.System.UserNotifyTTLSeconds)
	log.Printf("│ system.enable_notify_clean : %v", c.System.EnableNotifyCleanup)
	log.Println("│")
	log.Printf("│ ticket.expire_seconds      : %d", c.Ticket.ExpireSeconds)
	log.Printf("│ ticket.ase_key             : %s", maskSecret(c.Ticket.AseKey))
	log.Println("│")
	log.Printf("│ dispatcher.mode            : %s", c.Dispatcher.Mode)
	log.Println("│")
	log.Printf("│ database.postgres.host     : %s", c.Database.Postgres.Host)
	log.Printf("│ database.postgres.port     : %d", c.Database.Postgres.Port)
	log.Printf("│ database.postgres.user     : %s", c.Database.Postgres.User)
	log.Printf("│ database.postgres.password : %s", maskSecret(c.Database.Postgres.Password))
	log.Printf("│ database.postgres.dbname   : %s", c.Database.Postgres.DBName)
	log.Println("│")
	log.Printf("│ redis.addrs                : %v", c.Redis.Addrs)
	log.Printf("│ redis.password             : %s", maskSecret(c.Redis.Password))
	log.Printf("│ redis.db                   : %d", c.Redis.DB)
	log.Println("│")
	rsaStatus := "未配置"
	if c.rsaPublicKey != nil {
		rsaStatus = fmt.Sprintf("已加载 (%d bit)", c.rsaPublicKey.N.BitLen())
	}
	log.Printf("│ auth.rsa_public_key        : %s", rsaStatus)
	log.Printf("│ auth.admin.username        : %s", c.Auth.Admin.Username)
	log.Printf("│ auth.admin.password        : %s", maskSecret(c.Auth.Admin.Password))
	log.Printf("│ auth.admin.session_ttl     : %ds", c.Auth.Admin.SessionTTLSeconds)
	log.Println("└────────────────────────────────────────────────────────────────")
}

func maskSecret(s string) string {
	if len(s) == 0 {
		return "(空)"
	}
	if len(s) <= 2 {
		return "**"
	}
	return s[:2] + "**"
}

// GetConfig 获取已加载的配置单例，仅在 LoadConfig 成功后调用
func GetConfig() *Config {
	if globalConfig == nil {
		log.Fatal("[配置错误] 配置尚未加载，请先调用 LoadConfig")
	}
	return globalConfig
}

// load 加载配置：仅读取可执行文件同目录下的 config.yaml，不存在则报错
func (c *Config) load() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法获取可执行文件路径: %w", err)
	}
	externalPath := filepath.Join(filepath.Dir(exePath), "config.yaml")

	data, err := os.ReadFile(externalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("配置文件不存在: %s", externalPath)
		}
		return fmt.Errorf("读取配置文件 %s 失败: %w", externalPath, err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("解析配置文件 %s 失败: %w", externalPath, err)
	}

	// 解析 RSA 公钥（如果配置了）
	if c.Auth.RSAPublicKey != "" {
		pubKey, err := jwtutil.ParseRSAPublicKey(c.Auth.RSAPublicKey)
		if err != nil {
			return fmt.Errorf("解析 RSA 公钥失败: %w", err)
		}
		c.rsaPublicKey = pubKey
		log.Printf("[配置] RSA 公钥解析成功 (%d bit)", pubKey.N.BitLen())
	} else {
		log.Printf("[配置] 警告：未配置 RSA 公钥，JWT 鉴权将拒绝所有请求")
	}

	if c.Auth.ServiceTag == "" {
		return fmt.Errorf("auth.service_tag 不得为空，必须填写本服务的 JWT tag 标识（如 \"BS-Notify-Hub\"），防止其他服务的 JWT 越权访问")
	}

	log.Printf("[配置] 已加载配置文件: %s", externalPath)
	return nil
}
