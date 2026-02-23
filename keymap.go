package main

import evdev "github.com/holoplot/go-evdev"

// KeyChar maps an evdev keycode to its normal and shifted characters.
type KeyChar struct {
	Normal  string
	Shifted string
}

// KeyCharMap maps evdev key codes to their character representations
// for a US/International keyboard layout.
var KeyCharMap = map[evdev.EvCode]KeyChar{
	evdev.KEY_A: {"a", "A"}, evdev.KEY_B: {"b", "B"},
	evdev.KEY_C: {"c", "C"}, evdev.KEY_D: {"d", "D"},
	evdev.KEY_E: {"e", "E"}, evdev.KEY_F: {"f", "F"},
	evdev.KEY_G: {"g", "G"}, evdev.KEY_H: {"h", "H"},
	evdev.KEY_I: {"i", "I"}, evdev.KEY_J: {"j", "J"},
	evdev.KEY_K: {"k", "K"}, evdev.KEY_L: {"l", "L"},
	evdev.KEY_M: {"m", "M"}, evdev.KEY_N: {"n", "N"},
	evdev.KEY_O: {"o", "O"}, evdev.KEY_P: {"p", "P"},
	evdev.KEY_Q: {"q", "Q"}, evdev.KEY_R: {"r", "R"},
	evdev.KEY_S: {"s", "S"}, evdev.KEY_T: {"t", "T"},
	evdev.KEY_U: {"u", "U"}, evdev.KEY_V: {"v", "V"},
	evdev.KEY_W: {"w", "W"}, evdev.KEY_X: {"x", "X"},
	evdev.KEY_Y: {"y", "Y"}, evdev.KEY_Z: {"z", "Z"},

	evdev.KEY_1: {"1", "!"}, evdev.KEY_2: {"2", "@"},
	evdev.KEY_3: {"3", "#"}, evdev.KEY_4: {"4", "$"},
	evdev.KEY_5: {"5", "%"}, evdev.KEY_6: {"6", "^"},
	evdev.KEY_7: {"7", "&"}, evdev.KEY_8: {"8", "*"},
	evdev.KEY_9: {"9", "("}, evdev.KEY_0: {"0", ")"},

	evdev.KEY_MINUS:      {"-", "_"},
	evdev.KEY_EQUAL:      {"=", "+"},
	evdev.KEY_LEFTBRACE:  {"[", "{"},
	evdev.KEY_RIGHTBRACE: {"]", "}"},
	evdev.KEY_SEMICOLON:  {";", ":"},
	evdev.KEY_APOSTROPHE: {"'", "\""},
	evdev.KEY_GRAVE:      {"`", "~"},
	evdev.KEY_BACKSLASH:  {"\\", "|"},
	evdev.KEY_COMMA:      {",", "<"},
	evdev.KEY_DOT:        {".", ">"},
	evdev.KEY_SLASH:      {"/", "?"},
	evdev.KEY_SPACE:      {" ", " "},
}

// BufferResetKeys are keys that clear the typing buffer when pressed.
var BufferResetKeys = map[evdev.EvCode]bool{
	evdev.KEY_ENTER:     true,
	evdev.KEY_ESC:       true,
	evdev.KEY_TAB:       true,
	evdev.KEY_UP:        true,
	evdev.KEY_DOWN:      true,
	evdev.KEY_LEFT:      true,
	evdev.KEY_RIGHT:     true,
	evdev.KEY_HOME:      true,
	evdev.KEY_END:       true,
	evdev.KEY_PAGEUP:    true,
	evdev.KEY_PAGEDOWN:  true,
	evdev.KEY_DELETE:     true,
	evdev.KEY_INSERT:     true,
	evdev.KEY_LEFTCTRL:  true,
	evdev.KEY_RIGHTCTRL: true,
	evdev.KEY_LEFTALT:   true,
	evdev.KEY_RIGHTALT:  true,
	evdev.KEY_LEFTMETA:  true,
	evdev.KEY_RIGHTMETA: true,
}
