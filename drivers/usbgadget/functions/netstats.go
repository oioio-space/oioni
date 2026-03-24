package functions

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// NetStats holds USB network interface counters from the kernel.
type NetStats struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxDropped uint64
	TxDropped uint64
	RxErrors  uint64
	TxErrors  uint64
}

// findIfaceByMAC returns the name of the network interface whose hardware
// address matches mac (case-insensitive). Used as fallback when configfs
// ifname is not updated by the kernel.
func findIfaceByMAC(mac string) (string, error) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return "", err
	}
	want := strings.ToLower(mac)
	for _, e := range entries {
		addrPath := "/sys/class/net/" + e.Name() + "/address"
		b, err := os.ReadFile(addrPath)
		if err != nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(string(b))) == want {
			return e.Name(), nil
		}
	}
	return "", fmt.Errorf("no interface with MAC %s", mac)
}

// readIfName reads the interface name assigned by the kernel from configfs.
func readIfName(configDir string) (string, error) {
	b, err := os.ReadFile(configDir + "/ifname")
	if err != nil {
		return "", fmt.Errorf("read ifname: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// readNetStats reads interface counters from /sys/class/net/<ifname>/statistics/.
func readNetStats(ifname string) (NetStats, error) {
	base := fmt.Sprintf("/sys/class/net/%s/statistics/", ifname)
	readU64 := func(name string) uint64 {
		b, err := os.ReadFile(base + name)
		if err != nil {
			return 0
		}
		v, _ := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		return v
	}
	return NetStats{
		RxBytes:   readU64("rx_bytes"),
		TxBytes:   readU64("tx_bytes"),
		RxPackets: readU64("rx_packets"),
		TxPackets: readU64("tx_packets"),
		RxDropped: readU64("rx_dropped"),
		TxDropped: readU64("tx_dropped"),
		RxErrors:  readU64("rx_errors"),
		TxErrors:  readU64("tx_errors"),
	}, nil
}
