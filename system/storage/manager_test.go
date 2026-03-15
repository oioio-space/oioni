package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/oioni/system/storage/usbdetect"
)

// fakeDetector emits a preset list of events then blocks.
type fakeDetector struct {
	events []usbdetect.Event
}

func (f *fakeDetector) Start(ctx context.Context) (<-chan usbdetect.Event, error) {
	ch := make(chan usbdetect.Event, len(f.events))
	for _, ev := range f.events {
		ch <- ev
	}
	return ch, nil
}

// fakeMounter records mount/unmount calls; always succeeds.
type fakeMounter struct {
	mounted   []string
	unmounted []string
}

func (f *fakeMounter) Mount(device, mountpoint, fstype string) error {
	f.mounted = append(f.mounted, device)
	return nil
}
func (f *fakeMounter) Unmount(mountpoint string) error {
	f.unmounted = append(f.unmounted, mountpoint)
	return nil
}
func (f *fakeMounter) DetectFSType(device string) (string, error) {
	return "vfat", nil
}

func TestManager_permVolumeEmitted(t *testing.T) {
	var mounted []string
	m := newManager(
		&fakeDetector{},
		&fakeMounter{},
		"/tmp/fake-perm",
		"/tmp/fake-storage",
	)
	m.OnMount = func(v *Volume) { mounted = append(mounted, v.Name) }

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	m.Start(ctx)

	if len(mounted) == 0 || mounted[0] != "perm" {
		t.Errorf("expected perm to be first mounted, got %v", mounted)
	}
}

func TestManager_usbAddRemove(t *testing.T) {
	var mountedNames, unmountedNames []string

	det := &fakeDetector{events: []usbdetect.Event{
		{Action: "add", Device: "/dev/sda1"},
	}}
	mnt := &fakeMounter{}
	m := newManager(det, mnt, "/tmp/fake-perm", "/tmp/fake-storage")
	m.OnMount = func(v *Volume) { mountedNames = append(mountedNames, v.Name) }
	m.OnUnmount = func(v *Volume) { unmountedNames = append(unmountedNames, v.Name) }

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	m.Start(ctx)

	if len(mountedNames) < 2 {
		t.Fatalf("expected at least 2 mounts, got %v", mountedNames)
	}
	found := false
	for _, n := range mountedNames {
		if strings.Contains(n, "sda1") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected sda1 in mounted, got %v", mountedNames)
	}
}
