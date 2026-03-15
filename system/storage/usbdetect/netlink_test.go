package usbdetect

import (
	"testing"
)

func TestParseUevent_blockPartitionAdd(t *testing.T) {
	msg := "add@/devices/pci0000:00/xhci/usb1/1-1/sda/sda1\x00" +
		"ACTION=add\x00" +
		"DEVTYPE=partition\x00" +
		"SUBSYSTEM=block\x00" +
		"DEVNAME=sda1\x00"

	ev, ok := parseUevent([]byte(msg))
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Action != "add" {
		t.Errorf("Action: got %q, want %q", ev.Action, "add")
	}
	if ev.Device != "/dev/sda1" {
		t.Errorf("Device: got %q, want %q", ev.Device, "/dev/sda1")
	}
}

func TestParseUevent_nonBlock(t *testing.T) {
	msg := "add@/devices/usb1\x00" +
		"ACTION=add\x00" +
		"SUBSYSTEM=usb\x00" +
		"DEVTYPE=usb_device\x00"

	_, ok := parseUevent([]byte(msg))
	if ok {
		t.Fatal("expected ok=false for non-block subsystem")
	}
}

func TestParseUevent_diskNotPartition(t *testing.T) {
	msg := "add@/devices/sda\x00" +
		"ACTION=add\x00" +
		"SUBSYSTEM=block\x00" +
		"DEVTYPE=disk\x00" +
		"DEVNAME=sda\x00"

	_, ok := parseUevent([]byte(msg))
	if ok {
		t.Fatal("expected ok=false for disk (not partition)")
	}
}

func TestParseUevent_remove(t *testing.T) {
	msg := "remove@/devices/sda/sda1\x00" +
		"ACTION=remove\x00" +
		"SUBSYSTEM=block\x00" +
		"DEVTYPE=partition\x00" +
		"DEVNAME=sda1\x00"

	ev, ok := parseUevent([]byte(msg))
	if !ok {
		t.Fatal("expected ok=true for remove event")
	}
	if ev.Action != "remove" {
		t.Errorf("Action: got %q, want %q", ev.Action, "remove")
	}
}
