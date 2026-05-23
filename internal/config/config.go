package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Bind string `yaml:"bind"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Storage struct {
		SQLitePath string `yaml:"sqlite_path"`
		QdrantURL  string `yaml:"qdrant_url"`
		RedisAddr  string `yaml:"redis_addr"`
	} `yaml:"storage"`
	Ingestion struct {
		MaxBatchSize int `yaml:"max_batch_size"`
		WorkerCount  int `yaml:"worker_count"`
	} `yaml:"ingestion"`
	Quality struct {
		EvalInterval string `yaml:"eval_interval"`
	} `yaml:"quality"`
}

func Load(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.Bind) == "" {
		return fmt.Errorf("server.bind is required")
	}
	if c.Server.Port <= 0 {
		return fmt.Errorf("server.port must be > 0")
	}
	if strings.TrimSpace(c.Storage.SQLitePath) == "" {
		return fmt.Errorf("storage.sqlite_path is required")
	}
	return nil
}
