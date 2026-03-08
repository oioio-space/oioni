package functions

import (
	"fmt"
	"os"
)

type massStorageFunc struct {
	instance  string
	file      string
	cdrom     bool
	readOnly  bool
	removable bool
}

// MassStorageOption configures a MassStorage function.
type MassStorageOption func(*massStorageFunc)

func WithCDROM(v bool) MassStorageOption    { return func(f *massStorageFunc) { f.cdrom = v } }
func WithReadOnly(v bool) MassStorageOption  { return func(f *massStorageFunc) { f.readOnly = v } }
func WithRemovable(v bool) MassStorageOption { return func(f *massStorageFunc) { f.removable = v } }

// MassStorage creates a USB Mass Storage function.
// file is the path to the disk image (e.g. /perm/disk.img).
func MassStorage(file string, opts ...MassStorageOption) Function {
	f := &massStorageFunc{
		instance:  "usb0",
		file:      file,
		removable: true,
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *massStorageFunc) TypeName() string     { return "mass_storage" }
func (f *massStorageFunc) InstanceName() string { return f.instance }
func (f *massStorageFunc) Configure(dir string) error {
	boolStr := func(v bool) string {
		if v {
			return "1\n"
		}
		return "0\n"
	}
	lun0 := fmt.Sprintf("%s/lun.0", dir)
	if err := os.MkdirAll(lun0, 0755); err != nil {
		return fmt.Errorf("mkdir lun.0: %w", err)
	}
	// cdrom, ro, removable must be set BEFORE file: writing file opens
	// the backing image and subsequent attribute writes return EBUSY.
	if err := os.WriteFile(lun0+"/cdrom", []byte(boolStr(f.cdrom)), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(lun0+"/ro", []byte(boolStr(f.readOnly)), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(lun0+"/removable", []byte(boolStr(f.removable)), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(lun0+"/file", []byte(f.file+"\n"), 0644); err != nil {
		return err
	}
	return nil
}
