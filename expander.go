package main

import (
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bendahl/uinput"
	evdev "github.com/holoplot/go-evdev"
)

// Expander maintains a rolling keystroke buffer and triggers text
// expansion when a match is detected.
type Expander struct {
	config *Config
	vkbd   uinput.Keyboard
	buf    string
	shift  bool
	maxLen int
}

// NewExpander creates an Expander with the given config and virtual keyboard.
func NewExpander(cfg *Config, vkbd uinput.Keyboard) *Expander {
	maxLen := 0
	for _, m := range cfg.Matches {
		if len(m.Trigger) > maxLen {
			maxLen = len(m.Trigger)
		}
	}
	return &Expander{config: cfg, vkbd: vkbd, maxLen: maxLen}
}

// Reload swaps the config and recalculates maxLen. Typing session state
// (buf, shift) is preserved so in-progress typing is not disrupted.
func (e *Expander) Reload(cfg *Config) {
	e.config = cfg
	e.maxLen = 0
	for _, m := range cfg.Matches {
		if len(m.Trigger) > e.maxLen {
			e.maxLen = len(m.Trigger)
		}
	}
	if len(e.buf) > e.maxLen {
		e.buf = e.buf[len(e.buf)-e.maxLen:]
	}
}

// HandleEvent processes a single key event: tracks shift state, manages
// the buffer, and fires expansions.
func (e *Expander) HandleEvent(ev KeyEvent) {
	// Track shift state
	if ev.Code == evdev.KEY_LEFTSHIFT || ev.Code == evdev.KEY_RIGHTSHIFT {
		e.shift = ev.Value > 0
		return
	}

	// Only process key-down events
	if ev.Value != 1 {
		return
	}

	// Buffer reset keys
	if BufferResetKeys[ev.Code] {
		e.buf = ""
		return
	}

	// Backspace: remove last rune from buffer
	if ev.Code == evdev.KEY_BACKSPACE {
		if len(e.buf) > 0 {
			_, size := utf8.DecodeLastRuneInString(e.buf)
			e.buf = e.buf[:len(e.buf)-size]
		}
		return
	}

	// In "space" mode: check matches on space, then clear buffer
	if e.config.TriggerMode != "immediate" && ev.Code == evdev.KEY_SPACE {
		dbg("space pressed, buffer=%q, checking matches", e.buf)
		for _, m := range e.config.Matches {
			if !strings.HasSuffix(e.buf, m.Trigger) {
				continue
			}
			dbg("match: trigger=%q → expanding", m.Trigger)
			replacement := e.resolveReplacement(m)
			time.Sleep(30 * time.Millisecond)
			// +1 for the space that was just typed
			e.sendBackspaces(utf8.RuneCountInString(m.Trigger) + 1)
			e.clipboardPaste(replacement)
			e.vkbd.KeyPress(uinput.KeySpace)
			break
		}
		e.buf = ""
		return
	}

	// Map keycode to character
	kc, ok := KeyCharMap[ev.Code]
	if !ok {
		return
	}

	ch := kc.Normal
	if e.shift {
		ch = kc.Shifted
	}

	e.buf += ch
	if len(e.buf) > e.maxLen {
		e.buf = e.buf[len(e.buf)-e.maxLen:]
	}

	// In "immediate" mode: check matches after every keystroke
	if e.config.TriggerMode == "immediate" {
		dbg("key '%s', buffer=%q, checking matches", ch, e.buf)
		for _, m := range e.config.Matches {
			if !strings.HasSuffix(e.buf, m.Trigger) {
				continue
			}
			dbg("match: trigger=%q → expanding", m.Trigger)
			replacement := e.resolveReplacement(m)
			time.Sleep(30 * time.Millisecond)
			e.sendBackspaces(utf8.RuneCountInString(m.Trigger))
			e.clipboardPaste(replacement)
			e.buf = ""
			break
		}
	}
}

// resolveReplacement computes the final replacement text for a match,
// resolving date variables and {{ref}} placeholders.
func (e *Expander) resolveReplacement(m Match) string {
	now := time.Now()
	vars := ResolveVars(m.GlobalVars, m.Vars, now)
	return expandRefs(m.Replace, vars)
}

// sendBackspaces sends n backspace key presses via the virtual keyboard.
func (e *Expander) sendBackspaces(n int) {
	for i := 0; i < n; i++ {
		e.vkbd.KeyPress(uinput.KeyBackspace)
		time.Sleep(8 * time.Millisecond)
	}
}

// clipboardPaste copies the text to clipboard via wl-copy, sends Ctrl+V
// to paste, then restores the previous clipboard contents.
// Handles $|$ cursor marker: strips it and moves cursor back after paste.
func (e *Expander) clipboardPaste(text string) {
	// Handle $|$ cursor marker
	cursorOffset := 0
	if idx := strings.Index(text, "$|$"); idx != -1 {
		after := text[idx+3:]
		cursorOffset = utf8.RuneCountInString(after)
		text = text[:idx] + after
	}

	// Save current clipboard
	oldClip, _ := exec.Command("wl-paste", "-n").Output()

	// Copy replacement text
	exec.Command("wl-copy", "--", text).Run()
	time.Sleep(50 * time.Millisecond)

	// Send Ctrl+V
	e.vkbd.KeyDown(uinput.KeyLeftctrl)
	time.Sleep(10 * time.Millisecond)
	e.vkbd.KeyPress(uinput.KeyV)
	time.Sleep(10 * time.Millisecond)
	e.vkbd.KeyUp(uinput.KeyLeftctrl)
	time.Sleep(300 * time.Millisecond)

	// Move cursor back if $|$ was present
	if cursorOffset > 0 {
		time.Sleep(30 * time.Millisecond)
		for i := 0; i < cursorOffset; i++ {
			e.vkbd.KeyPress(uinput.KeyLeft)
			time.Sleep(8 * time.Millisecond)
		}
	}

	// Restore previous clipboard
	if len(oldClip) > 0 {
		cmd := exec.Command("wl-copy", "--", string(oldClip))
		cmd.Start()
	}
}
