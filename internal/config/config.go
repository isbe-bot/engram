package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBind               = "127.0.0.1"
	DefaultPort               = 8787
	DefaultMaxBodyBytes       = int64(1 << 20) // 1 MiB
	DefaultRateLimitPerMinute = 0              // disabled by default for local sidecars
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Storage   StorageConfig   `yaml:"storage"`
	Ingestion IngestionConfig `yaml:"ingestion"`
	Quality   QualityConfig   `yaml:"quality"`
}

type ServerConfig struct {
	Bind               string `yaml:"bind"`
	Port               int    `yaml:"port"`
	APIKey             string `yaml:"api_key"`
	MaxBodyBytes       int64  `yaml:"max_body_bytes"`
	RateLimitPerMinute int    `yaml:"rate_limit_per_minute"`
}

type StorageConfig struct {
	SQLitePath       string `yaml:"sqlite_path"`
	QdrantURL        string `yaml:"qdrant_url"`
	QdrantCollection string `yaml:"qdrant_collection"`
}

type IngestionConfig struct {
	MaxBatchSize int `yaml:"max_batch_size"`
	WorkerCount  int `yaml:"worker_count"`
}

type QualityConfig struct {
	EvalInterval string `yaml:"eval_interval"`
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	cfg.ApplyEnv()
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func Defaults() Config {
	var cfg Config
	cfg.Server.Bind = DefaultBind
	cfg.Server.Port = DefaultPort
	cfg.Server.MaxBodyBytes = DefaultMaxBodyBytes
	cfg.Server.RateLimitPerMinute = DefaultRateLimitPerMinute
	cfg.Storage.SQLitePath = "./engram.sqlite"
	cfg.Ingestion.MaxBatchSize = 200
	cfg.Ingestion.WorkerCount = 1
	cfg.Quality.EvalInterval = "24h"
	return cfg
}

func (c *Config) ApplyEnv() {
	setString(&c.Server.Bind, "ENGRAM_SERVER_BIND")
	setInt(&c.Server.Port, "ENGRAM_SERVER_PORT")
	setString(&c.Server.APIKey, "ENGRAM_API_KEY")
	setInt64(&c.Server.MaxBodyBytes, "ENGRAM_MAX_BODY_BYTES")
	setInt(&c.Server.RateLimitPerMinute, "ENGRAM_RATE_LIMIT_PER_MINUTE")
	setString(&c.Storage.SQLitePath, "ENGRAM_SQLITE_PATH")
	setString(&c.Storage.QdrantURL, "ENGRAM_QDRANT_URL")
	setString(&c.Storage.QdrantCollection, "ENGRAM_QDRANT_COLLECTION")
	setInt(&c.Ingestion.MaxBatchSize, "ENGRAM_MAX_BATCH_SIZE")
	setInt(&c.Ingestion.WorkerCount, "ENGRAM_WORKER_COUNT")
	setString(&c.Quality.EvalInterval, "ENGRAM_QUALITY_EVAL_INTERVAL")
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.Bind) == "" {
		return fmt.Errorf("server.bind is required")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if c.Server.MaxBodyBytes <= 0 {
		return fmt.Errorf("server.max_body_bytes must be > 0")
	}
	if c.Server.RateLimitPerMinute < 0 {
		return fmt.Errorf("server.rate_limit_per_minute must be >= 0")
	}
	if strings.TrimSpace(c.Storage.SQLitePath) == "" {
		return fmt.Errorf("storage.sqlite_path is required")
	}
	if strings.TrimSpace(c.Storage.QdrantURL) != "" && strings.TrimSpace(c.Storage.QdrantCollection) == "" {
		return fmt.Errorf("storage.qdrant_collection is required when storage.qdrant_url is set")
	}
	if c.Ingestion.MaxBatchSize <= 0 {
		return fmt.Errorf("ingestion.max_batch_size must be > 0")
	}
	if c.Ingestion.WorkerCount <= 0 {
		return fmt.Errorf("ingestion.worker_count must be > 0")
	}
	return nil
}

func setString(target *string, key string) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		*target = v
	}
}

func setInt(target *int, key string) {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			*target = v
		}
	}
}

func setInt64(target *int64, key string) {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			*target = v
		}
	}
}
