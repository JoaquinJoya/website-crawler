package config

import (
	"os"
	"strconv"
	"time"
)

type Settings struct {
	Server     ServerConfig
	Crawler    CrawlerConfig
	AI         AIConfig
	RateLimit  RateLimitConfig
	Colly      CollyConfig
	Monitoring MonitoringConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type CrawlerConfig struct {
	UserAgent           string
	Timeout             time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration
	TLSHandshakeTimeout time.Duration
	MaxRetries          int
}

type AIConfig struct {
	PythonPath string
	ScriptPath string
}

type RateLimitConfig struct {
	RequestsPerSecond float64
	MaxConcurrent     int
}

type CollyConfig struct {
	Enabled            bool
	UserAgent          string
	Delay              time.Duration
	RandomDelay        time.Duration
	Parallelism        int
	DomainGlob         string
	RespectRobotsTxt   bool
	AllowURLRevisit    bool
	CacheDir           string
	DebugMode          bool
	Async              bool
	CacheEnabled       bool
	CacheTTL           time.Duration
}

type MonitoringConfig struct {
	MultiLangEnabled     bool
	ChangeDetection      bool
	ContentHashTracking  bool
	AlertWebhookURL      string
	ComparisonThreshold  float64
	MonitoringInterval   time.Duration
}

func Load() *Settings {
	return &Settings{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8081"),
			ReadTimeout:  getDurationEnv("READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getDurationEnv("WRITE_TIMEOUT", 30*time.Second),
		},
		Crawler: CrawlerConfig{
			UserAgent:           getEnv("USER_AGENT", "Mozilla/5.0 (compatible; WebCrawler/1.0)"),
			Timeout:             getDurationEnv("CRAWLER_TIMEOUT", 30*time.Second),
			MaxIdleConns:        getIntEnv("MAX_IDLE_CONNS", 200),
			MaxIdleConnsPerHost: getIntEnv("MAX_IDLE_CONNS_PER_HOST", 50),
			MaxConnsPerHost:     getIntEnv("MAX_CONNS_PER_HOST", 100),
			IdleConnTimeout:     getDurationEnv("IDLE_CONN_TIMEOUT", 30*time.Second),
			TLSHandshakeTimeout: getDurationEnv("TLS_HANDSHAKE_TIMEOUT", 10*time.Second),
			MaxRetries:          getIntEnv("MAX_RETRIES", 3),
		},
		AI: AIConfig{
			PythonPath: getEnv("PYTHON_PATH", "./venv/bin/python"),
			ScriptPath: getEnv("AI_SCRIPT_PATH", "ai_processor.py"),
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: getFloatEnv("REQUESTS_PER_SECOND", 5.0),
			MaxConcurrent:     getIntEnv("MAX_CONCURRENT", 5),
		},
		Colly: CollyConfig{
			Enabled:            getBoolEnv("COLLY_ENABLED", true),
			UserAgent:          getEnv("COLLY_USER_AGENT", "Mozilla/5.0 (compatible; WebCrawler-Colly/1.0)"),
			Delay:              getDurationEnv("COLLY_DELAY", 200*time.Millisecond),
			RandomDelay:        getDurationEnv("COLLY_RANDOM_DELAY", 100*time.Millisecond),
			Parallelism:        getIntEnv("COLLY_PARALLELISM", 5),
			DomainGlob:         getEnv("COLLY_DOMAIN_GLOB", "*"),
			RespectRobotsTxt:   getBoolEnv("COLLY_RESPECT_ROBOTS", false),
			AllowURLRevisit:    getBoolEnv("COLLY_ALLOW_REVISIT", false),
			CacheDir:           getEnv("COLLY_CACHE_DIR", "./cache"),
			DebugMode:          getBoolEnv("COLLY_DEBUG", false),
			Async:              getBoolEnv("COLLY_ASYNC", true),
			CacheEnabled:       getBoolEnv("COLLY_CACHE_ENABLED", true),
			CacheTTL:           getDurationEnv("COLLY_CACHE_TTL", 24*time.Hour),
		},
		Monitoring: MonitoringConfig{
			MultiLangEnabled:     getBoolEnv("MONITORING_MULTILANG", true),
			ChangeDetection:      getBoolEnv("MONITORING_CHANGE_DETECTION", true),
			ContentHashTracking:  getBoolEnv("MONITORING_CONTENT_HASH", true),
			AlertWebhookURL:      getEnv("MONITORING_WEBHOOK_URL", ""),
			ComparisonThreshold:  getFloatEnv("MONITORING_THRESHOLD", 0.8),
			MonitoringInterval:   getDurationEnv("MONITORING_INTERVAL", 1*time.Hour),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getFloatEnv(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}