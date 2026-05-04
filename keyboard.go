package main

import (
	"fmt"

	evdev "github.com/holoplot/go-evdev"
)

// KeyEvent carries a key code and value (1=press, 0=release, 2=repeat)
// from a keyboard monitoring goroutine.
type KeyEvent struct {
	Code  evdev.EvCode
	Value int32
}

// FindKeyboards enumerates /dev/input/ devices and returns those that
// have both KEY_A and KEY_ENTER capabilities (i.e., physical keyboards).
func FindKeyboards() ([]*evdev.InputDevice, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, fmt.Errorf("list input devices: %w", err)
	}

	var kbds []*evdev.InputDevice
	for _, p := range paths {
		dev, err := evdev.Open(p.Path)
		if err != nil {
			continue
		}

		codes := dev.CapableEvents(evdev.EV_KEY)
		hasA := false
		hasEnter := false
		for _, c := range codes {
			if c == evdev.KEY_A {
				hasA = true
			}
			if c == evdev.KEY_ENTER {
				hasEnter = true
			}
		}

		if hasA && hasEnter {
			// Skip our own virtual keyboard to prevent feedback loops
			name, _ := dev.Name()
			if name == "texpand" {
				dev.Close()
				continue
			}
			kbds = append(kbds, dev)
		} else {
			dev.Close()
		}
	}

	return kbds, nil
}

type monitoredKeyboard struct {
	dev  *evdev.InputDevice
	name string
}

type keyboardMonitorExit struct {
	path string
	dev  *evdev.InputDevice
}

func startKeyboardMonitor(monitors map[string]monitoredKeyboard, dev *evdev.InputDevice, ch chan<- KeyEvent, done chan<- keyboardMonitorExit) {
	path := dev.Path()
	name, _ := dev.Name()
	monitors[path] = monitoredKeyboard{dev: dev, name: name}
	go MonitorKeyboard(dev, ch, done)
}

// RefreshKeyboardMonitors reconciles running keyboard monitors with the
// currently available evdev keyboard devices. It starts monitors for new
// keyboards and closes monitors whose device nodes have disappeared.
func RefreshKeyboardMonitors(monitors map[string]monitoredKeyboard, ch chan<- KeyEvent, done chan<- keyboardMonitorExit) (bool, error) {
	keyboards, err := FindKeyboards()
	if err != nil {
		return false, err
	}

	changed := false
	seen := make(map[string]bool, len(keyboards))
	for _, kb := range keyboards {
		path := kb.Path()
		seen[path] = true
		if _, ok := monitors[path]; ok {
			kb.Close()
			continue
		}

		name, _ := kb.Name()
		fmt.Printf("texpand: keyboard connected: %s\n", name)
		startKeyboardMonitor(monitors, kb, ch, done)
		changed = true
	}

	for path, mon := range monitors {
		if seen[path] {
			continue
		}
		fmt.Printf("texpand: keyboard removed: %s\n", mon.name)
		mon.dev.Close()
		delete(monitors, path)
		changed = true
	}

	return changed, nil
}

// MonitorKeyboard reads events from a single keyboard device and sends key
// events on the channel. It exits when the device is closed or errors, and
// reports the stopped device path so the main loop can rescan hotplugged
// keyboards.
func MonitorKeyboard(dev *evdev.InputDevice, ch chan<- KeyEvent, done chan<- keyboardMonitorExit) {
	path := dev.Path()
	name, _ := dev.Name()
	defer func() {
		done <- keyboardMonitorExit{path: path, dev: dev}
	}()
	for {
		ev, err := dev.ReadOne()
		if err != nil {
			dbg("keyboard monitor stopped: %s (%s): %v", name, path, err)
			return
		}
		if ev.Type == evdev.EV_KEY {
			ch <- KeyEvent{Code: ev.Code, Value: ev.Value}
		}
	}
}
