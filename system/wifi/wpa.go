// system/wifi/wpa.go — wpa_supplicant control socket protocol
package wifi

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// realWpaConn communicates with wpa_supplicant over a Unix socket.
type realWpaConn struct {
	conn      *net.UnixConn
	localPath string
}

// dialWpa connects to the wpa_supplicant control socket at ctrlPath.
func dialWpa(ctrlPath, localPath string) (*realWpaConn, error) {
	localAddr := &net.UnixAddr{Name: localPath, Net: "unixgram"}
	remoteAddr := &net.UnixAddr{Name: ctrlPath, Net: "unixgram"}

	conn, err := net.DialUnix("unixgram", localAddr, remoteAddr)
	if err != nil {
		os.Remove(localPath)
		return nil, fmt.Errorf("dial wpa socket %s: %w", ctrlPath, err)
	}
	return &realWpaConn{conn: conn, localPath: localPath}, nil
}

func (c *realWpaConn) SendCommand(cmd string) (string, error) {
	if err := c.conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return "", err
	}
	if _, err := c.conn.Write([]byte(cmd)); err != nil {
		return "", fmt.Errorf("wpa send %q: %w", cmd, err)
	}
	buf := make([]byte, 4096)
	n, err := c.conn.Read(buf)
	if err != nil {
		return "", fmt.Errorf("wpa recv %q: %w", cmd, err)
	}
	return strings.TrimRight(string(buf[:n]), "\n"), nil
}

func (c *realWpaConn) Close() error {
	err := c.conn.Close()
	os.Remove(c.localPath)
	return err
}

// parseScanResults parses the SCAN_RESULTS multiline response.
// First line is the header; each subsequent line: bssid\tfreq\tsignal\tflags\tssid
func parseScanResults(raw string) []Network {
	lines := strings.Split(raw, "\n")
	var nets []Network
	for _, line := range lines[1:] { // skip header
		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 5 {
			continue
		}
		sig := 0
		fmt.Sscanf(fields[2], "%d", &sig)
		sec := "Open"
		if strings.Contains(fields[3], "WPA2") {
			sec = "WPA2"
		} else if strings.Contains(fields[3], "WPA") {
			sec = "WPA"
		} else if strings.Contains(fields[3], "WEP") {
			sec = "WEP"
		}
		nets = append(nets, Network{SSID: fields[4], Signal: sig, Security: sec})
	}
	return nets
}

// parseWpaStatus parses the STATUS command response (key=value lines).
func parseWpaStatus(raw string) Status {
	var st Status
	for line := range strings.SplitSeq(raw, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "wpa_state":
			st.State = v
		case "ssid":
			st.SSID = v
		}
	}
	return st
}
