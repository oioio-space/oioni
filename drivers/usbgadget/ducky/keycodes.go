// Package ducky implements a Ducky Script parser and executor for USB HID gadgets.
//
// Ducky Script is the keystroke-injection scripting language used by Hak5 USB
// Rubber Ducky devices. This package parses scripts and executes them on any
// device implementing the Keyboard or Mouse interfaces (typically a
// *functions.HIDFunc configured as a keyboard or mouse gadget).
//
// Layouts represent the host system's active keyboard layout. Send the layout
// matching the target machine so that HID keycodes produce the expected characters.
//
//go:generate pigeon -o grammar.go grammar.peg
package ducky

// Key is a single HID keyboard entry: the physical keycode and the modifier
// byte to hold while pressing the key (e.g. shift for uppercase or symbols).
type Key struct {
	Keycode   byte // USB HID Usage ID (0x04–0x65)
	Modifiers byte // bitmask of modifier keys to hold simultaneously
}

// Modifier bitmask constants (USB HID keyboard modifier byte, report byte 0).
const (
	ModNone   byte = 0x00
	ModLCtrl  byte = 0x01 // Left Control
	ModLShift byte = 0x02 // Left Shift
	ModLAlt   byte = 0x04 // Left Alt
	ModLGUI   byte = 0x08 // Left GUI (Windows / Command)
	ModRCtrl  byte = 0x10 // Right Control
	ModRShift byte = 0x20 // Right Shift
	ModRAlt   byte = 0x40 // Right Alt (AltGr)
	ModRGUI   byte = 0x80 // Right GUI
)

// HID keycodes for special / named keys (layout-independent).
// Source: USB HID Usage Tables 1.4, table 10 (Keyboard/Keypad page 0x07).
const (
	KeyEnter       byte = 0x28
	KeyEsc         byte = 0x29
	KeyBackspace   byte = 0x2a
	KeyTab         byte = 0x2b
	KeySpace       byte = 0x2c
	KeyMinus       byte = 0x2d // - _
	KeyEqual       byte = 0x2e // = +
	KeyLBracket    byte = 0x2f // [ {
	KeyRBracket    byte = 0x30 // ] }
	KeyBackslash   byte = 0x31 // \ |
	KeySemicolon   byte = 0x33 // ; :
	KeyApostrophe  byte = 0x34 // ' "
	KeyGrave       byte = 0x35 // ` ~
	KeyComma       byte = 0x36 // , <
	KeyDot         byte = 0x37 // . >
	KeySlash       byte = 0x38 // / ?
	KeyCapsLock    byte = 0x39
	KeyF1          byte = 0x3a
	KeyF2          byte = 0x3b
	KeyF3          byte = 0x3c
	KeyF4          byte = 0x3d
	KeyF5          byte = 0x3e
	KeyF6          byte = 0x3f
	KeyF7          byte = 0x40
	KeyF8          byte = 0x41
	KeyF9          byte = 0x42
	KeyF10         byte = 0x43
	KeyF11         byte = 0x44
	KeyF12         byte = 0x45
	KeyPrintScreen byte = 0x46
	KeyScrollLock  byte = 0x47
	KeyPause       byte = 0x48
	KeyInsert      byte = 0x49
	KeyHome        byte = 0x4a
	KeyPageUp      byte = 0x4b
	KeyDelete      byte = 0x4c
	KeyEnd         byte = 0x4d
	KeyPageDown    byte = 0x4e
	KeyRight       byte = 0x4f
	KeyLeft        byte = 0x50
	KeyDown        byte = 0x51
	KeyUp          byte = 0x52
	KeyNumLock     byte = 0x53
	KeyMenu        byte = 0x65
)

// namedKeys maps Ducky Script key names to their HID Key.
// These are layout-independent special/modifier keys.
var namedKeys = map[string]Key{
	// Modifiers
	"CTRL":    {0xe0, ModNone}, // used as modifier, keycode irrelevant
	"CONTROL": {0xe0, ModNone},
	"SHIFT":   {0xe1, ModNone},
	"ALT":     {0xe2, ModNone},
	"GUI":     {0xe3, ModNone},
	"WINDOWS": {0xe3, ModNone},
	"COMMAND": {0xe3, ModNone},

	// Navigation
	"ENTER":      {KeyEnter, ModNone},
	"RETURN":     {KeyEnter, ModNone},
	"ESC":        {KeyEsc, ModNone},
	"ESCAPE":     {KeyEsc, ModNone},
	"TAB":        {KeyTab, ModNone},
	"BACKSPACE":  {KeyBackspace, ModNone},
	"DELETE":     {KeyDelete, ModNone},
	"DEL":        {KeyDelete, ModNone},
	"INSERT":     {KeyInsert, ModNone},
	"HOME":       {KeyHome, ModNone},
	"END":        {KeyEnd, ModNone},
	"PAGEUP":     {KeyPageUp, ModNone},
	"PAGEDOWN":   {KeyPageDown, ModNone},
	"UPARROW":    {KeyUp, ModNone},
	"DOWNARROW":  {KeyDown, ModNone},
	"LEFTARROW":  {KeyLeft, ModNone},
	"RIGHTARROW": {KeyRight, ModNone},
	"SPACE":      {KeySpace, ModNone},

	// Locks
	"CAPSLOCK":   {KeyCapsLock, ModNone},
	"NUMLOCK":    {KeyNumLock, ModNone},
	"SCROLLLOCK": {KeyScrollLock, ModNone},

	// System
	"PRINTSCREEN": {KeyPrintScreen, ModNone},
	"PAUSE":       {KeyPause, ModNone},
	"BREAK":       {KeyPause, ModNone},
	"MENU":        {KeyMenu, ModNone},
	"APP":         {KeyMenu, ModNone},

	// Function keys
	"F1":  {KeyF1, ModNone},
	"F2":  {KeyF2, ModNone},
	"F3":  {KeyF3, ModNone},
	"F4":  {KeyF4, ModNone},
	"F5":  {KeyF5, ModNone},
	"F6":  {KeyF6, ModNone},
	"F7":  {KeyF7, ModNone},
	"F8":  {KeyF8, ModNone},
	"F9":  {KeyF9, ModNone},
	"F10": {KeyF10, ModNone},
	"F11": {KeyF11, ModNone},
	"F12": {KeyF12, ModNone},
}

// Layout maps runes to the HID key needed to produce that character on a
// specific host keyboard layout. Use LayoutEN for QWERTY hosts and LayoutFR
// for AZERTY hosts.
type Layout struct {
	chars map[rune]Key
}

// KeyFor returns the Key for a character rune, or Key{} if not mapped.
func (l *Layout) KeyFor(r rune) (Key, bool) {
	k, ok := l.chars[r]
	return k, ok
}
