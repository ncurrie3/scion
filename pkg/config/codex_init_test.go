package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeedTemplateDir_Codex(t *testing.T) {
	// Setup a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "scion-test-codex-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	templateDir := filepath.Join(tmpDir, "codex-template")
	
	err = SeedTemplateDir(templateDir, "codex", "codex", "codex", "", true)
	if err != nil {
		t.Fatalf("SeedTemplateDir failed: %v", err)
	}

	// Verify .codex directory exists
	codexDir := filepath.Join(templateDir, "home", ".codex")
	if info, err := os.Stat(codexDir); err != nil {
		t.Errorf("expected .codex directory to exist: %v", err)
	} else if !info.IsDir() {
		t.Errorf(".codex is not a directory")
	}

	// Verify config.toml exists
	configPath := filepath.Join(codexDir, "config.toml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("expected config.toml to exist: %v", err)
	}
}
