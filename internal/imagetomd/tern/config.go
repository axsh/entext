package tern

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type LoadConfigInput struct {
	ExplicitPath string
	WorkingDir   string
}

type InProcessConfig struct {
	ConfigPath        string
	Port              int
	ModelProfilesPath string
	VaultBackend      string
}

type rawConfig struct {
	LLMGateway struct {
		Port              int    `yaml:"port"`
		ModelProfilesPath string `yaml:"model_profiles_path"`
	} `yaml:"llm_gateway"`
	Vault struct {
		Backend string `yaml:"backend"`
	} `yaml:"vault"`
}

func LoadInProcessConfig(in LoadConfigInput) (InProcessConfig, error) {
	configPath, err := resolveConfigPath(in)
	if err != nil {
		return InProcessConfig{}, err
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return InProcessConfig{}, fmt.Errorf("%w: %s", ErrConfigMissing, configPath)
		}
		return InProcessConfig{}, err
	}
	var cfg rawConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return InProcessConfig{}, fmt.Errorf("failed to parse tern config: %w", err)
	}
	if cfg.LLMGateway.Port <= 0 {
		return InProcessConfig{}, fmt.Errorf("invalid llm_gateway.port: %d", cfg.LLMGateway.Port)
	}
	if cfg.LLMGateway.ModelProfilesPath == "" {
		return InProcessConfig{}, errors.New("llm_gateway.model_profiles_path is required")
	}
	modelProfilesPath := cfg.LLMGateway.ModelProfilesPath
	if !filepath.IsAbs(modelProfilesPath) {
		modelProfilesPath = filepath.Join(filepath.Dir(configPath), modelProfilesPath)
	}
	modelProfilesPath = filepath.Clean(modelProfilesPath)
	if _, err := os.Stat(modelProfilesPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return InProcessConfig{}, fmt.Errorf("%w: model profiles file not found: %s", ErrConfigMissing, modelProfilesPath)
		}
		return InProcessConfig{}, err
	}
	return InProcessConfig{
		ConfigPath:        configPath,
		Port:              cfg.LLMGateway.Port,
		ModelProfilesPath: modelProfilesPath,
		VaultBackend:      cfg.Vault.Backend,
	}, nil
}

func resolveConfigPath(in LoadConfigInput) (string, error) {
	if in.ExplicitPath != "" {
		path, err := filepath.Abs(in.ExplicitPath)
		if err != nil {
			return "", err
		}
		return filepath.Clean(path), nil
	}
	wd := in.WorkingDir
	if wd == "" {
		wd = "."
	}
	path := filepath.Join(wd, "tern-config.yaml")
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(absPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: %s", ErrConfigMissing, absPath)
		}
		return "", err
	}
	return filepath.Clean(absPath), nil
}
