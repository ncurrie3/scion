/*
Copyright 2025 The Scion Authors.
*/

package dialects

import (
	"testing"

	"github.com/GoogleCloudPlatform/scion/pkg/sciontool/hooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeDialect_Name(t *testing.T) {
	d := NewClaudeDialect()
	assert.Equal(t, "claude", d.Name())
}

func TestClaudeDialect_Parse(t *testing.T) {
	tests := []struct {
		name       string
		input      map[string]interface{}
		wantName   string
		wantTool   string
		wantPrompt string
	}{
		{
			name: "PreToolUse event",
			input: map[string]interface{}{
				"hook_event_name": "PreToolUse",
				"tool_name":       "Bash",
			},
			wantName: hooks.EventToolStart,
			wantTool: "Bash",
		},
		{
			name: "PostToolUse event",
			input: map[string]interface{}{
				"hook_event_name": "PostToolUse",
				"tool_name":       "Read",
			},
			wantName: hooks.EventToolEnd,
			wantTool: "Read",
		},
		{
			name: "SessionStart event",
			input: map[string]interface{}{
				"hook_event_name": "SessionStart",
				"source":          "cli",
			},
			wantName: hooks.EventSessionStart,
		},
		{
			name: "SessionEnd event",
			input: map[string]interface{}{
				"hook_event_name": "SessionEnd",
				"reason":          "user_exit",
			},
			wantName: hooks.EventSessionEnd,
		},
		{
			name: "UserPromptSubmit event",
			input: map[string]interface{}{
				"hook_event_name": "UserPromptSubmit",
				"prompt":          "Help me write tests",
			},
			wantName:   hooks.EventPromptSubmit,
			wantPrompt: "Help me write tests",
		},
		{
			name: "Stop event",
			input: map[string]interface{}{
				"hook_event_name": "Stop",
			},
			wantName: hooks.EventAgentEnd,
		},
		{
			name: "SubagentStop event",
			input: map[string]interface{}{
				"hook_event_name": "SubagentStop",
			},
			wantName: hooks.EventAgentEnd,
		},
		{
			name: "Notification event",
			input: map[string]interface{}{
				"hook_event_name": "Notification",
				"message":         "Permission required",
			},
			wantName: hooks.EventNotification,
		},
		{
			name: "Unknown event preserves name",
			input: map[string]interface{}{
				"hook_event_name": "CustomEvent",
			},
			wantName: "CustomEvent",
		},
	}

	d := NewClaudeDialect()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := d.Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, event.Name)
			assert.Equal(t, "claude", event.Dialect)

			if tt.wantTool != "" {
				assert.Equal(t, tt.wantTool, event.Data.ToolName)
			}
			if tt.wantPrompt != "" {
				assert.Equal(t, tt.wantPrompt, event.Data.Prompt)
			}
		})
	}
}
