package functions

import (
	"fmt"
	"os"
)

type midiFunc struct {
	instance string
	bufLen   uint32
	qLen     uint32
}

// MIDI creates a USB MIDI function.
func MIDI() Function {
	return &midiFunc{
		instance: "usb0",
		bufLen:   256,
		qLen:     32,
	}
}

func (f *midiFunc) TypeName() string     { return "midi" }
func (f *midiFunc) InstanceName() string { return f.instance }
func (f *midiFunc) Configure(dir string) error {
	write := func(name string, val uint32) error {
		return os.WriteFile(fmt.Sprintf("%s/%s", dir, name),
			[]byte(fmt.Sprintf("%d\n", val)), 0644)
	}
	if err := write("buflen", f.bufLen); err != nil {
		return err
	}
	return write("qlen", f.qLen)
}
