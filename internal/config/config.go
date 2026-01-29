package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultConfigRelativePath = ".config/slack-cli/config.json"
	currentVersion            = 1
)

// Config represents the configuration stored on disk.
// This CLI operates as a user client, using User Token (xoxp-...) to act on behalf of the user.
type Config struct {
	Version   int            `json:"version"`
	UserToken string         `json:"user_token"`
	Defaults  Defaults       `json:"defaults"`
	Channels  map[string]ACL `json:"channels"`
}

// Defaults groups general default options.
type Defaults struct {
	OutputFormat   string `json:"output_format"`
	IncludeBots    bool   `json:"include_bots"`
	TextChunkLimit int    `json:"text_chunk_limit"`
}

// ACL describes per-channel rules.
type ACL struct {
	Name           string   `json:"name"`
	RequireMention bool     `json:"require_mention"`
	AllowedUsers   []string `json:"allowed_users"`
}

// Load reads configuration from disk, applying defaults and env overrides.
func Load(path string) (*Config, string, error) {
	actualPath, err := resolvePath(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve config path: %w", err)
	}

	cfg := DefaultConfig()
	if _, err := os.Stat(actualPath); err == nil {
		data, readErr := os.ReadFile(actualPath)
		if readErr != nil {
			return nil, "", fmt.Errorf("read config: %w", readErr)
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, "", fmt.Errorf("parse config: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, "", fmt.Errorf("stat config: %w", err)
	}

	applyEnvOverrides(cfg)
	return cfg, actualPath, nil
}

// Save writes the configuration to disk, ensuring directories exist.
func Save(path string, cfg *Config) (string, error) {
	if cfg == nil {
		return "", errors.New("config is nil")
	}
	actualPath, err := resolvePath(path)
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(actualPath), 0o700); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	cfg.Version = currentVersion
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(actualPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return actualPath, nil
}

// DefaultConfig returns a config populated with baseline values.
func DefaultConfig() *Config {
	return &Config{
		Version:   currentVersion,
		Defaults:  Defaults{OutputFormat: "human", IncludeBots: false, TextChunkLimit: 4000},
		Channels:  map[string]ACL{},
		UserToken: "",
	}
}

// Validate ensures required fields are set.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.UserToken == "" {
		return errors.New("user token is required (set SLACK_USER_TOKEN or add user_token to config)")
	}
	return nil
}

func resolvePath(path string) (string, error) {
	if path == "" {
		path = filepath.Join("~", defaultConfigRelativePath)
	}
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determine home directory: %w", err)
		}
		if path == "~" {
			return filepath.Join(home, defaultConfigRelativePath), nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func applyEnvOverrides(cfg *Config) {
	if val := os.Getenv("SLACK_USER_TOKEN"); val != "" {
		cfg.UserToken = val
	}
	if val := os.Getenv("SLACK_CLI_FORMAT"); val != "" {
		cfg.Defaults.OutputFormat = val
	}
}

// DefaultPath returns the resolved default config file path.
func DefaultPath() (string, error) {
	return resolvePath("")
}
