package agent

import (
	"github.com/ptone/scion-agent/pkg/api"
	"github.com/ptone/scion-agent/pkg/harness"
)

func getTestHarnesses() []api.Harness {
	return []api.Harness{
		&harness.GeminiCLI{},
		&harness.ClaudeCode{},
		&harness.OpenCode{},
		&harness.Codex{},
	}
}
