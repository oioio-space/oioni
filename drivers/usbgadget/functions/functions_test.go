package functions

import (
	"os"
	"path/filepath"
	"testing"
)

// ── C3: HostAddr() ────────────────────────────────────────────────────────────

func TestECMFunc_HostAddr_ReadsConfigfs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "host_addr"), []byte("02:00:00:cc:dd:01\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f := &ECMFunc{configDir: dir}
	got, err := f.HostAddr()
	if err != nil {
		t.Fatal(err)
	}
	if got != "02:00:00:cc:dd:01" {
		t.Errorf("got %q, want %q", got, "02:00:00:cc:dd:01")
	}
}

func TestRNDISFunc_HostAddr_ReadsConfigfs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "host_addr"), []byte("02:00:00:aa:bb:01\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f := &RNDISFunc{configDir: dir}
	got, err := f.HostAddr()
	if err != nil {
		t.Fatal(err)
	}
	if got != "02:00:00:aa:bb:01" {
		t.Errorf("got %q, want %q", got, "02:00:00:aa:bb:01")
	}
}

func TestECMFunc_HostAddr_NotConfigured(t *testing.T) {
	f := &ECMFunc{} // configDir not set
	_, err := f.HostAddr()
	if err == nil {
		t.Error("expected error when configDir is empty")
	}
}

// ── C4: EEM/Subset IfName with MAC fallback ───────────────────────────────────

func TestEEMFunc_IfName_ReturnsValidName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ifname"), []byte("usb0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f := &EEMFunc{configDir: dir}
	got, err := f.IfName()
	if err != nil {
		t.Fatal(err)
	}
	if got != "usb0" {
		t.Errorf("got %q, want %q", got, "usb0")
	}
}

// TestEEMFunc_IfName_SkipsUnnamed verifies that an "unnamed net_device" result
// from configfs does not cause IfName to return it directly. Without the MAC
// fallback fix, it would return the "unnamed" string instead of trying MAC scan.
func TestEEMFunc_IfName_SkipsUnnamed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ifname"), []byte("unnamed net_device\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// No devAddr set, so MAC fallback will fail — but IfName should NOT return "unnamed net_device".
	f := &EEMFunc{configDir: dir}
	name, err := f.IfName()
	if err == nil && name == "unnamed net_device" {
		t.Error("IfName should not return 'unnamed net_device' — MAC fallback not implemented")
	}
}

func TestSubsetFunc_IfName_ReturnsValidName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ifname"), []byte("usb0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f := &SubsetFunc{configDir: dir}
	got, err := f.IfName()
	if err != nil {
		t.Fatal(err)
	}
	if got != "usb0" {
		t.Errorf("got %q, want %q", got, "usb0")
	}
}

func TestSubsetFunc_IfName_SkipsUnnamed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ifname"), []byte("unnamed net_device\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f := &SubsetFunc{configDir: dir}
	name, err := f.IfName()
	if err == nil && name == "unnamed net_device" {
		t.Error("IfName should not return 'unnamed net_device' — MAC fallback not implemented")
	}
}
