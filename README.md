# texpand

Lightweight Wayland text expander. Reads raw keyboard events via `evdev`, expands triggers via clipboard paste. Works on any Wayland compositor (KDE, GNOME, Hyprland, Sway, etc.).

Single static binary. YAML config (espanso-compatible format). Zero runtime dependencies beyond `wl-clipboard`.

> **Warning**: This was vibe coded. It works, but don't expect anything from it xD.

## How it works

```
[Keyboard] ──evdev──→ texpand ──wl-copy + Ctrl+V──→ [Any App]
```

1. Monitors `/dev/input/event*` devices via evdev (non-exclusive)
2. Maintains a rolling buffer of recent keystrokes
3. On match: backspace the trigger, copy replacement to clipboard, Ctrl+V paste, restore clipboard

Two match modes:

- **Word** (`word: true`, default): fires when space is pressed after the trigger
- **Immediate** (`word: false`): fires as soon as the trigger is typed (accents)

The default mode is controlled by `trigger_mode` in `config.yml`. Individual matches can override it with `word: true` or `word: false`.

## Install

```bash
go install github.com/andresousadotpt/texpand@latest
```

### Initialize config

```bash
texpand init
```

Creates `~/.config/texpand/match/` with default YAML trigger files.

### Set up permissions

texpand reads from `/dev/input/` and writes to `/dev/uinput`.

```bash
# Add your user to the input group
sudo usermod -aG input $USER

# Allow input group to write to /dev/uinput
sudo cp 99-uinput.rules /etc/udev/rules.d/99-uinput.rules
sudo udevadm control --reload-rules && sudo udevadm trigger

# Log out and back in for group change to take effect
```

### Systemd service

```bash
cp texpand.service ~/.config/systemd/user/texpand.service
systemctl --user daemon-reload
systemctl --user enable --now texpand.service
```

## Update

```bash
go install github.com/andresousadotpt/texpand@latest
systemctl --user restart texpand.service
```

To pick up new default config files (without overwriting your existing ones):

```bash
texpand init
```

## Config format

YAML files in `~/.config/texpand/match/*.yml`. Espanso-compatible subset.

### Global settings (`config.yml`)

`~/.config/texpand/config.yml` controls global behavior:

```yaml
# "space" (default) - triggers fire on space
# "immediate" - triggers fire as soon as typed
trigger_mode: space
```

Individual matches override this with `word: true` or `word: false`.

### Simple trigger (fires on space by default)

```yaml
matches:
    - trigger: "'date"
      replace: "{{_date}}"
```

### Immediate trigger (fires as typed, e.g. accents)

```yaml
matches:
    - trigger: "]a"
      replace: "á"
      word: false
```

### Multiple triggers for same replacement

```yaml
matches:
    - triggers: ["'binsh", "'#!"]
      replace: "#!/bin/sh"
      word: true
```

### Date variables

```yaml
global_vars:
    - name: _date
      type: date
      params:
          format: "%d/%m/%Y"

matches:
    - trigger: "'date"
      replace: "{{_date}}"
      word: true
```

### Date with offset (tomorrow/yesterday)

```yaml
matches:
    - trigger: "'tdate"
      replace: "{{tomorrow}}"
      word: true
      vars:
          - name: tomorrow
            type: date
            params:
                format: "%a %m/%d/%Y"
                offset: 86400
```

### Cursor positioning

Use `$|$` to mark where the cursor should land after expansion:

```yaml
matches:
    - trigger: "'11"
      replace: "{{time_with_ampm}} - 1:1 with [$|$]"
      word: true
```

### Supported strftime tokens

| Token | Meaning             | Example |
| ----- | ------------------- | ------- |
| `%Y`  | 4-digit year        | 2026    |
| `%m`  | Month (zero-padded) | 02      |
| `%d`  | Day (zero-padded)   | 23      |
| `%H`  | Hour 24h            | 14      |
| `%I`  | Hour 12h            | 02      |
| `%M`  | Minute              | 30      |
| `%S`  | Second              | 05      |
| `%p`  | AM/PM               | PM      |
| `%a`  | Short weekday       | Mon     |
| `%A`  | Full weekday        | Monday  |
| `%b`  | Short month         | Jan     |
| `%B`  | Full month          | January |

## All default triggers

### Accented characters (fire immediately)

