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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- DetectSettingsFormat tests ---

func TestDetectSettingsFormat_Empty(t *testing.T) {
	version, isLegacy := DetectSettingsFormat(nil)
	assert.Equal(t, "", version)
	assert.False(t, isLegacy)

	version, isLegacy = DetectSettingsFormat([]byte{})
	assert.Equal(t, "", version)
	assert.False(t, isLegacy)
}

func TestDetectSettingsFormat_Versioned(t *testing.T) {
	data := []byte(`
schema_version: "1"
active_profile: local
harness_configs:
  gemini:
    harness: gemini
    image: example.com/gemini:latest
`)
	version, isLegacy := DetectSettingsFormat(data)
	assert.Equal(t, "1", version)
	assert.False(t, isLegacy)
}

func TestDetectSettingsFormat_Legacy(t *testing.T) {
	data := []byte(`
active_profile: local
harnesses:
  gemini:
    image: example.com/gemini:latest
    user: scion
`)
	version, isLegacy := DetectSettingsFormat(data)
	assert.Equal(t, "", version)
	assert.True(t, isLegacy)
}

func TestDetectSettingsFormat_Minimal(t *testing.T) {
	// No schema_version, no harnesses — neither versioned nor legacy
	data := []byte(`
active_profile: local
default_template: gemini
`)
	version, isLegacy := DetectSettingsFormat(data)
	assert.Equal(t, "", version)
	assert.False(t, isLegacy)
}

func TestDetectSettingsFormat_InvalidYAML(t *testing.T) {
	data := []byte(`{{{invalid yaml`)
	version, isLegacy := DetectSettingsFormat(data)
	assert.Equal(t, "", version)
	assert.False(t, isLegacy)
}

func TestDetectSettingsFormat_VersionedTakesPrecedence(t *testing.T) {
	// If both schema_version and harnesses exist, it's versioned
	data := []byte(`
schema_version: "1"
harnesses:
  gemini:
    image: test
`)
	version, isLegacy := DetectSettingsFormat(data)
	assert.Equal(t, "1", version)
	assert.False(t, isLegacy)
}

// --- ValidateSettings tests ---

