package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// AppConfig holds global application settings from config.yml.
type AppConfig struct {
	ConfigVersion int    `yaml:"config_version"`
	TriggerMode   string `yaml:"trigger_mode"`
}

// ConfigFile represents a single YAML config file (espanso-compatible).
type ConfigFile struct {
	GlobalVars []VarDef   `yaml:"global_vars"`
	Matches    []MatchDef `yaml:"matches"`
}

// VarDef defines a variable (e.g. a date variable).
type VarDef struct {
	Name   string    `yaml:"name"`
	Type   string    `yaml:"type"`
	Params VarParams `yaml:"params"`
}

// VarParams holds parameters for a variable definition.
type VarParams struct {
	Format string `yaml:"format"`
	Offset int    `yaml:"offset"`
}

// MatchDef is the raw YAML representation of a match entry.
type MatchDef struct {
	Trigger  string   `yaml:"trigger"`
	Triggers []string `yaml:"triggers"`
	Replace  string   `yaml:"replace"`
	Vars     []VarDef `yaml:"vars"`
}

// Match is a resolved, single-trigger match ready for the expander.
type Match struct {
	Trigger    string
	Replace    string
	Vars       []VarDef
	GlobalVars []VarDef
}

// Config holds all loaded matches and the global trigger mode.
type Config struct {
	TriggerMode string
	Matches     []Match
}

// LoadAppConfig reads config.yml from the given config directory.
// Returns a default config (trigger_mode: space) if the file doesn't exist.
func LoadAppConfig(dir string) (*AppConfig, error) {
	cfg := &AppConfig{TriggerMode: "space"}

	data, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config.yml: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config.yml: %w", err)
	}

	return cfg, nil
}

// LoadConfig reads all YAML files from dir/match/ and returns a Config
// with matches sorted longest-trigger-first.
func LoadConfig(dir string, appCfg *AppConfig) (*Config, error) {
	matchDir := filepath.Join(dir, "match")
	files, err := filepath.Glob(filepath.Join(matchDir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("glob config files: %w", err)
	}

	var allMatches []Match

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}

		var cf ConfigFile
		if err := yaml.Unmarshal(data, &cf); err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}

		for _, md := range cf.Matches {
			triggers := []string{md.Trigger}
			if len(md.Triggers) > 0 {
				triggers = md.Triggers
			}

			for _, t := range triggers {
				if t == "" {
					continue
				}
				allMatches = append(allMatches, Match{
					Trigger:    t,
					Replace:    md.Replace,
					Vars:       md.Vars,
					GlobalVars: cf.GlobalVars,
				})
			}
		}
	}

	// Longest trigger first to avoid false matches
	sort.Slice(allMatches, func(i, j int) bool {
		return len(allMatches[i].Trigger) > len(allMatches[j].Trigger)
	})

	return &Config{TriggerMode: appCfg.TriggerMode, Matches: allMatches}, nil
}
