# Contributing

Thanks for your interest in contributing to texpand!

## Getting started

```bash
git clone https://github.com/andresousadotpt/texpand.git
cd texpand
go build
```

### Requirements

- Go 1.24+
- Linux with Wayland
- `wl-clipboard` (`wl-copy` and `wl-paste`)
- User in the `input` group with `/dev/uinput` writable (see README for udev setup)

## Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new trigger mode
fix: handle empty clipboard on paste
docs: update config format section
refactor: simplify buffer management
chore: update dependencies
```

Keep commits small and focused on a single change.

## Project structure

```
main.go            Entry point, CLI, signal handling
keyboard.go        Keyboard device discovery and monitoring
keymap.go          Evdev keycode → character mapping
expander.go        Keystroke buffer, trigger matching, clipboard paste
config.go          App config + match file loading
config_defaults.go Embedded defaults, `texpand init`
variables.go       Variable resolution (date/time)
strftime.go        Strftime token replacement
```

All code lives in `package main`. No internal packages.

## Adding triggers

Default trigger files are embedded from `defaults/match/*.yml` and extracted on `texpand init`. To add new default triggers, create or edit a YAML file in `defaults/match/`.

The global config is in `defaults/config.yml`.

## Code style

- Standard Go formatting (`gofmt`)
- Error wrapping: `fmt.Errorf("context: %w", err)`
- No logging framework — use `fmt.Printf` / `fmt.Fprintf(os.Stderr, ...)`
- No external test framework — standard `testing` package if adding tests

## Submitting changes

1. Fork the repo and create a branch
2. Make your changes
3. Verify it builds: `go build`
4. Test manually (run `./texpand` in a terminal)
5. Open a pull request with a clear description of what changed and why
