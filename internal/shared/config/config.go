package config

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Logger   LoggerConfig
	Webhooks WebhooksConfig
	JWT      JWTConfig
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	Issuer        string
}

type AppConfig struct {
	Env string // local, development, staging, production
}

// IsLocal returns true if APP_ENV is "local"
func (c *AppConfig) IsLocal() bool {
	return c.Env == "local"
}

func (c *AppConfig) IsProduction() bool {
	return c.Env == "production"
}

type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type RedisConfig struct {
	Host              string
	Port              int
	Password          string
	DB                int
	PoolSize          int
	MinIdleConns      int
	MaxRetries        int
	DialTimeout       time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	StreamMaxLen      int64
	ConsumerGroup     string
	ConsumerBlockTime time.Duration

	// Worker pool
	WorkerPoolSize int

	// XCLAIM recovery
	ClaimMinIdleTime time.Duration
	ClaimInterval    time.Duration
	ClaimBatchSize   int64
	MaxRetryCount    int64
}

type LoggerConfig struct {
	Level       string
	Development bool
	Encoding    string
}

// WebhooksConfig holds webhook-related configuration for Meta platforms
type WebhooksConfig struct {
	Facebook  FacebookWebhookConfig
	Instagram InstagramWebhookConfig
	WhatsApp  WhatsAppWebhookConfig
}

type FacebookWebhookConfig struct {
	VerifyToken string
	AppSecret   string
}

type InstagramWebhookConfig struct {
	VerifyToken string
	AppSecret   string
}

type WhatsAppWebhookConfig struct {
	VerifyToken string
	AppSecret   string
}

