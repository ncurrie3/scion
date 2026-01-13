package config

import (
	"context"

	"github.com/ptone/scion-agent/pkg/api"
)

type MockHarness struct {
	NameVal      string
	EmbedDirVal  string
	ConfigDirVal string
}

func (m *MockHarness) Name() string { return m.NameVal }
func (m *MockHarness) SeedTemplateDir(dir string, force bool) error {
	return SeedCommonFiles(dir, "common", m.EmbedDirVal, m.ConfigDirVal, force)
}
func (m *MockHarness) DiscoverAuth(agentHome string) api.AuthConfig { return api.AuthConfig{} }
func (m *MockHarness) GetEnv(agentName string, agentHome string, unixUsername string, auth api.AuthConfig) map[string]string {
	return nil
}
func (m *MockHarness) GetCommand(task string, resume bool, baseArgs []string) []string { return nil }
func (m *MockHarness) PropagateFiles(homeDir, unixUsername string, auth api.AuthConfig) error {
	return nil
}
func (m *MockHarness) GetVolumes(unixUsername string, auth api.AuthConfig) []api.VolumeMount {
	return nil
}
func (m *MockHarness) DefaultConfigDir() string { return m.ConfigDirVal }
func (m *MockHarness) HasSystemPrompt(agentHome string) bool { return false }
func (m *MockHarness) Provision(ctx context.Context, agentName, agentHome, agentWorkspace string) error {
	return nil
}
func (m *MockHarness) GetEmbedDir() string       { return m.EmbedDirVal }
func (m *MockHarness) GetInterruptKey() string   { return "C-c" }

func GetMockHarnesses() []api.Harness {
	return []api.Harness{
		&MockHarness{NameVal: "gemini", EmbedDirVal: "gemini", ConfigDirVal: ".gemini"},
		&MockHarness{NameVal: "claude", EmbedDirVal: "claude", ConfigDirVal: ".claude"},
		&MockHarness{NameVal: "opencode", EmbedDirVal: "opencode", ConfigDirVal: ".config/opencode"},
		&MockHarness{NameVal: "codex", EmbedDirVal: "codex", ConfigDirVal: ""},
	}
}
