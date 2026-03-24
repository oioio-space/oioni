package main

import (
	"bufio"
	"log"
	"os"
	"strings"
	"time"
)

// logKernelMessages reads /dev/kmsg for 30s after boot and logs lines
// relevant to USB gadget and WiFi to help diagnose hardware issues.
func logKernelMessages(f *os.File) {
	km, err := os.Open("/dev/kmsg")
	if err != nil {
		return
	}
	defer km.Close()

	// /dev/kmsg lines are: "priority,seq,timestamp,flags;message"
	keywords := []string{"brcmfmac", "brcmutil", "usb_f_ecm", "dwc2", "g_cdc", "cdc_ncm", "usb0", "usb1"}

	deadline := time.Now().Add(30 * time.Second)
	scanner := bufio.NewScanner(km)
	for time.Now().Before(deadline) && scanner.Scan() {
		line := scanner.Text()
		// Strip syslog prefix "6,1234,5678,-;actual message"
		msg := line
		if idx := strings.IndexByte(line, ';'); idx >= 0 {
			msg = line[idx+1:]
		}
		for _, kw := range keywords {
			if strings.Contains(strings.ToLower(msg), kw) {
				if f != nil {
					f.WriteString("kmsg: " + msg + "\n")
				}
				log.Printf("kmsg: %s", msg)
				break
			}
		}
	}
}
