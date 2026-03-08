package functions

import (
	"fmt"
	"os"
)

type UAC1Func struct {
	instance   string
	pChmask    uint32 // playback channel mask (e.g. 3 = stereo)
	pSrate     uint32 // playback sample rate Hz
	pSsize     uint32 // playback sample size bytes (1, 2, 3, or 4)
	cChmask    uint32 // capture channel mask
	cSrate     uint32 // capture sample rate Hz
	cSsize     uint32 // capture sample size bytes
	reqNumber  uint32 // number of USB requests in flight
}

// UAC1Option configures a USB Audio Class 1 function.
type UAC1Option func(*UAC1Func)

func WithUAC1PlaybackChannels(mask uint32) UAC1Option  { return func(f *UAC1Func) { f.pChmask = mask } }
func WithUAC1PlaybackRate(hz uint32) UAC1Option        { return func(f *UAC1Func) { f.pSrate = hz } }
func WithUAC1PlaybackSampleSize(b uint32) UAC1Option   { return func(f *UAC1Func) { f.pSsize = b } }
func WithUAC1CaptureChannels(mask uint32) UAC1Option   { return func(f *UAC1Func) { f.cChmask = mask } }
func WithUAC1CaptureRate(hz uint32) UAC1Option         { return func(f *UAC1Func) { f.cSrate = hz } }
func WithUAC1CaptureSampleSize(b uint32) UAC1Option    { return func(f *UAC1Func) { f.cSsize = b } }
func WithUAC1ReqNumber(n uint32) UAC1Option            { return func(f *UAC1Func) { f.reqNumber = n } }

// UAC1 creates a USB Audio Class 1 function (speaker + microphone).
// UAC1 is isochronous and requires no driver on Windows/macOS/Linux — it
// appears as a standard USB audio device. Defaults: stereo, 48 kHz, 16-bit.
// Requires the usb_f_uac1 kernel module.
func UAC1(opts ...UAC1Option) *UAC1Func {
	f := &UAC1Func{
		instance:  "usb0",
		pChmask:   3,     // stereo (channels 0 and 1)
		pSrate:    48000,
		pSsize:    2,     // 16-bit
		cChmask:   3,
		cSrate:    48000,
		cSsize:    2,
		reqNumber: 2,
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *UAC1Func) TypeName() string     { return "uac1" }
func (f *UAC1Func) InstanceName() string { return f.instance }
func (f *UAC1Func) Configure(dir string) error {
	write := func(name string, val uint32) error {
		return os.WriteFile(fmt.Sprintf("%s/%s", dir, name),
			[]byte(fmt.Sprintf("%d\n", val)), 0644)
	}
	for _, kv := range []struct {
		k string
		v uint32
	}{
		{"p_chmask", f.pChmask},
		{"p_srate", f.pSrate},
		{"p_ssize", f.pSsize},
		{"c_chmask", f.cChmask},
		{"c_srate", f.cSrate},
		{"c_ssize", f.cSsize},
		{"req_number", f.reqNumber},
	} {
		if err := write(kv.k, kv.v); err != nil {
			return err
		}
	}
	return nil
}
