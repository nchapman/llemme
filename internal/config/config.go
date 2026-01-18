package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HuggingFace HuggingFace `yaml:"huggingface"`
	LlamaCpp    LlamaCpp    `yaml:"llamacpp"`
	Server      Server      `yaml:"server"`
}

type HuggingFace struct {
	Token        string `yaml:"token"`
	DefaultQuant string `yaml:"default_quant"`
}

type LlamaCpp struct {
	Path          string         `yaml:"path"`
	ContextLength int            `yaml:"context_length"`
	GPULayers     int            `yaml:"gpu_layers"`
	Temperature   float64        `yaml:"temperature"`
	TopP          float64        `yaml:"top_p"`
	TopK          int            `yaml:"top_k"`
	Extra         map[string]any `yaml:",inline"`
}

type Server struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	MaxModels       int    `yaml:"max_models"`
	IdleTimeoutMins int    `yaml:"idle_timeout_mins"`
	StartupTimeoutS int    `yaml:"startup_timeout_secs"`
	BackendPortMin  int    `yaml:"backend_port_min"`
	BackendPortMax  int    `yaml:"backend_port_max"`
}

const (
	configDir  = ".llemme"
	configFile = "config.yaml"
	modelsDir  = "models"
	binDir     = "bin"
	blobsDir   = "blobs"
	logsDir    = "logs"
)

func GetHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func ConfigPath() string {
	return filepath.Join(GetHomeDir(), configDir, configFile)
}

func ModelsPath() string {
	return filepath.Join(GetHomeDir(), configDir, modelsDir)
}

func BinPath() string {
	return filepath.Join(GetHomeDir(), configDir, binDir)
}

func BlobsPath() string {
	return filepath.Join(GetHomeDir(), configDir, blobsDir)
}

func LogsPath() string {
	return filepath.Join(GetHomeDir(), configDir, logsDir)
}

func DefaultConfig() *Config {
	return &Config{
		HuggingFace: HuggingFace{
			Token:        "",
			DefaultQuant: "Q4_K_M",
		},
		LlamaCpp: LlamaCpp{
			Path:          "",
			ContextLength: 4096,
			GPULayers:     -1,
			Temperature:   0.7,
			TopP:          0.9,
			TopK:          40,
		},
		Server: Server{
			Host:            "127.0.0.1",
			Port:            8080,
			MaxModels:       3,
			IdleTimeoutMins: 10,
			StartupTimeoutS: 120,
			BackendPortMin:  49152,
			BackendPortMax:  49200,
		},
	}
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	configPath := ConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	configPath := ConfigPath()
	configDir := filepath.Dir(configPath)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func EnsureDirectories() error {
	dirs := []string{
		ConfigPath(),
		ModelsPath(),
		BinPath(),
		BlobsPath(),
		LogsPath(),
	}

	for _, dir := range dirs {
		if filepath.Ext(dir) != "" {
			dir = filepath.Dir(dir)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
