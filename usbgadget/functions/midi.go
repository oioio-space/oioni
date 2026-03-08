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

type MIDIOption func(*midiFunc)

// WithMIDIBufLen sets the buffer length in bytes (default 256).
func WithMIDIBufLen(n uint32) MIDIOption { return func(f *midiFunc) { f.bufLen = n } }

// WithMIDIQLen sets the request queue length (default 32).
func WithMIDIQLen(n uint32) MIDIOption { return func(f *midiFunc) { f.qLen = n } }

// MIDI creates a USB MIDI function.
func MIDI(opts ...MIDIOption) Function {
	f := &midiFunc{instance: "usb0", bufLen: 256, qLen: 32}
	for _, o := range opts {
		o(f)
	}
	return f
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
