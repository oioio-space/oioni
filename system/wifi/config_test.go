package wifi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadConf(t *testing.T) {
	dir := t.TempDir()
	cfg := &confManager{dir: dir}
	networks := []savedNetwork{{SSID: "TestNet", PSK: "secret123"}}
	if err := cfg.write(networks); err != nil {
		t.Fatal(err)
	}
	got, err := cfg.read()
	if err != nil {
		t.Fatal(err)
	}
	// PSK is stored as pre-computed hex PMK to avoid wpa_supplicant passphrase issues.
	wantPMK := wpa2PMK("secret123", "TestNet")
	if len(got) != 1 || got[0].SSID != "TestNet" || got[0].PSK != wantPMK {
		t.Fatalf("unexpected result: %+v", got)
	}
	// Round-trip: write the already-hashed PMK, read back the same PMK (no double-hash).
	if err := cfg.write(got); err != nil {
		t.Fatal(err)
	}
	got2, err := cfg.read()
	if err != nil {
		t.Fatal(err)
	}
	if len(got2) != 1 || got2[0].PSK != wantPMK {
		t.Fatalf("double-hash on round-trip: %+v", got2)
	}
}

func TestMigrateWifiJSON(t *testing.T) {
	dir := t.TempDir()
	// Write legacy wifi.json (gokrazy/wifi format: {"ssid":"OldNet","passphrase":"pw"})
	legacy := filepath.Join(dir, "wifi.json")
	os.WriteFile(legacy, []byte(`{"ssid":"OldNet","passphrase":"oldpass"}`), 0600)

	cfg := &confManager{dir: dir, legacyPath: legacy}
	if err := cfg.migrateIfNeeded(); err != nil {
		t.Fatal(err)
	}
	// Marker file created in conf dir (not renaming /etc/wifi.json which is read-only squashfs)
	if _, err := os.Stat(filepath.Join(dir, ".migrated")); err != nil {
		t.Error("expected .migrated marker to exist in conf dir")
	}
	// Legacy file still present (we can't rename it in production; in test it stays)
	if _, err := os.Stat(legacy); err != nil {
		t.Error("legacy wifi.json should still exist (marker approach, not rename)")
	}
	// conf should now contain OldNet
	nets, err := cfg.read()
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 || nets[0].SSID != "OldNet" || nets[0].PSK != wpa2PMK("oldpass", "OldNet") {
		t.Fatalf("unexpected networks after migration: %+v", nets)
	}

	// Second call must be idempotent (no duplicate entry)
	if err := cfg.migrateIfNeeded(); err != nil {
		t.Fatal(err)
	}
	nets, err = cfg.read()
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 {
		t.Fatalf("expected 1 network after second migrate, got %d: %+v", len(nets), nets)
	}
}

func TestMigrateWifiJSONPskKey(t *testing.T) {
	dir := t.TempDir()
	// wifi.json with "psk" key (actual oioio format)
	legacy := filepath.Join(dir, "wifi.json")
	os.WriteFile(legacy, []byte(`{"ssid":"MyNet","psk":"mypassword"}`), 0600)

	cfg := &confManager{dir: dir, legacyPath: legacy}
	if err := cfg.migrateIfNeeded(); err != nil {
		t.Fatal(err)
	}
	nets, err := cfg.read()
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 || nets[0].SSID != "MyNet" || nets[0].PSK != wpa2PMK("mypassword", "MyNet") {
		t.Fatalf("unexpected networks: %+v", nets)
	}
}
