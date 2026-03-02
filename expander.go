package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bendahl/uinput"
	evdev "github.com/holoplot/go-evdev"
)

// ReverseKey maps a character to the uinput key code needed to type it,
// plus whether Shift must be held.
type ReverseKey struct {
	Code  int
	Shift bool
}

// reverseKeyMap maps runes to their key code + shift state, built from KeyCharMap at init.
var reverseKeyMap map[rune]ReverseKey

func init() {
	// evdev key codes are numerically identical to uinput key codes, so we
	// can cast directly.
	reverseKeyMap = make(map[rune]ReverseKey, len(KeyCharMap)*2)
	for evCode, kc := range KeyCharMap {
		code := int(evCode)
		for _, r := range kc.Normal {
			reverseKeyMap[r] = ReverseKey{Code: code, Shift: false}
		}
		for _, r := range kc.Shifted {
			if r != []rune(kc.Normal)[0] { // avoid overwriting if Normal == Shifted (e.g. space)
				reverseKeyMap[r] = ReverseKey{Code: code, Shift: true}
			}
		}
	}
	// Special keys not in KeyCharMap
	reverseKeyMap['\n'] = ReverseKey{Code: uinput.KeyEnter, Shift: false}
	reverseKeyMap['\t'] = ReverseKey{Code: uinput.KeyTab, Shift: false}
}

// hasWtype is true if the wtype binary is available on PATH.
// Checked once at init to avoid repeated lookups.
var hasWtype bool

// wtypeBroken is set to true after the first wtype failure, so we
// skip it on subsequent expansions and go straight to clipboard paste.
var wtypeBroken bool

func init() {
	_, err := exec.LookPath("wtype")
	hasWtype = err == nil
}

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

// canTypeDirectly returns true if every rune in text has a reverse key mapping.
func canTypeDirectly(text string) bool {
	for _, r := range text {
		if _, ok := reverseKeyMap[r]; !ok {
			return false
		}
	}
	return true
}

// typeText types text character-by-character via the virtual keyboard.
// No inter-key delays — uinput events are kernel-FIFO-ordered.
func (e *Expander) typeText(text string) {
	for _, r := range text {
		rk := reverseKeyMap[r]
		if rk.Shift {
			e.vkbd.KeyDown(uinput.KeyLeftshift)
		}
		e.vkbd.KeyPress(rk.Code)
		if rk.Shift {
			e.vkbd.KeyUp(uinput.KeyLeftshift)
		}
	}
}

// performExpansion handles the full expansion sequence: backspace the
// trigger, type/paste the replacement, and position the cursor.
// extraBackspaces is 1 in space mode (to delete the trailing space) and 0 in immediate mode.
func (e *Expander) performExpansion(m Match, extraBackspaces int) {
	replacement := e.resolveReplacement(m)

	// Handle $|$ cursor marker
	cursorOffset := 0
	if idx := strings.Index(replacement, "$|$"); idx != -1 {
		after := replacement[idx+3:]
		cursorOffset = utf8.RuneCountInString(after)
		replacement = replacement[:idx] + after
	}

	e.sendBackspaces(utf8.RuneCountInString(m.Trigger) + extraBackspaces)

	if canTypeDirectly(replacement) {
		dbg("typing directly (%d chars)", utf8.RuneCountInString(replacement))
		e.typeText(replacement)
	} else if hasWtype && !wtypeBroken {
		dbg("using wtype (%d chars, has unmappable runes)", utf8.RuneCountInString(replacement))
		if err := e.wtypeText(replacement); err != nil {
			dbg("wtype broken: %v — disabling, falling back to clipboard", err)
			wtypeBroken = true
			e.clipboardPaste(replacement)
		}
	} else {
		dbg("clipboard paste (%d chars, unmappable runes)", utf8.RuneCountInString(replacement))
		e.clipboardPaste(replacement)
	}

	// Move cursor back if $|$ was present
	if cursorOffset > 0 {
		for i := 0; i < cursorOffset; i++ {
			e.vkbd.KeyPress(uinput.KeyLeft)
		}
	}
}

// HandleEvent processes a single key event: tracks shift state, manages
// the buffer, and fires expansions. Returns true if an expansion was
// performed (caller should drain the event channel).
func (e *Expander) HandleEvent(ev KeyEvent) bool {
	// Track shift state
	if ev.Code == evdev.KEY_LEFTSHIFT || ev.Code == evdev.KEY_RIGHTSHIFT {
		e.shift = ev.Value > 0
		return false
	}

	// Only process key-down events
	if ev.Value != 1 {
		return false
	}

	// Buffer reset keys
	if BufferResetKeys[ev.Code] {
		e.buf = ""
		return false
	}

	// Backspace: remove last rune from buffer
	if ev.Code == evdev.KEY_BACKSPACE {
		if len(e.buf) > 0 {
			_, size := utf8.DecodeLastRuneInString(e.buf)
			e.buf = e.buf[:len(e.buf)-size]
		}
		return false
	}

	// In "space" mode: check matches on space, then clear buffer
	if e.config.TriggerMode != "immediate" && ev.Code == evdev.KEY_SPACE {
		dbg("space pressed, buffer=%q, checking matches", e.buf)
		for _, m := range e.config.Matches {
			if !strings.HasSuffix(e.buf, m.Trigger) {
				continue
			}
			dbg("match: trigger=%q → expanding", m.Trigger)
			e.performExpansion(m, 1) // +1 for the space
			e.buf = ""
			return true
		}
		e.buf = ""
		return false
	}

	// Map keycode to character
	kc, ok := KeyCharMap[ev.Code]
	if !ok {
		return false
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
			e.performExpansion(m, 0)
			e.buf = ""
			return true
		}
	}
	return false
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
	}
}

// wtypeText types text via the wtype Wayland tool. Handles Unicode characters
// that can't be typed via uinput key codes. Returns error on failure.
func (e *Expander) wtypeText(text string) error {
	cmd := exec.Command("wtype", "--", text)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// clipboardPaste copies text to clipboard via wl-copy, sends Ctrl+V to
// paste, then restores the previous clipboard asynchronously.
func (e *Expander) clipboardPaste(text string) {
	// Save current clipboard
	oldClip, _ := exec.Command("wl-paste", "-n").Output()

	// Copy replacement text (.Run() blocks until complete — no extra sleep needed)
	exec.Command("wl-copy", "--", text).Run()

	// Send Ctrl+V to paste
	e.vkbd.KeyDown(uinput.KeyLeftctrl)
	time.Sleep(5 * time.Millisecond)
	e.vkbd.KeyPress(uinput.KeyV)
	time.Sleep(5 * time.Millisecond)
	e.vkbd.KeyUp(uinput.KeyLeftctrl)
	time.Sleep(20 * time.Millisecond)

	// Restore previous clipboard asynchronously
	if len(oldClip) > 0 {
		go func() {
			time.Sleep(200 * time.Millisecond)
			exec.Command("wl-copy", "--", string(oldClip)).Run()
		}()
	}
}
