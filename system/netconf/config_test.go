package netconf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfg := &ifaceConfig{dir: dir}
	in := map[string]IfaceCfg{
		"wlan0": {Mode: ModeDHCP},
		"usb0":  {Mode: ModeStatic, IP: "10.0.0.1/24", Gateway: "10.0.0.1", DNS: []string{"8.8.8.8"}},
	}
	if err := cfg.write(in); err != nil {
		t.Fatal(err)
	}
	out, err := cfg.read()
	if err != nil {
		t.Fatal(err)
	}
	if out["usb0"].IP != "10.0.0.1/24" {
		t.Errorf("unexpected IP: %s", out["usb0"].IP)
	}
	if out["wlan0"].Mode != ModeDHCP {
		t.Errorf("unexpected mode: %s", out["wlan0"].Mode)
	}
}

// TestConfigRead_NullJSON verifies that a file containing JSON null does not
// return a nil map. Callers call saved[iface] = cfg on the result, so a nil
// map causes a panic.
func TestConfigRead_NullJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := &ifaceConfig{dir: dir}
	if err := os.WriteFile(filepath.Join(dir, "interfaces.json"), []byte("null"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := cfg.read()
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("read() returned nil map for JSON null — map write would panic")
	}
	// Must be writable.
	m["eth0"] = IfaceCfg{Mode: ModeStatic}
}
