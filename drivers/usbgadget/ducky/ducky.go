package ducky

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Instruction is a parsed Ducky Script command.
type Instruction interface {
	duckyInstruction()
}

// StringInstr types a text string, optionally followed by Enter.
type StringInstr struct {
	Text      string
	WithEnter bool
}

func (StringInstr) duckyInstruction() {}

// DelayInstr pauses execution for the given number of milliseconds.
type DelayInstr struct{ MS int }

func (DelayInstr) duckyInstruction() {}

// DefaultDelayInstr sets the inter-command delay applied after every subsequent instruction.
type DefaultDelayInstr struct{ MS int }

func (DefaultDelayInstr) duckyInstruction() {}

// KeyInstr presses one or more keys simultaneously.
// Each element of Keys is a Ducky Script key name (e.g. "CTRL", "ALT", "DELETE", "a").
type KeyInstr struct {
	Keys []string
}

func (KeyInstr) duckyInstruction() {}

// Keyboard is the minimal interface needed to send HID keyboard reports.
// It is satisfied by *functions.HIDFunc configured as a Keyboard gadget.
type Keyboard interface {
	// WriteReport writes an 8-byte HID keyboard report to the host.
	// Format: [modifier, 0x00, key1, key2, key3, key4, key5, key6]
	WriteReport(report []byte) error
}

// Mouse is the minimal interface needed to send HID mouse reports.
// It is satisfied by *functions.HIDFunc configured as a Mouse gadget.
type Mouse interface {
	// WriteReport writes a 4-byte HID mouse report to the host.
	// Format: [buttons, deltaX, deltaY, wheel]
	WriteReport(report []byte) error
}

// ParseScript parses a Ducky Script string and returns the list of instructions.
// A trailing newline is appended if absent so the last line is not dropped.
// The pigeon-generated Parse function is used internally.
func ParseScript(script string) ([]Instruction, error) {
	if !strings.HasSuffix(script, "\n") {
		script += "\n"
	}
	v, err := Parse("", []byte(script))
	if err != nil {
		return nil, fmt.Errorf("ducky: parse: %w", err)
	}
	if instrs, ok := v.([]Instruction); ok {
		return instrs, nil
	}
	return nil, nil
}

// ExecuteScript parses and executes a Ducky Script on the given keyboard.
// Each character in STRING commands is typed using the given layout (LayoutEN or LayoutFR).
// A 5 ms inter-key delay is applied between keystrokes; DELAY commands override this.
// ctx cancellation stops execution between instructions.
func ExecuteScript(ctx context.Context, kbd Keyboard, script string, layout *Layout) error {
	instrs, err := ParseScript(script)
	if err != nil {
		return err
	}
	defaultDelay := 0
	for _, instr := range instrs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		switch v := instr.(type) {
		case StringInstr:
			if err := TypeString(ctx, kbd, v.Text, layout); err != nil {
				return err
			}
			if v.WithEnter {
				if err := pressNamedKey(kbd, "ENTER"); err != nil {
					return err
				}
				time.Sleep(5 * time.Millisecond)
				if err := releaseKeys(kbd); err != nil {
					return err
				}
			}
		case DelayInstr:
			sleep(ctx, time.Duration(v.MS)*time.Millisecond)
			continue // skip defaultDelay for explicit DELAY
		case DefaultDelayInstr:
			defaultDelay = v.MS
			continue
		case KeyInstr:
			if err := PressKeys(ctx, kbd, v.Keys); err != nil {
				return err
			}
		}
		if defaultDelay > 0 {
			sleep(ctx, time.Duration(defaultDelay)*time.Millisecond)
		}
	}
	return nil
}

// TypeString types a string character-by-character using the given layout.
// A 5 ms inter-key delay is applied between characters.
func TypeString(ctx context.Context, kbd Keyboard, text string, layout *Layout) error {
	for _, r := range text {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		k, ok := layout.KeyFor(r)
		if !ok {
			// Unknown character: skip silently (matches Rubber Ducky behavior).
			continue
		}
		report := keyboardReport(k.Modifiers, k.Keycode)
		if err := kbd.WriteReport(report); err != nil {
			return fmt.Errorf("ducky: type %q: %w", r, err)
		}
		time.Sleep(5 * time.Millisecond)
		if err := kbd.WriteReport(keyboardReport(0, 0)); err != nil {
			return fmt.Errorf("ducky: release %q: %w", r, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

// PressKeys presses a combination of Ducky Script key names simultaneously,
// then releases them all. Examples: ["ENTER"], ["CTRL", "ALT", "DELETE"], ["GUI", "r"].
func PressKeys(ctx context.Context, kbd Keyboard, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	var modifiers byte
	var keycodes []byte
	for _, name := range keys {
		k, ok := namedKeys[strings.ToUpper(name)]
		if !ok {
			// Single character key (e.g. "r", "a", "1").
			if len([]rune(name)) == 1 {
				// Use raw keycode from LayoutEN (physical position is layout-independent
				// for simple printable keys passed via Ducky Script key names).
				r := []rune(name)[0]
				if ek, ok2 := LayoutEN.KeyFor(r); ok2 {
					k = ek
				}
			}
		}
		// Modifier keys: add to modifier byte, no keycode entry needed.
		switch strings.ToUpper(name) {
		case "CTRL", "CONTROL":
			modifiers |= ModLCtrl
		case "SHIFT":
			modifiers |= ModLShift
		case "ALT":
			modifiers |= ModLAlt
		case "GUI", "WINDOWS", "COMMAND":
			modifiers |= ModLGUI
		default:
			if k.Keycode != 0 {
				modifiers |= k.Modifiers
				keycodes = append(keycodes, k.Keycode)
			}
		}
	}
	// Build report: up to 6 simultaneous keycodes.
	report := keyboardReport(modifiers, 0)
	for i, kc := range keycodes {
		if i+2 < len(report) {
			report[2+i] = kc
		}
	}
	if err := kbd.WriteReport(report); err != nil {
		return fmt.Errorf("ducky: press %v: %w", keys, err)
	}
	time.Sleep(5 * time.Millisecond)
	return kbd.WriteReport(keyboardReport(0, 0))
}

// MouseMove sends a relative mouse movement report.
// dx and dy are signed deltas in the range -127 to +127.
func MouseMove(ctx context.Context, m Mouse, dx, dy int8) error {
	report := []byte{0x00, byte(dx), byte(dy), 0x00}
	return m.WriteReport(report)
}

// MouseClick sends a mouse button press-and-release.
// btn: 0x01=left, 0x02=right, 0x04=middle.
func MouseClick(ctx context.Context, m Mouse, btn byte) error {
	if err := m.WriteReport([]byte{btn, 0x00, 0x00, 0x00}); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return m.WriteReport([]byte{0x00, 0x00, 0x00, 0x00})
}

// keyboardReport builds an 8-byte HID keyboard report.
func keyboardReport(modifier, keycode byte) []byte {
	return []byte{modifier, 0x00, keycode, 0x00, 0x00, 0x00, 0x00, 0x00}
}

// pressNamedKey sends a key press for a named key (modifier + keycode).
func pressNamedKey(kbd Keyboard, name string) error {
	k, ok := namedKeys[name]
	if !ok {
		return fmt.Errorf("ducky: unknown key %q", name)
	}
	return kbd.WriteReport(keyboardReport(k.Modifiers, k.Keycode))
}

// releaseKeys sends an all-zero report (all keys released).
func releaseKeys(kbd Keyboard) error {
	return kbd.WriteReport(keyboardReport(0, 0))
}

// sleep sleeps for d or until ctx is cancelled.
func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
