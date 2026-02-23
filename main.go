package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/bendahl/uinput"
)

var version = "0.2.0"

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

	appCfg, err := LoadAppConfig(dir)
	if err != nil {
		return fmt.Errorf("load app config: %w", err)
	}

	cfg, err := LoadConfig(dir, appCfg)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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

	for ev := range ch {
		expander.HandleEvent(ev)
	}

	return nil
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
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
		default:
			fmt.Fprintf(os.Stderr, "usage: texpand [init|version]\n")
			os.Exit(1)
		}
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "texpand: %v\n", err)
		os.Exit(1)
	}
}
