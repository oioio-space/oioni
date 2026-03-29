package ducky

import (
	"context"
	"strings"
	"testing"
)

// fakeKbd records all HID reports written to it.
type fakeKbd struct{ reports [][]byte }

func (f *fakeKbd) WriteReport(r []byte) error {
	cp := make([]byte, len(r))
	copy(cp, r)
	f.reports = append(f.reports, cp)
	return nil
}

// ── ParseScript ────────────────────────────────────────────────────────────

func TestParseScript_Empty(t *testing.T) {
	instrs, err := ParseScript("")
	if err != nil {
		t.Fatal(err)
	}
	if len(instrs) != 0 {
		t.Errorf("expected 0 instructions, got %d", len(instrs))
	}
}

func TestParseScript_Rem(t *testing.T) {
	instrs, err := ParseScript("REM this is a comment\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(instrs) != 0 {
		t.Errorf("REM should produce no instructions, got %d", len(instrs))
	}
}

func TestParseScript_String(t *testing.T) {
	instrs, err := ParseScript("STRING Hello World\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(instrs) != 1 {
		t.Fatalf("expected 1 instruction, got %d", len(instrs))
	}
	s, ok := instrs[0].(StringInstr)
	if !ok {
		t.Fatalf("expected StringInstr, got %T", instrs[0])
	}
	if s.Text != "Hello World" || s.WithEnter {
		t.Errorf("unexpected StringInstr: %+v", s)
	}
}

func TestParseScript_StringLn(t *testing.T) {
	instrs, err := ParseScript("STRINGLN Hello\n")
	if err != nil {
		t.Fatal(err)
	}
	s := instrs[0].(StringInstr)
	if s.Text != "Hello" || !s.WithEnter {
		t.Errorf("unexpected StringInstr: %+v", s)
	}
}

func TestParseScript_Delay(t *testing.T) {
	instrs, err := ParseScript("DELAY 500\n")
	if err != nil {
		t.Fatal(err)
	}
	d, ok := instrs[0].(DelayInstr)
	if !ok || d.MS != 500 {
		t.Errorf("expected DelayInstr{500}, got %+v", instrs[0])
	}
}

func TestParseScript_DefaultDelay(t *testing.T) {
	instrs, err := ParseScript("DEFAULT_DELAY 100\n")
	if err != nil {
		t.Fatal(err)
	}
	d, ok := instrs[0].(DefaultDelayInstr)
	if !ok || d.MS != 100 {
		t.Errorf("expected DefaultDelayInstr{100}, got %+v", instrs[0])
	}
}

func TestParseScript_KeyCombo(t *testing.T) {
	instrs, err := ParseScript("CTRL ALT DELETE\n")
	if err != nil {
		t.Fatal(err)
	}
	k, ok := instrs[0].(KeyInstr)
	if !ok {
		t.Fatalf("expected KeyInstr, got %T", instrs[0])
	}
	if strings.Join(k.Keys, " ") != "CTRL ALT DELETE" {
		t.Errorf("unexpected keys: %v", k.Keys)
	}
}

func TestParseScript_SpecialKey(t *testing.T) {
	instrs, err := ParseScript("ENTER\n")
	if err != nil {
		t.Fatal(err)
	}
	k, ok := instrs[0].(KeyInstr)
	if !ok || k.Keys[0] != "ENTER" {
		t.Errorf("unexpected instruction: %+v", instrs[0])
	}
}

func TestParseScript_GUIKey(t *testing.T) {
	instrs, err := ParseScript("GUI r\n")
	if err != nil {
		t.Fatal(err)
	}
	k, ok := instrs[0].(KeyInstr)
	if !ok || len(k.Keys) != 2 || k.Keys[0] != "GUI" || k.Keys[1] != "r" {
		t.Errorf("unexpected instruction: %+v", instrs[0])
	}
}

func TestParseScript_FunctionKeys(t *testing.T) {
	cases := []string{"F1", "F10", "F12"}
	for _, fkey := range cases {
		instrs, err := ParseScript(fkey + "\n")
		if err != nil {
			t.Fatalf("%s: parse error: %v", fkey, err)
		}
		k, ok := instrs[0].(KeyInstr)
		if !ok || k.Keys[0] != fkey {
			t.Errorf("%s: unexpected: %+v", fkey, instrs[0])
		}
	}
}

func TestParseScript_MultiLine(t *testing.T) {
	script := "REM hello\nSTRING test\nDELAY 100\nENTER\n"
	instrs, err := ParseScript(script)
	if err != nil {
		t.Fatal(err)
	}
	if len(instrs) != 3 { // REM produces nil, filtered out
		t.Errorf("expected 3 instructions, got %d: %+v", len(instrs), instrs)
	}
}