func TestValidateSettings_ValidV1(t *testing.T) {
	data := []byte(`
schema_version: "1"
active_profile: local
default_template: gemini
cli:
  autohelp: true
  interactive_disabled: false
hub:
  enabled: true
  endpoint: "https://hub.example.com"
  grove_id: "abc-123"
  local_only: false
runtimes:
  docker:
    type: docker
    host: ""
  container:
    type: container
    tmux: true
harness_configs:
  gemini:
    harness: gemini
    image: "us-central1-docker.pkg.dev/test/scion-gemini:latest"
    user: scion
  claude:
    harness: claude
    image: "us-central1-docker.pkg.dev/test/scion-claude:latest"
    user: scion
profiles:
  local:
    runtime: container
    tmux: true
  remote:
    runtime: kubernetes
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.Empty(t, errors, "valid settings should produce no validation errors")
}

func TestValidateSettings_MinimalValid(t *testing.T) {
	data := []byte(`
schema_version: "1"
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.Empty(t, errors, "minimal valid settings should produce no errors")
}

func TestValidateSettings_UnknownTopLevelField(t *testing.T) {
	data := []byte(`
schema_version: "1"
unknown_field: value
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "unknown top-level field should produce validation error")

	// Check that the error mentions the unknown field
	found := false
	for _, e := range errors {
		if e.Path == "" || e.Path == "unknown_field" {
			found = true
			break
		}
	}
	assert.True(t, found, "should report error about unknown_field, got: %v", errors)
}

func TestValidateSettings_InvalidSchemaVersion(t *testing.T) {
	data := []byte(`
schema_version: "2"
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "wrong schema_version value should produce validation error")
}

func TestValidateSettings_InvalidRuntimeType(t *testing.T) {
	data := []byte(`
schema_version: "1"
runtimes:
  custom:
    type: invalid_type
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "invalid runtime type should produce validation error")
}

func TestValidateSettings_InvalidHarnessType(t *testing.T) {
	data := []byte(`
schema_version: "1"
harness_configs:
  test:
    harness: nonexistent
    image: test:latest
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "invalid harness type should produce validation error")
}

func TestValidateSettings_MissingRequiredHarnessField(t *testing.T) {
	data := []byte(`
schema_version: "1"
harness_configs:
  test:
    image: test:latest
    user: scion
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "missing required 'harness' field should produce validation error")
}

func TestValidateSettings_MissingRequiredProfileRuntime(t *testing.T) {
	data := []byte(`
schema_version: "1"
profiles:
  test:
    tmux: true
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "missing required 'runtime' in profile should produce validation error")
}

func TestValidateSettings_UnknownFieldInHub(t *testing.T) {
	data := []byte(`
schema_version: "1"
hub:
  enabled: true
  token: "secret"
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "unknown field 'token' in hub should produce validation error")
}

func TestValidateSettings_UnsupportedVersion(t *testing.T) {
	data := []byte(`schema_version: "99"`)
	_, err := ValidateSettings(data, "99")
	assert.Error(t, err, "unsupported schema version should return an error")
	assert.Contains(t, err.Error(), "unsupported schema version")
}

func TestValidateSettings_InvalidYAML(t *testing.T) {
	data := []byte(`{{{not yaml`)
	_, err := ValidateSettings(data, "1")
	assert.Error(t, err, "invalid YAML should return a parse error")
}

func TestValidateSettings_ServerSection(t *testing.T) {
	data := []byte(`
schema_version: "1"
server:
  env: prod
  log_level: info
  log_format: text
  hub:
    port: 9810
    host: "0.0.0.0"
  broker:
    enabled: true
    port: 9800
    broker_id: "test-broker-uuid"
  database:
    driver: sqlite
  auth:
    dev_mode: false
  storage:
    provider: local
  secrets:
    backend: local
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.Empty(t, errors, "valid server section should produce no errors")
}

func TestValidateSettings_InvalidServerLogLevel(t *testing.T) {
	data := []byte(`
schema_version: "1"
server:
  log_level: verbose
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "invalid log_level should produce validation error")
}

func TestValidateSettings_UnknownFieldInServer(t *testing.T) {
	data := []byte(`
schema_version: "1"
server:
  unknown_server_field: true
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "unknown field in server should produce validation error")
}

func TestValidateSettings_HarnessConfigWithAllFields(t *testing.T) {
	data := []byte(`
schema_version: "1"
harness_configs:
  gemini-custom:
    harness: gemini
    image: "example.com/gemini:v2"
    user: scion
    model: "gemini-2.5-pro"
    args: ["--sandbox=strict"]
    env:
      GEMINI_SAFETY: "maximum"
    volumes:
      - source: /host/path
        target: /container/path
        read_only: true
    auth_selected_type: "vertex-ai"
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.Empty(t, errors, "harness config with all valid fields should pass")
}

func TestValidateSettings_ProfileWithOverrides(t *testing.T) {
	data := []byte(`
schema_version: "1"
profiles:
  staging:
    runtime: docker
    default_template: gemini
    default_harness_config: gemini
    tmux: false
    env:
      ENV: staging
    resources:
      requests:
        cpu: "500m"
        memory: "512Mi"
      limits:
        cpu: "2"
        memory: "2Gi"
      disk: "10Gi"
    harness_overrides:
      gemini:
        image: "custom:staging"
        env:
          EXTRA: "value"
`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.Empty(t, errors, "profile with overrides should pass validation")
}

// --- ValidateAgentConfig tests ---

func TestValidateAgentConfig_Valid(t *testing.T) {
	data := []byte(`
schema_version: "1"
harness_config: gemini
image: "example.com/gemini:latest"
user: scion
model: "gemini-2.5-pro"
max_turns: 50
max_duration: "2h"
env:
  FOO: bar
`)
	errors, err := ValidateAgentConfig(data, "1")
	require.NoError(t, err)
	assert.Empty(t, errors, "valid agent config should produce no errors")
}

func TestValidateAgentConfig_WithServices(t *testing.T) {
	data := []byte(`
schema_version: "1"
harness_config: gemini
services:
  - name: browser
    command: ["chromium", "--headless"]
    restart: on-failure
    ready_check:
      type: tcp
      target: "localhost:9222"
      timeout: "10s"
`)
	errors, err := ValidateAgentConfig(data, "1")
	require.NoError(t, err)
	assert.Empty(t, errors, "agent config with services should pass validation")
}

func TestValidateAgentConfig_InvalidMaxDuration(t *testing.T) {
	data := []byte(`
schema_version: "1"
harness_config: gemini
max_duration: "2 hours"
`)
	errors, err := ValidateAgentConfig(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "invalid max_duration format should produce validation error")
}

func TestValidateAgentConfig_InvalidMaxTurns(t *testing.T) {
	data := []byte(`
schema_version: "1"
harness_config: gemini
max_turns: 0
`)
	errors, err := ValidateAgentConfig(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "max_turns=0 should produce validation error (minimum is 1)")
}

// --- Schema retrieval tests ---

func TestGetSettingsSchemaJSON(t *testing.T) {
	data, err := GetSettingsSchemaJSON("1")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), `"$schema"`)
	assert.Contains(t, string(data), `"Scion Settings"`)
}

func TestGetSettingsSchemaJSON_UnsupportedVersion(t *testing.T) {
	_, err := GetSettingsSchemaJSON("99")
	assert.Error(t, err)
}

func TestGetAgentSchemaJSON(t *testing.T) {
	data, err := GetAgentSchemaJSON("1")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), `"Scion Agent Configuration"`)
}

func TestGetAgentSchemaJSON_UnsupportedVersion(t *testing.T) {
	_, err := GetAgentSchemaJSON("99")
	assert.Error(t, err)
}

// --- ValidationError tests ---

func TestValidationError_String(t *testing.T) {
	e := ValidationError{Path: "hub/endpoint", Message: "must be a valid URI"}
	assert.Equal(t, "hub/endpoint: must be a valid URI", e.Error())

	e2 := ValidationError{Path: "", Message: "root-level error"}
	assert.Equal(t, "root-level error", e2.Error())
}

// --- Edge cases ---

func TestValidateSettings_EmptyDocument(t *testing.T) {
	// An empty YAML document should be parsed as nil, which becomes a
	// null value. The schema expects an object, so this should fail.
	data := []byte(``)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.NotEmpty(t, errors, "empty document should fail validation (not an object)")
}

func TestValidateSettings_JSONInput(t *testing.T) {
	// The validator should handle JSON input (which is valid YAML).
	data := []byte(`{"schema_version": "1", "active_profile": "local"}`)
	errors, err := ValidateSettings(data, "1")
	require.NoError(t, err)
	assert.Empty(t, errors, "valid JSON input should pass validation")
}

func TestDetectSettingsFormat_JSONInput(t *testing.T) {
	data := []byte(`{"schema_version": "1", "harness_configs": {}}`)
	version, isLegacy := DetectSettingsFormat(data)
	assert.Equal(t, "1", version)
	assert.False(t, isLegacy)
}

func TestDetectSettingsFormat_LegacyJSON(t *testing.T) {
	data := []byte(`{"harnesses": {"gemini": {"image": "test"}}}`)
	version, isLegacy := DetectSettingsFormat(data)
	assert.Equal(t, "", version)
	assert.True(t, isLegacy)
}
