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

package templateimport

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeImporter parses Claude Code agent definitions (.claude/agents/*.md).
type ClaudeImporter struct{}

func (c *ClaudeImporter) Detect(path string, frontMatter map[string]any) int {
	score := 0
	// Path-based boost
	if detectFromPath(path) == "claude" {
		score += 3
	}
	if frontMatter != nil {
		score += scoreClaude(frontMatter)
	}
	return score
}

func (c *ClaudeImporter) Parse(path string) (*ImportedAgent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	fm, body, err := ParseFrontMatter(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse front matter in %s: %w", path, err)
	}
	if fm == nil {
		return nil, fmt.Errorf("no front matter found in %s", path)
	}

	agent := &ImportedAgent{
		Harness:        "claude",
		SystemPrompt:   strings.TrimSpace(body),
		RawFrontMatter: fm,
		SourcePath:     path,
	}

	// Extract name
	if name, ok := fm["name"].(string); ok {
		agent.Name = name
	} else {
		// Derive from filename
		agent.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	if desc, ok := fm["description"].(string); ok {
		agent.Description = desc
	}

	// Extract model (pass through as-is; "inherit" means omit)
	if model, ok := fm["model"].(string); ok && model != "inherit" {
		agent.Model = model
	}

	// Extract tools (comma-separated string)
	if tools, ok := fm["tools"].(string); ok {
		for _, t := range strings.Split(tools, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				agent.Tools = append(agent.Tools, t)
			}
		}
	}

	// Extract maxTurns
	if mt, ok := fm["maxTurns"]; ok {
		agent.MaxTurns = toInt(mt)
	}

	return agent, nil
}

func (c *ClaudeImporter) ParseDir(dir string) ([]*ImportedAgent, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var agents []*ImportedAgent
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		agent, err := c.Parse(filepath.Join(dir, e.Name()))
		if err != nil {
			// Skip files that don't parse as valid agent definitions
			continue
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

// toInt converts a YAML-parsed numeric value to int.
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case int64:
		return int(n)
	default:
		return 0
	}
}