func TestParseScript_NoTrailingNewline(t *testing.T) {
	instrs, err := ParseScript("ENTER") // no trailing newline
	if err != nil {
		t.Fatal(err)
	}
	if len(instrs) != 1 {
		t.Errorf("expected 1 instruction without trailing newline, got %d", len(instrs))
	}
}

// ── PressKeys ──────────────────────────────────────────────────────────────

func TestPressKeys_Enter(t *testing.T) {
	kbd := &fakeKbd{}
	if err := PressKeys(context.Background(), kbd, []string{"ENTER"}); err != nil {
		t.Fatal(err)
	}
	// 2 reports: press + release
	if len(kbd.reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(kbd.reports))
	}
	// First report: keycode 0x28 (ENTER), no modifier
	if kbd.reports[0][2] != KeyEnter {
		t.Errorf("expected ENTER keycode 0x%02x, got 0x%02x", KeyEnter, kbd.reports[0][2])
	}
	// Second report: all zeros (release)
	for _, b := range kbd.reports[1] {
		if b != 0 {
			t.Errorf("release report not all zeros: %v", kbd.reports[1])
			break
		}
	}
}

func TestPressKeys_CtrlAltDelete(t *testing.T) {
	kbd := &fakeKbd{}
	if err := PressKeys(context.Background(), kbd, []string{"CTRL", "ALT", "DELETE"}); err != nil {
		t.Fatal(err)
	}
	if len(kbd.reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(kbd.reports))
	}
	mod := kbd.reports[0][0]
	if mod&ModLCtrl == 0 {
		t.Error("expected CTRL modifier bit")
	}
	if mod&ModLAlt == 0 {
		t.Error("expected ALT modifier bit")
	}
	if kbd.reports[0][2] != KeyDelete {
		t.Errorf("expected DELETE keycode, got 0x%02x", kbd.reports[0][2])
	}
}

// ── TypeString ─────────────────────────────────────────────────────────────

func TestTypeString_EN(t *testing.T) {
	kbd := &fakeKbd{}
	if err := TypeString(context.Background(), kbd, "Hi", LayoutEN); err != nil {
		t.Fatal(err)
	}
	// 'H' → shift+h (2 reports: press+release), 'i' → i (2 reports) = 4 total
	if len(kbd.reports) != 4 {
		t.Fatalf("expected 4 reports for 'Hi', got %d", len(kbd.reports))
	}
	// 'H' = keycode 0x0b + shift
	if kbd.reports[0][0] != ModLShift || kbd.reports[0][2] != 0x0b {
		t.Errorf("'H': expected shift+0x0b, got mod=0x%02x kc=0x%02x", kbd.reports[0][0], kbd.reports[0][2])
	}
}

// ── LayoutFR sanity ────────────────────────────────────────────────────────

func TestLayoutFR_AMapping(t *testing.T) {
	// 'a' on AZERTY = physical Q key = keycode 0x14
	k, ok := LayoutFR.KeyFor('a')
	if !ok {
		t.Fatal("LayoutFR: 'a' not mapped")
	}
	if k.Keycode != 0x14 || k.Modifiers != ModNone {
		t.Errorf("LayoutFR 'a': expected {0x14, 0}, got {0x%02x, 0x%02x}", k.Keycode, k.Modifiers)
	}
}

func TestLayoutFR_1IsShifted(t *testing.T) {
	// '1' on AZERTY requires shift + physical 1 key (0x1e)
	k, ok := LayoutFR.KeyFor('1')
	if !ok {
		t.Fatal("LayoutFR: '1' not mapped")
	}
	if k.Keycode != 0x1e || k.Modifiers != ModLShift {
		t.Errorf("LayoutFR '1': expected {0x1e, shift}, got {0x%02x, 0x%02x}", k.Keycode, k.Modifiers)
	}
}

// ── MouseClick ─────────────────────────────────────────────────────────────

func TestMouseClick_Left(t *testing.T) {
	m := &fakeKbd{}
	if err := MouseClick(context.Background(), m, 0x01); err != nil {
		t.Fatal(err)
	}
	if len(m.reports) != 2 {
		t.Fatalf("expected 2 mouse reports, got %d", len(m.reports))
	}
	if m.reports[0][0] != 0x01 {
		t.Errorf("left click: expected button 0x01, got 0x%02x", m.reports[0][0])
	}
}
