package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed defaults/match/*.yml
var defaultConfigs embed.FS

// initConfig creates the config directory and extracts embedded default
// YAML files, skipping any that already exist.
func initConfig(dir string) error {
	matchDir := filepath.Join(dir, "match")
	if err := os.MkdirAll(matchDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	entries, err := defaultConfigs.ReadDir("defaults/match")
	if err != nil {
		return fmt.Errorf("read embedded defaults: %w", err)
	}

	for _, entry := range entries {
		dst := filepath.Join(matchDir, entry.Name())
		if _, err := os.Stat(dst); err == nil {
			fmt.Printf("  skip %s (already exists)\n", entry.Name())
			continue
		}

		data, err := defaultConfigs.ReadFile("defaults/match/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", entry.Name(), err)
		}

		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		fmt.Printf("  created %s\n", entry.Name())
	}

	return nil
}
