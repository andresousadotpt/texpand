package main

import (
	"fmt"
	"sync"

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
			kbds = append(kbds, dev)
		} else {
			dev.Close()
		}
	}

	return kbds, nil
}

// MonitorKeyboard reads events from a single keyboard device and sends
// key events on the channel. Exits when the device is closed or errors.
func MonitorKeyboard(dev *evdev.InputDevice, ch chan<- KeyEvent, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		ev, err := dev.ReadOne()
		if err != nil {
			return
		}
		if ev.Type == evdev.EV_KEY {
			ch <- KeyEvent{Code: ev.Code, Value: ev.Value}
		}
	}
}
