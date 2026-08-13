package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ClientID    string `json:"client_id"`
	TenantID    string `json:"tenant_id"`
	EnableTeams bool   `json:"enable_teams"`
}

// configDir returns the directory where blick keeps its config and token
// cache. If a legacy ~/.config/checkin/ directory exists from before the
// rename and ~/.config/blick/ does not, the legacy directory is moved
// in-place — silent, one-time, transparent to the user. Migration failures
// fall back to the legacy path so existing users keep working.
func configDir() string {
	home, _ := os.UserHomeDir()
	blickDir := filepath.Join(home, ".config", "blick")
	legacyDir := filepath.Join(home, ".config", "checkin")

	if _, err := os.Stat(blickDir); os.IsNotExist(err) {
		if _, err := os.Stat(legacyDir); err == nil {
			if renameErr := os.Rename(legacyDir, blickDir); renameErr != nil {
				return legacyDir
			}
		}
	}
	return blickDir
}

func loadConfig() (Config, error) {
	path := filepath.Join(configDir(), "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("config file not found at %s\n\nCreate it with:\n  mkdir -p %s\n  cat > %s << 'EOF'\n  {\n    \"client_id\": \"YOUR_APP_CLIENT_ID\",\n    \"tenant_id\": \"YOUR_TENANT_ID\"\n  }\n  EOF\n\nSee README.md for Azure AD app registration steps", path, configDir(), path)
		}
		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.ClientID == "" || cfg.TenantID == "" {
		return Config{}, fmt.Errorf("config must include both client_id and tenant_id")
	}

	return cfg, nil
}
