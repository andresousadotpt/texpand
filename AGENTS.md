# Agents

Context for AI coding agents working on this project.

## Project overview

**texpand** is a lightweight, single-binary Wayland text expander written in Go. It reads raw keyboard events via evdev, maintains a rolling keystroke buffer, and when a trigger matches it backspaces the trigger text, copies the replacement to clipboard, and pastes via Ctrl+V. Clipboard contents are preserved and restored after paste.

Wayland-only. Requires `wl-clipboard` at runtime and access to `/dev/input/` and `/dev/uinput`.

## Architecture

```
main.go           → Entry point, CLI commands (init, version), signal handling
keyboard.go       → Enumerates /dev/input/ devices, monitors key events via goroutines
keymap.go         → US/International evdev keycode → character mapping (normal + shifted)
expander.go       → Rolling keystroke buffer, trigger matching, clipboard paste, virtual keyboard
config.go         → Loads app config (config.yml) and YAML match files from ~/.config/texpand/match/
config_defaults.go→ Embedded default configs (//go:embed), extracted on `texpand init`
variables.go      → Variable resolution (date type with offset), {{ref}} expansion
strftime.go       → Strftime token replacement (%Y, %m, %d, etc.)
```

### Control flow

```
main() → ensureWaylandEnv() → LoadAppConfig() → LoadConfig()
       → FindKeyboards()
       → MonitorKeyboard() goroutines (one per keyboard) → event channel
       → Expander.HandleEvent() → buffer management → trigger match
       → resolveReplacement() → clipboardPaste() + Ctrl+V
```

### Key patterns

- **Single package (`main`)** — all files are in package main, no internal packages
- **Goroutine per keyboard** — each keyboard device gets its own monitoring goroutine, events are funneled into a single channel
- **Rolling buffer with suffix matching** — buffer is capped to the longest trigger length, matches check `strings.HasSuffix`
- **Longest-trigger-first sorting** — prevents partial false matches
- **Clipboard preservation** — saves clipboard before paste, restores after
- **Timing delays** — strategic `time.Sleep` calls (8-50ms) between virtual keyboard operations for app responsiveness
- **Shift state tracking** — tracks shift key press/release to map correct character (normal vs shifted)
- **Configurable trigger mode** — `config.yml` sets the default (`space` or `immediate`), individual matches override with `word: true/false`

## Build and run

```bash
go build              # compile
go install ./...      # install to $GOPATH/bin
./texpand             # run (needs input group membership + udev rules)
./texpand init        # extract default config to ~/.config/texpand/
./texpand version     # print version
```

No Makefile. No test suite currently. Version is set via `var version` in `main.go`.

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/bendahl/uinput` | Virtual keyboard (backspace, Ctrl+V, arrow keys) |
| `github.com/holoplot/go-evdev` | Raw keyboard event reading from /dev/input/ |
| `gopkg.in/yaml.v3` | YAML config parsing |

Runtime: `wl-copy` and `wl-paste` from `wl-clipboard`.

## Config format

### Global settings (`~/.config/texpand/config.yml`)

```yaml
# "space" (default) - triggers fire when space is pressed after the trigger
# "immediate" - triggers fire as soon as the trigger is typed
trigger_mode: space
```

### Match files (`~/.config/texpand/match/*.yml`)

Espanso-compatible subset.

```yaml
global_vars:
  - name: _date
    type: date
    params:
      format: "%d/%m/%Y"
      offset: 0          # optional: seconds offset for date math

matches:
  - trigger: "]a"
    replace: "á"
    word: false           # immediate mode (overrides global trigger_mode)

  - triggers: ["'binsh", "'#!"]
    replace: "#!/bin/sh"
    word: true            # fires on space

  - trigger: "'date"
    replace: "{{_date}}"  # variable reference — uses global default (space)
```

- `word: true` / `word: false` — override the global `trigger_mode` per match
- `right_word: true` — alias for `word: true`
- `$|$` in replacement — cursor positioning marker (moves cursor back after paste)
- `{{varname}}` — resolved from global_vars or match-level vars

## Conventions

- **Commits**: follow [Conventional Commits](https://www.conventionalcommits.org/) (e.g. `feat:`, `fix:`, `docs:`, `refactor:`, `chore:`)
- **Error handling**: `if err != nil` with `fmt.Errorf("context: %w", err)` wrapping
- **Naming**: PascalCase for exported types/functions, camelCase for locals, UPPER_SNAKE for evdev constants
- **Output**: `fmt.Printf` for normal output, `fmt.Fprintf(os.Stderr, ...)` for errors/warnings
- **No logging framework** — plain fmt prints
- **No tests** — project has no test files currently

## Platform requirements

- Linux with Wayland compositor
- User must be in `input` group
- `/dev/uinput` must be writable (udev rule provided in `99-uinput.rules`)
- US/International keyboard layout assumed for symbol key mapping