// Load reads configuration from environment variables and config files
func Load() (*Config, error) {
	// Load .env file into OS environment (ignore if not found)
	_ = godotenv.Load()

	// Set defaults
	setDefaults()

	// Override with environment variables
	viper.AutomaticEnv()

	// Explicitly bind all nested keys (AutomaticEnv doesn't handle nested keys with Unmarshal)
	// App
	_ = viper.BindEnv("app.env", "APP_ENV")

	// Server
	_ = viper.BindEnv("server.host", "SERVER_HOST")
	_ = viper.BindEnv("server.port", "SERVER_PORT")
	_ = viper.BindEnv("server.readtimeout", "SERVER_READTIMEOUT")
	_ = viper.BindEnv("server.writetimeout", "SERVER_WRITETIMEOUT")
	_ = viper.BindEnv("server.idletimeout", "SERVER_IDLETIMEOUT")
	_ = viper.BindEnv("server.shutdowntimeout", "SERVER_SHUTDOWNTIMEOUT")

	// Database
	_ = viper.BindEnv("database.host", "DATABASE_HOST")
	_ = viper.BindEnv("database.port", "DATABASE_PORT")
	_ = viper.BindEnv("database.user", "DATABASE_USER")
	_ = viper.BindEnv("database.password", "DATABASE_PASSWORD")
	_ = viper.BindEnv("database.database", "DATABASE_DATABASE")
	_ = viper.BindEnv("database.sslmode", "DATABASE_SSLMODE")
	_ = viper.BindEnv("database.maxopenconns", "DATABASE_MAXOPENCONNS")
	_ = viper.BindEnv("database.maxidleconns", "DATABASE_MAXIDLECONNS")
	_ = viper.BindEnv("database.connmaxlifetime", "DATABASE_CONNMAXLIFETIME")
	_ = viper.BindEnv("database.connmaxidletime", "DATABASE_CONNMAXIDLETIME")

	// Redis
	_ = viper.BindEnv("redis.host", "REDIS_HOST")
	_ = viper.BindEnv("redis.port", "REDIS_PORT")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")
	_ = viper.BindEnv("redis.poolsize", "REDIS_POOLSIZE")
	_ = viper.BindEnv("redis.minidleconns", "REDIS_MINIDLECONNS")
	_ = viper.BindEnv("redis.maxretries", "REDIS_MAXRETRIES")
	_ = viper.BindEnv("redis.dialtimeout", "REDIS_DIALTIMEOUT")
	_ = viper.BindEnv("redis.readtimeout", "REDIS_READTIMEOUT")
	_ = viper.BindEnv("redis.writetimeout", "REDIS_WRITETIMEOUT")
	_ = viper.BindEnv("redis.streammaxlen", "REDIS_STREAMMAXLEN")
	_ = viper.BindEnv("redis.consumergroup", "REDIS_CONSUMERGROUP")
	_ = viper.BindEnv("redis.consumerblocktime", "REDIS_CONSUMERBLOCKTIME")
	_ = viper.BindEnv("redis.workerpoolsize", "REDIS_WORKERPOOLSIZE")
	_ = viper.BindEnv("redis.claimminidletime", "REDIS_CLAIMMINIDLETIME")
	_ = viper.BindEnv("redis.claiminterval", "REDIS_CLAIMINTERVAL")
	_ = viper.BindEnv("redis.claimbatchsize", "REDIS_CLAIMBATCHSIZE")
	_ = viper.BindEnv("redis.maxretrycount", "REDIS_MAXRETRYCOUNT")

	// Logger
	_ = viper.BindEnv("logger.level", "LOGGER_LEVEL")
	_ = viper.BindEnv("logger.development", "LOGGER_DEVELOPMENT")
	_ = viper.BindEnv("logger.encoding", "LOGGER_ENCODING")

	// Webhooks
	_ = viper.BindEnv("webhooks.facebook.verifytoken", "WEBHOOKS_FACEBOOK_VERIFYTOKEN")
	_ = viper.BindEnv("webhooks.facebook.appsecret", "WEBHOOKS_FACEBOOK_APPSECRET")
	_ = viper.BindEnv("webhooks.instagram.verifytoken", "WEBHOOKS_INSTAGRAM_VERIFYTOKEN")
	_ = viper.BindEnv("webhooks.instagram.appsecret", "WEBHOOKS_INSTAGRAM_APPSECRET")
	_ = viper.BindEnv("webhooks.whatsapp.verifytoken", "WEBHOOKS_WHATSAPP_VERIFYTOKEN")
	_ = viper.BindEnv("webhooks.whatsapp.appsecret", "WEBHOOKS_WHATSAPP_APPSECRET")

	// JWT
	_ = viper.BindEnv("jwt.accesssecret", "JWT_ACCESS_SECRET")
	_ = viper.BindEnv("jwt.refreshsecret", "JWT_REFRESH_SECRET")
	_ = viper.BindEnv("jwt.accessttl", "JWT_ACCESS_TTL")
	_ = viper.BindEnv("jwt.refreshttl", "JWT_REFRESH_TTL")
	_ = viper.BindEnv("jwt.issuer", "JWT_ISSUER")

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults() {
	// App
	viper.SetDefault("app.env", "local")

	// Server
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.readtimeout", 15*time.Second)
	viper.SetDefault("server.writetimeout", 15*time.Second)
	viper.SetDefault("server.idletimeout", 60*time.Second)
	viper.SetDefault("server.shutdowntimeout", 10*time.Second)

	// Database
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.database", "voronka")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.maxopenconns", 25)
	viper.SetDefault("database.maxidleconns", 5)
	viper.SetDefault("database.connmaxlifetime", 30*time.Minute)
	viper.SetDefault("database.connmaxidletime", 5*time.Minute)

	// Redis
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.poolsize", 10)
	viper.SetDefault("redis.minidleconns", 2)
	viper.SetDefault("redis.maxretries", 3)
	viper.SetDefault("redis.dialtimeout", 5*time.Second)
	viper.SetDefault("redis.readtimeout", 3*time.Second)
	viper.SetDefault("redis.writetimeout", 3*time.Second)
	viper.SetDefault("redis.streammaxlen", 10000)
	viper.SetDefault("redis.consumergroup", "voronka-workers")
	viper.SetDefault("redis.consumerblocktime", 2*time.Second)
	viper.SetDefault("redis.workerpoolsize", 10)
	viper.SetDefault("redis.claimminidletime", 60*time.Second)
	viper.SetDefault("redis.claiminterval", 30*time.Second)
	viper.SetDefault("redis.claimbatchsize", 50)
	viper.SetDefault("redis.maxretrycount", 3)

	// Logger
	viper.SetDefault("logger.level", "info")
	viper.SetDefault("logger.development", false)
	viper.SetDefault("logger.encoding", "json")

	// Webhooks
	viper.SetDefault("webhooks.facebook.verifytoken", "")
	viper.SetDefault("webhooks.facebook.appsecret", "")
	viper.SetDefault("webhooks.instagram.verifytoken", "")
	viper.SetDefault("webhooks.instagram.appsecret", "")
	viper.SetDefault("webhooks.whatsapp.verifytoken", "")
	viper.SetDefault("webhooks.whatsapp.appsecret", "")

	// JWT
	viper.SetDefault("jwt.accesssecret", "change-me-access-secret")
	viper.SetDefault("jwt.refreshsecret", "change-me-refresh-secret")
	viper.SetDefault("jwt.accessttl", 15*time.Minute)
	viper.SetDefault("jwt.refreshttl", 7*24*time.Hour)
	viper.SetDefault("jwt.issuer", "vortex")
}

// GetDSN returns PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// GetRedisAddr returns Redis address
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
