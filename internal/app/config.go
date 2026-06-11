package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const configDirName = "tui-worktree"

type UserConfig struct {
	Theme string `json:"theme,omitempty"`
}

func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configDirName, "config.json"), nil
}

func LoadConfig() (UserConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		return UserConfig{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return UserConfig{}, nil
	}
	if err != nil {
		return UserConfig{}, err
	}
	var cfg UserConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return UserConfig{}, err
	}
	return cfg, nil
}

func SaveConfig(cfg UserConfig) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func ResolveTheme(opts Options) string {
	if opts.Theme != "" {
		return opts.Theme
	}
	cfg, err := LoadConfig()
	if err == nil && cfg.Theme != "" {
		return cfg.Theme
	}
	return "tokyonight"
}
