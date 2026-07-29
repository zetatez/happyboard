package config

import (
	"os"

	"github.com/happyboard/happyboard/internal/hook"
	"gopkg.in/yaml.v3"
)

type ActionType string

const (
	ActionShell     ActionType = "shell"
	ActionInternal  ActionType = "internal"
	ActionTextInput ActionType = "text_input"
)

type ActionRule struct {
	Trigger hook.KeyCombo `yaml:"trigger"`
	Type    ActionType    `yaml:"type"`
	Command string        `yaml:"command"`
	Async   bool          `yaml:"async"`
	Text    string        `yaml:"text"`
	Enter   bool          `yaml:"enter"`
	Delay   int           `yaml:"delay"`
}

type AppMatcher struct {
	AppID       string `yaml:"app_id"`
	ProcessName string `yaml:"process_name"`
	Title       string `yaml:"title"`
}

type Profile struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Match       []AppMatcher `yaml:"match"`
	IsDefault   bool         `yaml:"is_default"`
	Actions     []ActionRule `yaml:"actions"`
}

type LinuxConfig struct {
	Backend string `yaml:"backend"`
}

type PlatformConfig struct {
	Linux LinuxConfig `yaml:"linux"`
}

type Config struct {
	Profiles      []Profile      `yaml:"profiles"`
	LogLevel      string         `yaml:"log_level"`
	HijackEnabled bool           `yaml:"hijack_enabled"`
	ToggleKey     hook.KeyCombo  `yaml:"toggle_key"`
	Platform      PlatformConfig `yaml:"platform"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if _, ok := raw["hijack_enabled"]; !ok {
		cfg.HijackEnabled = true
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.Platform.Linux.Backend == "" {
		cfg.Platform.Linux.Backend = "auto"
	}

	return &cfg, nil
}
