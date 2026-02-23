package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

const latestConfigVersion = 1

// migration is a named config migration step.
type migration struct {
	version int
	name    string
	run     func(dir string) error
}

var migrations = []migration{
	{version: 1, name: "remove_word_fields", run: removeWordFields},
}

// migrateConfig runs all pending migrations on the config directory.
func migrateConfig(dir string) error {
	configPath := filepath.Join(dir, "config.yml")

	appCfg, err := LoadAppConfig(dir)
	if err != nil {
		return err
	}

	current := appCfg.ConfigVersion
	if current >= latestConfigVersion {
		fmt.Println("texpand: config already up to date")
		return nil
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		fmt.Printf("texpand: running migration %d (%s)\n", m.version, m.name)
		if err := m.run(dir); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.version, m.name, err)
		}
	}

	// Update config_version in config.yml using yaml.Node to preserve comments.
	if err := setConfigVersion(configPath, latestConfigVersion); err != nil {
		return fmt.Errorf("set config_version: %w", err)
	}

	fmt.Println("texpand: migration complete")
	return nil
}

// setConfigVersion updates or inserts config_version in config.yml,
// preserving existing comments and formatting.
func setConfigVersion(path string, version int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No config.yml — create a minimal one.
			content := fmt.Sprintf("config_version: %d\n", version)
			return os.WriteFile(path, []byte(content), 0644)
		}
		return err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse config.yml: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		content := fmt.Sprintf("config_version: %d\n", version)
		return os.WriteFile(path, []byte(content), 0644)
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("config.yml root is not a mapping")
	}

	// Look for existing config_version key.
	found := false
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "config_version" {
			root.Content[i+1].Value = fmt.Sprintf("%d", version)
			root.Content[i+1].Tag = "!!int"
			found = true
			break
		}
	}

	if !found {
		// Prepend config_version as the first key.
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "config_version", Tag: "!!str"}
		valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", version), Tag: "!!int"}
		root.Content = append([]*yaml.Node{keyNode, valNode}, root.Content...)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal config.yml: %w", err)
	}

	return os.WriteFile(path, out, 0644)
}

// removeWordFields removes "word" and "right_word" keys from all match files.
func removeWordFields(dir string) error {
	matchDir := filepath.Join(dir, "match")
	files, err := filepath.Glob(filepath.Join(matchDir, "*.yml"))
	if err != nil {
		return fmt.Errorf("glob match files: %w", err)
	}

	for _, f := range files {
		if err := removeWordFieldsFromFile(f); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

// removeWordFieldsFromFile removes "word" and "right_word" keys from a single
// match YAML file, preserving comments and formatting via yaml.Node.
func removeWordFieldsFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		fmt.Printf("  skip %s (empty)\n", filepath.Base(path))
		return nil
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		fmt.Printf("  skip %s (not a mapping)\n", filepath.Base(path))
		return nil
	}

	// Find the "matches" sequence.
	var matchesNode *yaml.Node
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "matches" {
			matchesNode = root.Content[i+1]
			break
		}
	}

	if matchesNode == nil || matchesNode.Kind != yaml.SequenceNode {
		fmt.Printf("  skip %s (no matches list)\n", filepath.Base(path))
		return nil
	}

	totalRemoved := 0
	for _, entry := range matchesNode.Content {
		if entry.Kind != yaml.MappingNode {
			continue
		}
		removed := removeKeysFromMapping(entry, "word", "right_word")
		totalRemoved += removed
	}

	if totalRemoved == 0 {
		fmt.Printf("  skip %s (nothing to migrate)\n", filepath.Base(path))
		return nil
	}

	// Back up original file.
	bakPath := path + ".bak"
	if err := os.WriteFile(bakPath, data, 0644); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	fmt.Printf("  migrated %s (removed word fields)\n", filepath.Base(path))
	return nil
}

// removeKeysFromMapping removes key/value pairs from a mapping node where the
// key matches any of the given names. Returns the number of pairs removed.
func removeKeysFromMapping(node *yaml.Node, keys ...string) int {
	removed := 0
	filtered := make([]*yaml.Node, 0, len(node.Content))
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]
		if slices.Contains(keys, key.Value) {
			removed++
			continue
		}
		filtered = append(filtered, key, val)
	}
	// Handle odd trailing node (shouldn't happen in valid YAML).
	if len(node.Content)%2 != 0 {
		filtered = append(filtered, node.Content[len(node.Content)-1])
	}
	node.Content = filtered
	return removed
}

