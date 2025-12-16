package main

import (
	"log"
	"os"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Output OutputConfig `yaml:"output"`
}

type OutputConfig struct {
	Mode    string `yaml:"mode"`
	APIKey  string `yaml:"api-key"`
	Model   string `yaml:"model"`
	BaseURL string `yaml:"baseurl"`
}

const defaultConfigPath = "config.yml"

// LoadConfig 从给定路径读取 YAML 配置文件，并在出现错误时返回空配置，方便调用方做安全降级。
func LoadConfig(path string, logger *log.Logger) *Config {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) && logger != nil {
			logger.Printf("error reading config file %s: %v", path, err)
		}
		return &Config{}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		if logger != nil {
			logger.Printf("error parsing config file %s: %v", path, err)
		}
		return &Config{}
	}
	return &cfg
}
