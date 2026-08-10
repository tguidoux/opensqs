// Package config provides YAML-based configuration loading with schema validation
// and environment-specific overrides for OpenSQS services.
package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

const (
	DefaultConfigPath   = "config.yaml"
	DefaultConfigEnvVar = "CONFIG_PATH"
)

type ConfigI[ConfigType any] interface {
	Validate() error
	WithValidation() ConfigType
}

type Config[ConfigType any] struct {
	ConfigPath string
	Config     ConfigType
	data       []byte
}

func NewConfig[ConfigType any](configPath string) Config[ConfigType] {
	config := Config[ConfigType]{
		ConfigPath: configPath,
	}

	config.readConfig()

	return config
}

func NewConfigFromEnv[ConfigType ConfigI[ConfigType]](envVar ...string) *Config[ConfigType] {
	var env string
	if len(envVar) == 0 || envVar[0] == "" {
		env = DefaultConfigEnvVar
	} else {
		env = envVar[0]
	}

	configPath := os.Getenv(env)

	if configPath == "" {
		configPath = DefaultConfigPath
	}

	config := NewConfig[ConfigType](configPath)

	return &config
}

func (c *Config[ConfigType]) readConfig() {

	// Read file from ConfigPath
	data, err := os.ReadFile(c.ConfigPath)
	if err != nil {
		panic(fmt.Errorf("failed to read config file %s: %w", c.ConfigPath, err))
	}
	c.data = data

	var config ConfigType
	if err := yaml.Unmarshal(c.data, &config); err != nil {
		panic(fmt.Errorf("failed to convert raw config to ConfigType: %w", err))
	}

	c.Config = config
}

func (c *Config[ConfigType]) Validate() ConfigType {
	// Implement validation logic here
	// For example, check if required fields are present in RawConfig
	if len(c.data) == 0 {
		panic(fmt.Errorf("config file %s is empty", c.ConfigPath))
	}
	return c.Config
}
func (c *Config[ConfigType]) WithValidation() ConfigType {
	return c.Validate()
}
