package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

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
	Trigger   string   `yaml:"trigger"`
	Triggers  []string `yaml:"triggers"`
	Replace   string   `yaml:"replace"`
	Word      bool     `yaml:"word"`
	RightWord bool     `yaml:"right_word"`
	Vars      []VarDef `yaml:"vars"`
}

// Match is a resolved, single-trigger match ready for the expander.
type Match struct {
	Trigger    string
	Replace    string
	Word       bool
	Vars       []VarDef
	GlobalVars []VarDef
}

// Config holds all loaded matches.
type Config struct {
	Matches []Match
}

// LoadConfig reads all YAML files from dir/match/ and returns a Config
// with matches sorted longest-trigger-first.
func LoadConfig(dir string) (*Config, error) {
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

			word := md.Word || md.RightWord

			for _, t := range triggers {
				if t == "" {
					continue
				}
				allMatches = append(allMatches, Match{
					Trigger:    t,
					Replace:    md.Replace,
					Word:       word,
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

	return &Config{Matches: allMatches}, nil
}
