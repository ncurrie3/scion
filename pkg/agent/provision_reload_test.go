// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/api"
	"github.com/GoogleCloudPlatform/scion/pkg/config"
)

func TestProvisionAgentReloadsConfig(t *testing.T) {
	mockRuntimeForTest(t)
	// This test verifies that ProvisionAgent reloads the config after harness.Provision
	// which allows harness-injected changes (like GEMINI_API_KEY) to be returned.

	tmpDir := t.TempDir()

	// Move to tmpDir
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Mock HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	// Seed global harness-configs (required for agent creation)
	if err := config.InitMachine(getTestHarnesses()); err != nil {
		t.Fatalf("InitMachine failed: %v", err)
	}

	// Initialize a mock project
	projectDir := filepath.Join(tmpDir, "project")
	projectScionDir := filepath.Join(projectDir, ".scion")
	if err := config.InitProject(projectScionDir, getTestHarnesses()); err != nil {
		t.Fatalf("InitProject failed: %v", err)
	}

	// Chdir to projectDir so GetProjectDir finds it
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}

	// Provision a gemini agent using the "default" agnostic template with --harness-config=gemini
	agentName := "reload-test-agent"
	_, _, cfg, err := ProvisionAgent(context.Background(), agentName, "default", "", "gemini", projectScionDir, "", "", "", "")
	if err != nil {
		t.Fatalf("ProvisionAgent failed: %v", err)
	}

	// Verify env
	if cfg.Env == nil {
		t.Fatal("expected cfg.Env to be non-nil")
	}

	val, ok := cfg.Env["GEMINI_API_KEY"]
	if !ok {
		t.Error("expected GEMINI_API_KEY to be in returned config Env")
	}
	if val != "${GEMINI_API_KEY}" {
		t.Errorf("expected GEMINI_API_KEY to be '${GEMINI_API_KEY}', got '%s'", val)
	}
}

func TestProvisionAgentWithHarnessAuthOverride(t *testing.T) {
	mockRuntimeForTest(t)
	// Verify that when --harness-auth vertex-ai is used with the gemini harness,
	// GEMINI_API_KEY is NOT injected into the env map by harness Provision().

	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	if err := config.InitMachine(getTestHarnesses()); err != nil {
		t.Fatalf("InitMachine failed: %v", err)
	}

	projectDir := filepath.Join(tmpDir, "project")
	projectScionDir := filepath.Join(projectDir, ".scion")
	if err := config.InitProject(projectScionDir, getTestHarnesses()); err != nil {
		t.Fatalf("InitProject failed: %v", err)
	}

	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}

	// Provision with vertex-ai override via inline config (simulates --harness-auth vertex-ai)
	agentName := "vertex-ai-override"
	inlineCfg := &api.ScionConfig{AuthSelectedType: "vertex-ai"}
	_, _, cfg, err := ProvisionAgent(context.Background(), agentName, "default", "", "gemini", projectScionDir, "", "", "", "", inlineCfg)
	if err != nil {
		t.Fatalf("ProvisionAgent failed: %v", err)
	}

	if cfg.Env == nil {
		t.Fatal("expected cfg.Env to be non-nil")
	}

	// GEMINI_API_KEY should NOT be present — vertex-ai doesn't use it
	if _, ok := cfg.Env["GEMINI_API_KEY"]; ok {
		t.Error("GEMINI_API_KEY should not be in env when auth is vertex-ai")
	}

	// GOOGLE_CLOUD_PROJECT should be present for vertex-ai
	if _, ok := cfg.Env["GOOGLE_CLOUD_PROJECT"]; !ok {
		t.Error("expected GOOGLE_CLOUD_PROJECT to be in env for vertex-ai auth")
	}
}
