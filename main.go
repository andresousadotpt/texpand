package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bendahl/uinput"
	"github.com/fsnotify/fsnotify"
)

var (
	version  = "dev"
	debugLog bool
)

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			v := info.Main.Version
			if v != "" && v != "(devel)" && !strings.Contains(v, "+dirty") {
				version = strings.TrimPrefix(v, "v")
			}
		}
	}
}

func dbg(format string, args ...any) {
	if debugLog {
		fmt.Fprintf(os.Stderr, "texpand [DEBUG] "+format+"\n", args...)
	}
}

func ensureWaylandEnv() {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "texpand: WARNING: WAYLAND_DISPLAY not set and could not auto-detect\n")
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "wayland-") && !strings.HasSuffix(name, ".lock") {
			os.Setenv("WAYLAND_DISPLAY", name)
			fmt.Printf("texpand: auto-detected %s\n", name)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "texpand: WARNING: WAYLAND_DISPLAY not set and could not auto-detect\n")
}

func configDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "texpand")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "texpand")
}

func run() error {
	ensureWaylandEnv()

	dir := configDir()
	dbg("config directory: %s", dir)

	appCfg, err := LoadAppConfig(dir)
	if err != nil {
		return fmt.Errorf("load app config: %w", err)
	}
	dbg("trigger_mode: %q", appCfg.TriggerMode)

	cfg, err := LoadConfig(dir, appCfg)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	for _, m := range cfg.Matches {
		dbg("  trigger=%q replace=%q", m.Trigger, m.Replace)
	}

	keyboards, err := FindKeyboards()
	if err != nil {
		return fmt.Errorf("find keyboards: %w", err)
	}
	if len(keyboards) == 0 {
		return fmt.Errorf("no keyboard devices found\nMake sure you are in the 'input' group:\n  sudo usermod -aG input $USER\nThen log out and back in")
	}

	vkbd, err := uinput.CreateKeyboard("/dev/uinput", []byte("texpand"))
	if err != nil {
		return fmt.Errorf("create virtual keyboard: %w", err)
	}
	defer vkbd.Close()

	expander := NewExpander(cfg, vkbd)

	fmt.Printf("texpand: monitoring %d keyboard(s) — %d triggers loaded\n",
		len(keyboards), len(cfg.Matches))
	for _, kb := range keyboards {
		name, _ := kb.Name()
		fmt.Printf("  %s\n", name)
	}

	// Watch config directory for changes
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create file watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(dir); err != nil {
		fmt.Fprintf(os.Stderr, "texpand: WARNING: could not watch %s: %v\n", dir, err)
	}
	matchDir := filepath.Join(dir, "match")
	if err := watcher.Add(matchDir); err != nil {
		fmt.Fprintf(os.Stderr, "texpand: WARNING: could not watch %s: %v\n", matchDir, err)
	}

	ch := make(chan KeyEvent, 64)
	var wg sync.WaitGroup

	for _, kb := range keyboards {
		wg.Add(1)
		go MonitorKeyboard(kb, ch, &wg)
	}

	// Clean shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\ntexpand: shutting down")
		for _, kb := range keyboards {
			kb.Close()
		}
		vkbd.Close()
		os.Exit(0)
	}()

	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			expander.HandleEvent(ev)
		case <-debounce.C:
			newAppCfg, err := LoadAppConfig(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "texpand: reload error: %v\n", err)
				continue
			}
			newCfg, err := LoadConfig(dir, newAppCfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "texpand: reload error: %v\n", err)
				continue
			}
			expander.Reload(newCfg)
			fmt.Printf("texpand: config reloaded — %d triggers loaded\n", len(newCfg.Matches))
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if isRelevantChange(event) {
				dbg("config change detected: %s %s", event.Op, event.Name)
				debounce.Reset(500 * time.Millisecond)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "texpand: watch error: %v\n", err)
		}
	}
}

// isRelevantChange returns true if the fsnotify event represents a
// write/create/remove of a .yml file (config or match file change).
func isRelevantChange(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
		return false
	}
	return strings.HasSuffix(event.Name, ".yml")
}

func main() {
	args := os.Args[1:]

	// Parse flags
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "--debug", "-d":
			debugLog = true
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[0])
			fmt.Fprintf(os.Stderr, "usage: texpand [--debug] [init|version|migrate]\n")
			os.Exit(1)
		}
		args = args[1:]
	}

	if len(args) > 0 {
		switch args[0] {
		case "init":
			dir := configDir()
			fmt.Printf("texpand: initializing config in %s\n", dir)
			if err := initConfig(dir); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("texpand: config initialized")
			return
		case "version":
			fmt.Printf("texpand %s\n", version)
			return
		case "migrate":
			dir := configDir()
			if err := migrateConfig(dir); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		default:
			fmt.Fprintf(os.Stderr, "usage: texpand [--debug] [init|version|migrate]\n")
			os.Exit(1)
		}
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "texpand: %v\n", err)
		os.Exit(1)
	}
}
