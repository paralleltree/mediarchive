package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Twitter struct {
		ConsumerKey    string `yaml:"consumer_key"`
		ConsumerSecret string `yaml:"consumer_secret"`
		AccessKey      string `yaml:"access_key"`
		AccessSecret   string `yaml:"access_secret"`
	} `yaml:"twitter"`
}

func LoadConfig(configPath string) (*AppConfig, error) {
	conf := &AppConfig{}
	f, err := os.Open(configPath)
	if err != nil {
		return conf, fmt.Errorf("open file: %w", err)
	}
	bytes, err := io.ReadAll(f)
	if err != nil {
		return conf, fmt.Errorf("read file: %w", err)
	}
	if err := yaml.Unmarshal(bytes, conf); err != nil {
		return conf, fmt.Errorf("unmarshal yaml: %w", err)
	}
	return conf, nil
}
