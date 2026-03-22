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
	if len(got) != 1 || got[0].SSID != "TestNet" || got[0].PSK != "secret123" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestMigrateWifiJSON(t *testing.T) {
	dir := t.TempDir()
	// Write legacy wifi.json (gokrazy/wifi format: {"ssid":"OldNet","passphrase":"pw"})
	legacy := filepath.Join(dir, "wifi.json")
	os.WriteFile(legacy, []byte(`{"ssid":"OldNet","passphrase":"oldpass"}`), 0600)

	cfg := &confManager{dir: dir}
	if err := cfg.migrateIfNeeded(); err != nil {
		t.Fatal(err)
	}
	// Legacy file renamed
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("expected wifi.json to be renamed after migration")
	}
	if _, err := os.Stat(legacy + ".migrated"); err != nil {
		t.Error("expected wifi.json.migrated to exist")
	}
	// conf should now contain OldNet
	nets, err := cfg.read()
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 || nets[0].SSID != "OldNet" {
		t.Fatalf("unexpected networks after migration: %+v", nets)
	}
}
