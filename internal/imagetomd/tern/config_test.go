package tern

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadInProcessConfigExplicitPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "tern-config.yaml")
	if err := os.WriteFile(cfgPath, []byte(sampleConfig("./profiles.yaml")), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "profiles.yaml"), []byte("default_profile:\n  provider: openai\n  model: gpt-5.3-codex\nproviders: {}\n"), 0o644); err != nil {
		t.Fatalf("write profiles: %v", err)
	}
	cfg, err := LoadInProcessConfig(LoadConfigInput{
		ExplicitPath: cfgPath,
	})
	if err != nil {
		t.Fatalf("LoadInProcessConfig returned error: %v", err)
	}
	if cfg.Port != 14000 {
		t.Fatalf("unexpected port: %d", cfg.Port)
	}
	wantModelPath := filepath.Join(tmpDir, "profiles.yaml")
	if cfg.ModelProfilesPath != filepath.Clean(wantModelPath) {
		t.Fatalf("unexpected model_profiles_path: got %s want %s", cfg.ModelProfilesPath, wantModelPath)
	}
}

func TestLoadInProcessConfigSearchCurrentDirectory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "tern-config.yaml")
	if err := os.WriteFile(cfgPath, []byte(sampleConfig("./profiles.yaml")), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "profiles.yaml"), []byte("default_profile:\n  provider: openai\n  model: gpt-5.3-codex\nproviders: {}\n"), 0o644); err != nil {
		t.Fatalf("write profiles: %v", err)
	}
	cfg, err := LoadInProcessConfig(LoadConfigInput{
		WorkingDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("LoadInProcessConfig returned error: %v", err)
	}
	if cfg.ConfigPath != filepath.Clean(cfgPath) {
		t.Fatalf("unexpected config path: got %s want %s", cfg.ConfigPath, cfgPath)
	}
}

func TestLoadInProcessConfigMissingFile(t *testing.T) {
	t.Parallel()
	_, err := LoadInProcessConfig(LoadConfigInput{
		WorkingDir: t.TempDir(),
	})
	if err == nil {
		t.Fatalf("expected config missing error")
	}
	if !errors.Is(err, ErrConfigMissing) {
		t.Fatalf("expected ErrConfigMissing, got %v", err)
	}
}

func sampleConfig(modelProfilesPath string) string {
	return `llm_gateway:
  port: 14000
  model_profiles_path: "` + modelProfilesPath + `"
vault:
  backend: keyring
`
}
