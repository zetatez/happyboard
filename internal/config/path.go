package config

import (
	"os"
	"path/filepath"
)

func DefaultConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "happyboard", "config.yaml")
}
