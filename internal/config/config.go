package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const appName = "memctl"

type Paths struct {
	Home            string
	ConfigPath      string
	StorePath       string
	LegacyStorePath string
}

type Config struct {
	StorePath         string `json:"store_path"`
	DefaultPackLimit  int    `json:"default_pack_limit"`
	DefaultPackFormat string `json:"default_pack_format"`
	LegacyStorePath   string `json:"-"`
}

func ResolvePaths() (Paths, error) {
	if home := os.Getenv("MEMCTL_HOME"); home != "" {
		return Paths{
			Home:            home,
			ConfigPath:      filepath.Join(home, "config.json"),
			StorePath:       filepath.Join(home, "memories.db"),
			LegacyStorePath: filepath.Join(home, "memories.json"),
		}, nil
	}

	root, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	home := filepath.Join(root, appName)
	return Paths{
		Home:            home,
		ConfigPath:      filepath.Join(home, "config.json"),
		StorePath:       filepath.Join(home, "memories.db"),
		LegacyStorePath: filepath.Join(home, "memories.json"),
	}, nil
}

func Default(paths Paths) Config {
	return Config{
		StorePath:         paths.StorePath,
		DefaultPackLimit:  8,
		DefaultPackFormat: "markdown",
	}
}

func Load() (Config, Paths, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return Config{}, Paths{}, err
	}

	cfg := Default(paths)
	raw, err := os.ReadFile(paths.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, paths, nil
		}
		return Config{}, Paths{}, err
	}
	if len(raw) == 0 {
		return cfg, paths, nil
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, Paths{}, err
	}
	cfg.StorePath, cfg.LegacyStorePath = normalizeStorePath(cfg.StorePath, paths)
	if cfg.DefaultPackLimit <= 0 {
		cfg.DefaultPackLimit = 8
	}
	if cfg.DefaultPackFormat == "" {
		cfg.DefaultPackFormat = "markdown"
	}
	return cfg, paths, nil
}

func normalizeStorePath(storePath string, paths Paths) (string, string) {
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		return paths.StorePath, paths.LegacyStorePath
	}

	cleaned := filepath.Clean(storePath)
	if cleaned == filepath.Clean(paths.LegacyStorePath) {
		return paths.StorePath, paths.LegacyStorePath
	}
	if strings.EqualFold(filepath.Ext(cleaned), ".json") {
		return strings.TrimSuffix(cleaned, filepath.Ext(cleaned)) + ".db", cleaned
	}
	return cleaned, ""
}

func WriteDefault(force bool) (Config, Paths, error) {
	cfg, paths, err := Load()
	if err != nil {
		return Config{}, Paths{}, err
	}
	if !force {
		if _, err := os.Stat(paths.ConfigPath); err == nil {
			return cfg, paths, nil
		}
	}
	if err := os.MkdirAll(paths.Home, 0o755); err != nil {
		return Config{}, Paths{}, err
	}
	raw, err := json.MarshalIndent(Default(paths), "", "  ")
	if err != nil {
		return Config{}, Paths{}, err
	}
	if err := os.WriteFile(paths.ConfigPath, raw, 0o644); err != nil {
		return Config{}, Paths{}, err
	}
	return Default(paths), paths, nil
}
