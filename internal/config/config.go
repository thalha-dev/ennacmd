package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

const (
	DirName        = ".ennacmd"
	ConfigFileName = "config.yaml"
	CacheDir       = "cache"
)

type Paths struct {
	Root       string
	ConfigFile string
	CacheDir   string
}

type Config struct {
	Provider    string  `mapstructure:"provider" yaml:"provider"`
	BaseURL     string  `mapstructure:"base_url" yaml:"base_url"`
	APIKey      string  `mapstructure:"api_key" yaml:"api_key"`
	Model       string  `mapstructure:"model" yaml:"model"`
	Temperature float64 `mapstructure:"temperature" yaml:"temperature"`
	Shell       string  `mapstructure:"shell" yaml:"shell"`
	Streaming   bool    `mapstructure:"streaming" yaml:"streaming"`
	Paths       Paths   `mapstructure:"-" yaml:"-"`
}

func Default() Config {
	return DefaultForProvider("openai")
}

func DefaultForProvider(provider string) Config {
	providerName := strings.ToLower(strings.TrimSpace(provider))
	if providerName == "" {
		providerName = "openai"
	}

	return Config{
		Provider:    providerName,
		BaseURL:     DefaultBaseURL(providerName),
		Model:       DefaultModel(providerName),
		Temperature: 0.2,
		Shell:       "auto",
		Streaming:   true,
	}
}

func DefaultBaseURL(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "ollama":
		return "http://localhost:11434"
	default:
		return "https://api.openai.com/v1"
	}
}

func DefaultModel(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openrouter":
		return "openai/gpt-4o-mini"
	case "ollama":
		return "llama3.2"
	default:
		return "gpt-4o-mini"
	}
}

func Bootstrap() (Paths, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return Paths{}, err
	}

	if err := EnsurePaths(paths); err != nil {
		return Paths{}, err
	}

	if _, err := os.Stat(paths.ConfigFile); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(paths.ConfigFile, []byte(defaultConfigYAML()), 0o600); err != nil {
			return Paths{}, fmt.Errorf("create config file: %w", err)
		}
	}

	return paths, nil
}

func Load() (Config, error) {
	paths, err := Bootstrap()
	if err != nil {
		return Config{}, err
	}

	v := viper.New()
	v.SetConfigFile(paths.ConfigFile)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("ENNACMD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	defaults := Default()
	v.SetDefault("provider", defaults.Provider)
	v.SetDefault("base_url", defaults.BaseURL)
	v.SetDefault("model", defaults.Model)
	v.SetDefault("temperature", defaults.Temperature)
	v.SetDefault("shell", defaults.Shell)
	v.SetDefault("streaming", defaults.Streaming)

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if cfg.Provider == "" {
		cfg.Provider = defaults.Provider
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL(cfg.Provider)
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel(cfg.Provider)
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = defaults.Temperature
	}
	if cfg.Shell == "" {
		cfg.Shell = defaults.Shell
	}
	cfg.Paths = paths

	return cfg, nil
}

func ResolvePaths() (Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}

	root := filepath.Join(homeDir, DirName)
	return Paths{
		Root:       root,
		ConfigFile: filepath.Join(root, ConfigFileName),
		CacheDir:   filepath.Join(root, CacheDir),
	}, nil
}

func EnsurePaths(paths Paths) error {
	for _, dir := range []string{paths.Root, paths.CacheDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	return nil
}

func (c Config) Validate() error {
	providerName := strings.ToLower(strings.TrimSpace(c.Provider))
	if providerName == "" {
		return errors.New("config provider must not be empty")
	}
	if !isSupportedProvider(providerName) {
		return fmt.Errorf("config provider %q is not supported; expected one of: openai, openrouter, ollama", providerName)
	}
	if strings.TrimSpace(c.Model) == "" {
		return errors.New("config model must not be empty")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return errors.New("config base_url must not be empty")
	}
	if providerName != "ollama" && strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("config api_key is required for provider %q", providerName)
	}
	return nil
}

func isSupportedProvider(providerName string) bool {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "openai", "openrouter", "ollama":
		return true
	default:
		return false
	}
}

func Save(cfg Config) (Config, error) {
	defaults := DefaultForProvider(cfg.Provider)
	paths := cfg.Paths
	if paths.ConfigFile == "" {
		var err error
		paths, err = ResolvePaths()
		if err != nil {
			return Config{}, err
		}
	}
	if err := EnsurePaths(paths); err != nil {
		return Config{}, err
	}

	normalized := cfg
	normalized.Paths = paths
	if strings.TrimSpace(normalized.Provider) == "" {
		normalized.Provider = defaults.Provider
	} else {
		normalized.Provider = strings.ToLower(strings.TrimSpace(normalized.Provider))
	}
	normalized.BaseURL = strings.TrimSpace(normalized.BaseURL)
	if normalized.BaseURL == "" {
		normalized.BaseURL = DefaultBaseURL(normalized.Provider)
	}
	normalized.APIKey = strings.TrimSpace(normalized.APIKey)
	normalized.Model = strings.TrimSpace(normalized.Model)
	if normalized.Model == "" {
		normalized.Model = DefaultModel(normalized.Provider)
	}
	if normalized.Temperature == 0 {
		normalized.Temperature = defaults.Temperature
	}
	normalized.Shell = strings.TrimSpace(normalized.Shell)
	if normalized.Shell == "" {
		normalized.Shell = defaults.Shell
	}

	if err := normalized.Validate(); err != nil {
		return Config{}, err
	}

	stored := Config{
		Provider:    normalized.Provider,
		BaseURL:     normalized.BaseURL,
		APIKey:      normalized.APIKey,
		Model:       normalized.Model,
		Temperature: normalized.Temperature,
		Shell:       normalized.Shell,
		Streaming:   normalized.Streaming,
	}
	data, err := yaml.Marshal(stored)
	if err != nil {
		return Config{}, fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(paths.ConfigFile, data, 0o600); err != nil {
		return Config{}, fmt.Errorf("write config: %w", err)
	}

	return normalized, nil
}

func defaultConfigYAML() string {
	return strings.TrimSpace(`provider: openai
base_url: https://api.openai.com/v1
api_key: ""
model: gpt-4o-mini
temperature: 0.2
shell: auto
streaming: true
`) + "\n"
}