| Trigger | Output | Trigger | Output |
| ------- | ------ | ------- | ------ |
| `]a`    | á      | `]A`    | Á      |
| `}a`    | à      | `}A`    | Á      |
| `~a`    | ã      | `~o`    | õ      |
| `]e`    | é      | `]E`    | É      |
| `}e`    | è      | `}E`    | È      |
| `]i`    | í      | `]I`    | Í      |
| `}i`    | ì      | `}I`    | Ì      |
| `]o`    | ó      | `]O`    | Ó      |
| `}o`    | ò      | `}O`    | Ò      |
| `]u`    | ú      | `]U`    | Ú      |
| `}u`    | ù      | `}U`    | Ù      |
| `'c,`   | ç      |         |        |

### Symbols (fire on space)

| Trigger | Output |
| ------- | ------ |
| `'deg`  | º      |
| `'...`  | ...    |
| `euros` | €      |

### Coding shortcuts (fire on space)

| Trigger          | Output                                    |
| ---------------- | ----------------------------------------- |
| `'binsh` / `'#!` | `#!/bin/sh`                               |
| `'gsm`           | `git switch main && git pull origin main` |
| `'gpomr`         | `git pull origin main --rebase`           |

### Date & time (fire on space)

| Trigger  | Example output                              |
| -------- | ------------------------------------------- |
| `'n`     | `10:56 AM -`                                |
| `'date`  | `23/02/2026`                                |
| `'ddate` | `Mon 23/02/2026`                            |
| `'nn`    | `Mon 23/02/2026 - 10:56 AM -`               |
| `'st`    | `Mon 23/02/2026 - 10:56 AM - meeting start` |
| `'end`   | `Mon 23/02/2026 - 10:56 AM - meeting end`   |
| `'11`    | `10:56 AM - 1:1 with [cursor]`              |
| `'tdate` | Tomorrow's date                             |
| `'ydate` | Yesterday's date                            |

## Adding triggers

Edit or create YAML files in `~/.config/texpand/match/`. Then restart:

```bash
systemctl --user restart texpand.service
```

## Managing the service

```bash
systemctl --user status texpand.service    # Check status
journalctl --user -u texpand.service -f    # View logs
systemctl --user restart texpand.service   # Restart after config changes
systemctl --user stop texpand.service      # Stop
systemctl --user disable texpand.service   # Disable auto-start
```

## Debugging

Run texpand directly in a terminal (not via systemd) to see diagnostic output:

```bash
# Stop the service first to avoid conflicts
systemctl --user stop texpand.service

# Run in foreground — shows detected keyboards and trigger count
./texpand
```

You'll see output like:

```
texpand: monitoring 2 keyboard(s) — 35 triggers loaded
  AT Translated Set 2 keyboard
  Logitech USB Receiver
```

### Checking what config was loaded

Run `texpand init` to see the config directory, then inspect the YAML files:

```bash
texpand init    # Shows config path, skips existing files
ls ~/.config/texpand/match/
```

### Watching events in real time

To see raw kernel input events (useful for verifying your keyboard is detected):

```bash
# List all input devices
ls -la /dev/input/event*

# Watch events from a specific device (Ctrl+C to stop)
# Requires: sudo pacman -S evtest
sudo evtest /dev/input/event0
```

### Checking clipboard operations

If triggers fire but paste wrong text, verify wl-clipboard works:

```bash
echo "test" | wl-copy
wl-paste -n    # Should print "test"
```

### Systemd logs

```bash
# Live logs
journalctl --user -u texpand.service -f

# Last 50 lines
journalctl --user -u texpand.service -n 50

# Since last boot
journalctl --user -u texpand.service -b
```

## Troubleshooting

### "No keyboard devices found"

```bash
groups  # Should include 'input'
sudo usermod -aG input $USER
# Log out and back in
```

### "/dev/uinput" permission denied

```bash
sudo cp 99-uinput.rules /etc/udev/rules.d/99-uinput.rules
sudo udevadm control --reload-rules && sudo udevadm trigger
# If still failing:
sudo modprobe -r uinput && sudo modprobe uinput
ls -la /dev/uinput  # Should show crw-rw---- root input
```

### WAYLAND_DISPLAY not set

texpand auto-detects the Wayland socket at startup. If it fails:

```bash
systemctl --user import-environment WAYLAND_DISPLAY
systemctl --user restart texpand.service
```

### Wrong characters

The keymap assumes US/International layout. Letters and numbers work across layouts, but symbol keys (`]`, `}`, `~`, `'`) may differ.

## License

MIT
