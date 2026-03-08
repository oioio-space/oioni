package usbdetect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSysfs_empty(t *testing.T) {
	root := t.TempDir()
	devs, err := scanSysfs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 0 {
		t.Fatalf("expected 0 devices, got %v", devs)
	}
}

func TestScanSysfs_nonUSB(t *testing.T) {
	root := t.TempDir()
	sda := filepath.Join(root, "sda")
	os.MkdirAll(sda, 0755)
	devs, err := scanSysfs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 0 {
		t.Fatalf("expected 0 devices, got %v", devs)
	}
}

func TestScanSysfs_USBWithPartition(t *testing.T) {
	root := t.TempDir()
	sda := filepath.Join(root, "sda")
	sda1 := filepath.Join(sda, "sda1")
	os.MkdirAll(sda1, 0755)

	deviceDir := filepath.Join(sda, "device")
	os.MkdirAll(deviceDir, 0755)
	subsystemTarget := filepath.Join(t.TempDir(), "bus", "usb")
	os.MkdirAll(subsystemTarget, 0755)
	os.Symlink(subsystemTarget, filepath.Join(deviceDir, "subsystem"))

	devs, err := scanSysfs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 1 || devs[0] != "/dev/sda1" {
		t.Fatalf("expected [/dev/sda1], got %v", devs)
	}
}
