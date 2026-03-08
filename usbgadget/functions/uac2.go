package functions

import (
	"fmt"
	"os"
)

type UAC2Func struct {
	instance  string
	pChmask   uint32 // playback channel mask
	pSrate    uint32 // playback sample rate Hz
	pSsize    uint32 // playback sample size bytes
	cChmask   uint32 // capture channel mask
	cSrate    uint32 // capture sample rate Hz
	cSsize    uint32 // capture sample size bytes
	reqNumber uint32 // number of USB requests
	fbMax     uint32 // max frequency feedback value
}

// UAC2Option configures a USB Audio Class 2 function.
type UAC2Option func(*UAC2Func)

func WithUAC2PlaybackChannels(mask uint32) UAC2Option { return func(f *UAC2Func) { f.pChmask = mask } }
func WithUAC2PlaybackRate(hz uint32) UAC2Option       { return func(f *UAC2Func) { f.pSrate = hz } }
func WithUAC2PlaybackSampleSize(b uint32) UAC2Option  { return func(f *UAC2Func) { f.pSsize = b } }
func WithUAC2CaptureChannels(mask uint32) UAC2Option  { return func(f *UAC2Func) { f.cChmask = mask } }
func WithUAC2CaptureRate(hz uint32) UAC2Option        { return func(f *UAC2Func) { f.cSrate = hz } }
func WithUAC2CaptureSampleSize(b uint32) UAC2Option   { return func(f *UAC2Func) { f.cSsize = b } }
func WithUAC2ReqNumber(n uint32) UAC2Option           { return func(f *UAC2Func) { f.reqNumber = n } }
func WithUAC2FbMax(v uint32) UAC2Option               { return func(f *UAC2Func) { f.fbMax = v } }

// UAC2 creates a USB Audio Class 2 function (speaker + microphone).
// UAC2 supports higher sample rates and bit depths than UAC1 and is natively
// supported on Linux (snd-usb-audio), macOS, and Windows 10+.
// Defaults: stereo, 48 kHz, 16-bit playback and capture.
// Requires the usb_f_uac2 kernel module.
func UAC2(opts ...UAC2Option) *UAC2Func {
	f := &UAC2Func{
		instance:  "usb0",
		pChmask:   3,     // stereo
		pSrate:    48000,
		pSsize:    2,     // 16-bit
		cChmask:   3,
		cSrate:    48000,
		cSsize:    2,
		reqNumber: 2,
		fbMax:     0,
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *UAC2Func) TypeName() string     { return "uac2" }
func (f *UAC2Func) InstanceName() string { return f.instance }
func (f *UAC2Func) Configure(dir string) error {
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
	if f.fbMax != 0 {
		if err := write("fb_max", f.fbMax); err != nil {
			return err
		}
	}
	return nil
}
