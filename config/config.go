package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type SourceConfig struct {
	IP              string `yaml:"ip"`
	Port            int    `yaml:"port"`
	SlaveID         byte   `yaml:"slave_id"`
	TimeoutMs       int    `yaml:"timeout_ms"`
	PollingInterval int    `yaml:"polling_interval_ms"`
	RetryCount      int    `yaml:"retry_count"`
	RetryDelayMs    int    `yaml:"retry_delay_ms"`
}

type ServerConfig struct {
	IP      string `yaml:"ip"`
	Port    int    `yaml:"port"`
	SlaveID byte   `yaml:"slave_id"`
}

type RegisterBlock struct {
	Name         string `yaml:"name"`
	Function     string `yaml:"function"`
	StartAddress uint16 `yaml:"start_address"`
	Count        uint16 `yaml:"count"`
}

type LoggingConfig struct {
	Level          string `yaml:"level"`
	LogDataChanges bool   `yaml:"log_data_changes"`
}

type AppConfig struct {
	Source         SourceConfig    `yaml:"source"`
	Server         ServerConfig    `yaml:"server"`
	RegisterBlocks []RegisterBlock `yaml:"register_blocks"`
	Logging        LoggingConfig   `yaml:"logging"`
}

func (rb *RegisterBlock) FunctionType() string {
	return strings.ToLower(strings.TrimSpace(rb.Function))
}

func Load(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return &cfg, nil
}

func (c *AppConfig) validate() error {
	if c.Source.IP == "" {
		return fmt.Errorf("source.ip must not be empty")
	}
	if c.Source.Port <= 0 || c.Source.Port > 65535 {
		return fmt.Errorf("source.port is invalid: %d", c.Source.Port)
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port is invalid: %d", c.Server.Port)
	}
	if c.Source.TimeoutMs <= 0 {
		c.Source.TimeoutMs = 3000
	}
	if c.Source.PollingInterval <= 0 {
		c.Source.PollingInterval = 1000
	}
	if c.Source.RetryCount <= 0 {
		c.Source.RetryCount = 3
	}
	if len(c.RegisterBlocks) == 0 {
		return fmt.Errorf("at least one register_block must be defined")
	}

	for i, block := range c.RegisterBlocks {
		ft := block.FunctionType()
		switch ft {
		case "holding_register", "input_register", "coil", "discrete_input":
		default:
			return fmt.Errorf("register_blocks[%d]: invalid function type: %s", i, block.Function)
		}
		if block.Count == 0 {
			return fmt.Errorf("register_blocks[%d]: count must not be 0", i)
		}
		if ft == "holding_register" || ft == "input_register" {
			if block.Count > 125 {
				return fmt.Errorf("register_blocks[%d]: max 125 registers per read (count=%d)", i, block.Count)
			}
		} else {
			if block.Count > 2000 {
				return fmt.Errorf("register_blocks[%d]: max 2000 coils/discrete inputs per read (count=%d)", i, block.Count)
			}
		}
	}

	return nil
}

func (c *AppConfig) SourceAddress() string {
	return fmt.Sprintf("%s:%d", c.Source.IP, c.Source.Port)
}

func (c *AppConfig) ServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.IP, c.Server.Port)
}
