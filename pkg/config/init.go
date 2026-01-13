package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ptone/scion-agent/pkg/api"
)

//go:embed all:embeds/*
var EmbedsFS embed.FS

func GetDefaultSettingsData() ([]byte, error) {
	data, err := EmbedsFS.ReadFile("embeds/default_settings.json")
	if err != nil {
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err == nil {
		if local, ok := settings.Profiles["local"]; ok {
			if runtime.GOOS == "darwin" {
				local.Runtime = "container"
			} else {
				local.Runtime = "docker"
			}
			settings.Profiles["local"] = local
			if updated, err := json.MarshalIndent(settings, "", "  "); err == nil {
				return updated, nil
			}
		}
	}
	return data, nil
}

// SeedCommonFiles seeds the common files for a harness template.
// genericEmbedDir is usually "common".
// specificEmbedDir is the harness specific dir in embeds (e.g. "gemini").
func SeedCommonFiles(templateDir, genericEmbedDir, specificEmbedDir, configDirName string, force bool) error {
	homeDir := filepath.Join(templateDir, "home")
	// Create directories
	dirs := []string{
		templateDir,
		homeDir,
		filepath.Join(homeDir, ".config", "gcloud"),
	}
	if configDirName != "" {
		dirs = append(dirs, filepath.Join(homeDir, configDirName))
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Helper to read embedded file
	readEmbed := func(dir, name string) string {
		data, err := EmbedsFS.ReadFile(filepath.Join("embeds", dir, name))
		if err != nil {
			// Fallback to gemini if not found in specific dir
			// Only fallback for non-opencode harnesses
			if dir != "opencode" {
				data, err = EmbedsFS.ReadFile(filepath.Join("embeds", "gemini", name))
				if err == nil {
					return string(data)
				}
			}
			return ""
		}
		return string(data)
	}

	// Try to read scion-agent.json from specific dir.
	// readEmbed handles fallback to gemini if not found (except for opencode).
	scionJSONStr := readEmbed(specificEmbedDir, "scion-agent.json")

	// Seed template files
	files := []struct {
		path    string
		content string
		mode    os.FileMode
	}{
		{filepath.Join(templateDir, "scion-agent.json"), scionJSONStr, 0644},
		{filepath.Join(homeDir, "scion_hook.py"), readEmbed(specificEmbedDir, "scion_hook.py"), 0644},
		{filepath.Join(homeDir, "scion_tool.py"), readEmbed(genericEmbedDir, "scion_tool.py"), 0644},
		{filepath.Join(homeDir, ".bashrc"), readEmbed(specificEmbedDir, "bashrc"), 0644},
		{filepath.Join(homeDir, ".tmux.conf"), readEmbed(genericEmbedDir, ".tmux.conf"), 0644},
	}

	if configDirName != "" {
		files = append(files, []struct {
			path    string
			content string
			mode    os.FileMode
		}{
			{filepath.Join(homeDir, configDirName, "settings.json"), readEmbed(specificEmbedDir, "settings.json"), 0644},
			{filepath.Join(homeDir, configDirName, "system_prompt.md"), readEmbed(specificEmbedDir, "system_prompt.md"), 0644},
		}...)
	}

	for _, f := range files {
		if f.content == "" {
			continue
		}
		baseName := filepath.Base(f.path)
		// Force overwrite for critical config files
		if force || baseName == "settings.json" {
			if err := os.WriteFile(f.path, []byte(f.content), f.mode); err != nil {
				return fmt.Errorf("failed to write file %s: %w", f.path, err)
			}
			continue
		}

		if _, err := os.Stat(f.path); os.IsNotExist(err) {
			if err := os.WriteFile(f.path, []byte(f.content), f.mode); err != nil {
				return fmt.Errorf("failed to write file %s: %w", f.path, err)
			}
		}
	}

	return nil
}



func InitProject(targetDir string, harnesses []api.Harness) error {
	var projectDir string
	var err error

	if targetDir != "" {
		projectDir = targetDir
	} else {
		projectDir, err = GetTargetProjectDir()
		if err != nil {
			return err
		}
	}

	// Create grove-level settings file if it doesn't exist
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}
	settingsPath := filepath.Join(projectDir, "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		// Seed with default settings
		defaultSettings, err := GetDefaultSettingsData()
		if err != nil {
			return fmt.Errorf("failed to read default settings: %w", err)
		}
		if err := os.WriteFile(settingsPath, defaultSettings, 0644); err != nil {
			return fmt.Errorf("failed to seed settings.json: %w", err)
		}
	}

	templatesDir := filepath.Join(projectDir, "templates")
	agentsDir := filepath.Join(projectDir, "agents")

	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	for _, h := range harnesses {
		if err := h.SeedTemplateDir(filepath.Join(templatesDir, h.Name()), false); err != nil {
			return fmt.Errorf("failed to seed %s template: %w", h.Name(), err)
		}
	}

	return nil
}

func InitGlobal(harnesses []api.Harness) error {
	globalDir, err := GetGlobalDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(globalDir, 0755); err != nil {
		return fmt.Errorf("failed to create global directory: %w", err)
	}

	// Create global settings file if it doesn't exist
	settingsPath := filepath.Join(globalDir, "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		defaultSettings, err := GetDefaultSettingsData()
		if err != nil {
			return fmt.Errorf("failed to read default settings: %w", err)
		}
		if err := os.WriteFile(settingsPath, defaultSettings, 0644); err != nil {
			return fmt.Errorf("failed to seed global settings.json: %w", err)
		}
	}

	templatesDir := filepath.Join(globalDir, "templates")
	agentsDir := filepath.Join(globalDir, "agents")

	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create global agents directory: %w", err)
	}

	for _, h := range harnesses {
		if err := h.SeedTemplateDir(filepath.Join(templatesDir, h.Name()), false); err != nil {
			return fmt.Errorf("failed to seed global %s template: %w", h.Name(), err)
		}
	}

	return nil
}
