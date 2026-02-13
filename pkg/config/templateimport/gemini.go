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

// GeminiImporter parses Gemini CLI agent definitions (.gemini/agents/*.md).
type GeminiImporter struct{}

func (g *GeminiImporter) Detect(path string, frontMatter map[string]any) int {
	score := 0
	if detectFromPath(path) == "gemini" {
		score += 3
	}
	if frontMatter != nil {
		score += scoreGemini(frontMatter)
	}
	return score
}

func (g *GeminiImporter) Parse(path string) (*ImportedAgent, error) {
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

	// Skip remote agents
	if kind, ok := fm["kind"].(string); ok && kind == "remote" {
		return nil, fmt.Errorf("skipping remote agent in %s (kind: remote not supported)", path)
	}

	agent := &ImportedAgent{
		Harness:        "gemini",
		SystemPrompt:   strings.TrimSpace(body),
		RawFrontMatter: fm,
		SourcePath:     path,
	}

	// Extract name
	if name, ok := fm["name"].(string); ok {
		agent.Name = name
	} else {
		agent.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	if desc, ok := fm["description"].(string); ok {
		agent.Description = desc
	}

	// Extract model (pass through as-is; "inherit" means omit)
	if model, ok := fm["model"].(string); ok && model != "inherit" {
		agent.Model = model
	}

	// Extract tools (YAML array)
	if tools, ok := fm["tools"].([]any); ok {
		for _, t := range tools {
			if ts, ok := t.(string); ok {
				agent.Tools = append(agent.Tools, ts)
			}
		}
	}

	// Extract max_turns
	if mt, ok := fm["max_turns"]; ok {
		agent.MaxTurns = toInt(mt)
	}

	// Extract temperature
	if temp, ok := fm["temperature"]; ok {
		agent.Temperature = toFloat(temp)
	}

	return agent, nil
}

func (g *GeminiImporter) ParseDir(dir string) ([]*ImportedAgent, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var agents []*ImportedAgent
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		agent, err := g.Parse(filepath.Join(dir, e.Name()))
		if err != nil {
			// Skip files that don't parse (including remote agents)
			continue
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

// toFloat converts a YAML-parsed numeric value to float64.
func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}
