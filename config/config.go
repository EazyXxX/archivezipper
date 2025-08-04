package config

import (
	"time"
)

type Config struct {
	Server   ServerConfig   `toml:"server"`
	Task     TaskConfig     `toml:"task"`
	Shutdown ShutdownConfig `toml:"shutdown"`
}

type ServerConfig struct {
	Port string `toml:"port"`
}

type TaskConfig struct {
	MaxConcurrent int `toml:"max_concurrent"`
}

type ShutdownConfig struct {
	Delay time.Duration `toml:"delay"`
}
