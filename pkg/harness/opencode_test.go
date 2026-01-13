package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeedTemplateDir_OpenCode(t *testing.T) {
	// Setup a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "scion-opencode-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	templateDir := filepath.Join(tmpDir, "opencode")
	o := &OpenCode{}

	err = o.SeedTemplateDir(templateDir, true)
	if err != nil {
		t.Fatalf("SeedTemplateDir failed: %v", err)
	}

	// Verify opencode.json exists in the correct location
	opencodeJSONPath := filepath.Join(templateDir, "home", ".config", "opencode", "opencode.json")
	if _, err := os.Stat(opencodeJSONPath); os.IsNotExist(err) {
		t.Errorf("expected opencode.json to exist at %s", opencodeJSONPath)
	}

	// Verify gemini-specific material is NOT there
	opencodeConfigPath := filepath.Join(templateDir, "home", ".opencode")
	if _, err := os.Stat(opencodeConfigPath); err == nil {
		t.Error("expected .opencode directory to NOT exist in opencode template")
	}

	// Verify it's not empty
	data, err := os.ReadFile(opencodeJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected opencode.json to have content, but it's empty")
	}
}
