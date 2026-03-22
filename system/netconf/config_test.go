package netconf

import (
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
