package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type AppConfig struct {
	Remote          string   `json:"remote"`
	Listen          string   `json:"listen"`
	ManualAddresses []string `json:"manual_addresses"`
}

func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		home, herr := os.UserHomeDir()
		if herr != nil || home == "" {
			if err != nil {
				return "", err
			}
			return "", fmt.Errorf("cannot determine config directory")
		}
		dir = filepath.Join(home, ".config")
	}
	appDir := filepath.Join(dir, APP_NAME)
	if mkerr := os.MkdirAll(appDir, 0o755); mkerr != nil {
		return "", mkerr
	}
	return filepath.Join(appDir, "config.json"), nil
}

func LoadAppConfig() (AppConfig, error) {
	var cfg AppConfig
	p, err := ConfigPath()
	if err != nil {
		return cfg, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func SaveAppConfig(cfg AppConfig) error {
	p, err := ConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
