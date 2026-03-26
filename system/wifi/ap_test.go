// system/wifi/ap_test.go — unit tests for AP mode (APManager, DHCP server, vif helpers)
package wifi

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// ── fakeAPProcess ─────────────────────────────────────────────────────────────

// fakeAPProcess records subprocess calls and optionally returns a real
// *os.Process so APManager can call Wait() without panic.
type fakeAPProcess struct {
	calls [][]string
}

func (f *fakeAPProcess) Start(_ string, args []string) error {
	f.calls = append(f.calls, args)
	return nil
}

func (f *fakeAPProcess) StartProcess(_ string, args []string) (*os.Process, error) {
	f.calls = append(f.calls, args)
	// Return a no-op process (already-exited "sleep 0").
	cmd := fakeExitedProcess()
	return cmd, nil
}

// fakeExitedProcess starts a subprocess that exits immediately.
// On gokrazy there is no shell, but in tests we run on Linux with /bin/true.
func fakeExitedProcess() *os.Process {
	// Use /bin/true — always available on the dev machine.
	proc, _ := os.StartProcess("/bin/true", []string{"/bin/true"}, &os.ProcAttr{})
	if proc != nil {
		_, _ = proc.Wait() // reap it so tests don't leak zombies
	}
	return proc
}

// ── createVirtualAP / deleteVirtualAP ─────────────────────────────────────────

func TestCreateVirtualAP(t *testing.T) {
	proc := &fakeAPProcess{}
	if err := createVirtualAP(proc, "/user/iw", "wlan0", "uap0"); err != nil {
		t.Fatal(err)
	}
	if len(proc.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(proc.calls))
	}
	want := []string{"dev", "wlan0", "interface", "add", "uap0", "type", "__ap"}
	for i, a := range want {
		if proc.calls[0][i] != a {
			t.Errorf("arg[%d] = %q, want %q", i, proc.calls[0][i], a)
		}
	}
}

func TestDeleteVirtualAP(t *testing.T) {
	proc := &fakeAPProcess{}
	if err := deleteVirtualAP(proc, "/user/iw", "uap0"); err != nil {
		t.Fatal(err)
	}
	if len(proc.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(proc.calls))
	}
	want := []string{"dev", "uap0", "del"}
	for i, a := range want {
		if proc.calls[0][i] != a {
			t.Errorf("arg[%d] = %q, want %q", i, proc.calls[0][i], a)
		}
	}
}

func TestAssignIP(t *testing.T) {
	proc := &fakeAPProcess{}
	if err := assignIP(proc, "/user/ip", "uap0", "192.168.4.1/24"); err != nil {
		t.Fatal(err)
	}
	if len(proc.calls) != 2 {
		t.Fatalf("expected 2 calls (addr add + link set), got %d", len(proc.calls))
	}
}

// ── APManager ─────────────────────────────────────────────────────────────────

func newTestAPManager(t *testing.T) (*APManager, *fakeAPProcess) {
	proc := &fakeAPProcess{}
	dir := t.TempDir()
	cfg := APConfig{
		SSID:    "TestAP",
		PSK:     "secret123",
		Channel: 6,
		IP:      "192.168.4.1/24",
		DNS:     []string{"8.8.8.8"},
	}
	conf := &confManager{dir: dir}
	ap := newAPManager(cfg, conf, proc, "/user/hostapd", "/user/iw", "/user/ip")
	return ap, proc
}

func TestAPManager_Start_CallsSubprocesses(t *testing.T) {
	ap, proc := newTestAPManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ap.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer ap.Stop()

	// Expect: createVirtualAP, assignIP (×2), StartProcess (hostapd)
	// = at least 4 calls
	if len(proc.calls) < 4 {
		t.Errorf("expected ≥4 subprocess calls, got %d: %v", len(proc.calls), proc.calls)
	}
}

func TestAPManager_Start_WritesHostapdConf(t *testing.T) {
	ap, _ := newTestAPManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ap.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer ap.Stop()

	confPath := filepath.Join(ap.conf.dir, "hostapd.conf")
	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("hostapd.conf not written: %v", err)
	}
	if string(data) == "" {
		t.Error("hostapd.conf is empty")
	}
}

func TestAPManager_Stop_CallsDelete(t *testing.T) {
	ap, proc := newTestAPManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ap.Start(ctx); err != nil {
		t.Fatal(err)
	}
	callsBefore := len(proc.calls)
	ap.Stop()

	// Stop must call deleteVirtualAP → at least one more call.
	if len(proc.calls) <= callsBefore {
		t.Error("Stop() did not call deleteVirtualAP")
	}
}

func TestAPManager_Status_NotRunning(t *testing.T) {
	ap, _ := newTestAPManager(t)
	st := ap.Status()
	if st.Running {
		t.Error("Status.Running should be false before Start")
	}
}

// ── Mode persistence ───────────────────────────────────────────────────────────

func TestConfManager_WriteReadMode(t *testing.T) {
	c := &confManager{dir: t.TempDir()}
	m := Mode{STA: true, AP: true}
	if err := c.writeMode(m); err != nil {
		t.Fatal(err)
	}
	got, err := c.readMode()
	if err != nil {
		t.Fatal(err)
	}
	if got != m {
		t.Errorf("got %+v, want %+v", got, m)
	}
}

func TestConfManager_ReadMode_Default(t *testing.T) {
	c := &confManager{dir: t.TempDir()}
	m, err := c.readMode()
	if err != nil {
		t.Fatal(err)
	}
	if m.STA || m.AP {
		t.Errorf("default mode should be zero, got %+v", m)
	}
}

func TestConfManager_WriteReadAPConfig(t *testing.T) {
	c := &confManager{dir: t.TempDir()}
	cfg := APConfig{SSID: "Pi0", PSK: "pass", Channel: 11, IP: "10.0.0.1/24", DNS: []string{"1.1.1.1"}}
	if err := c.writeAPConfig(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := c.readAPConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.SSID != cfg.SSID || got.Channel != cfg.Channel || got.IP != cfg.IP {
		t.Errorf("got %+v, want %+v", got, cfg)
	}
}

// ── DHCP server ───────────────────────────────────────────────────────────────

func TestAPDHCPServer_AssignIP_Unique(t *testing.T) {
	cfg := APConfig{IP: "192.168.4.1/24", DNS: []string{"8.8.8.8"}}
	s := newAPDHCPServer("uap0", cfg)

	mac1 := [6]byte{0, 1, 2, 3, 4, 5}
	mac2 := [6]byte{0, 1, 2, 3, 4, 6}

	ip1 := s.assignIP(mac1)
	ip2 := s.assignIP(mac2)

	if ip1.Equal(ip2) {
		t.Errorf("same IP assigned to two different MACs: %v", ip1)
	}
}

func TestAPDHCPServer_AssignIP_Stable(t *testing.T) {
	cfg := APConfig{IP: "192.168.4.1/24", DNS: []string{"8.8.8.8"}}
	s := newAPDHCPServer("uap0", cfg)

	mac := [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	ip1 := s.assignIP(mac)
	ip2 := s.assignIP(mac)
	if !ip1.Equal(ip2) {
		t.Errorf("IP changed between calls: %v → %v", ip1, ip2)
	}
}

func TestAPDHCPServer_AssignIP_InPool(t *testing.T) {
	cfg := APConfig{IP: "192.168.4.1/24", DNS: []string{"8.8.8.8"}}
	s := newAPDHCPServer("uap0", cfg)

	mac := [6]byte{1, 2, 3, 4, 5, 6}
	ip := s.assignIP(mac)

	last := ip.To4()[3]
	if last < 100 || last > 200 {
		t.Errorf("assigned IP %v is outside pool (.100-.200)", ip)
	}
}

func TestAPDHCPServer_ClientCount(t *testing.T) {
	cfg := APConfig{IP: "192.168.4.1/24", DNS: []string{"8.8.8.8"}}
	s := newAPDHCPServer("uap0", cfg)

	if s.ClientCount() != 0 {
		t.Error("expected 0 clients initially")
	}
	s.assignIP([6]byte{1, 2, 3, 4, 5, 6})
	s.assignIP([6]byte{1, 2, 3, 4, 5, 7})
	if s.ClientCount() != 2 {
		t.Errorf("expected 2 clients, got %d", s.ClientCount())
	}
}

func TestAPDHCPServer_ParseCIDR_Invalid(t *testing.T) {
	// Should not panic on bad IP — falls back to defaults.
	cfg := APConfig{IP: "bad-cidr", DNS: nil}
	s := newAPDHCPServer("uap0", cfg)
	if s.gw == nil {
		t.Error("gateway should not be nil after fallback")
	}
}

// ── Manager.SetMode / GetMode ──────────────────────────────────────────────────

func newTestManagerForMode(t *testing.T) *Manager {
	dir := t.TempDir()
	proc := &fakeProcess{}
	wpa := &fakeWpa{responses: map[string]string{}}
	m := &Manager{
		cfg:  Config{ConfDir: dir, Iface: "wlan0"},
		conf: &confManager{dir: dir},
		proc: proc,
		newConn: func(_, _ string) (wpaConn, error) {
			return wpa, nil
		},
	}
	return m
}

func TestManager_GetMode_Default(t *testing.T) {
	m := newTestManagerForMode(t)
	mode := m.GetMode()
	if mode.STA || mode.AP {
		t.Errorf("default mode should be zero, got %+v", mode)
	}
}

func TestManager_SetMode_PersistsMode(t *testing.T) {
	m := newTestManagerForMode(t)
	ctx := context.Background()

	want := Mode{STA: true, AP: false}
	if err := m.SetMode(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err := m.conf.readMode()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("persisted mode = %+v, want %+v", got, want)
	}
}

func TestManager_SetMode_GetMode_Roundtrip(t *testing.T) {
	m := newTestManagerForMode(t)
	ctx := context.Background()

	want := Mode{STA: true, AP: false}
	if err := m.SetMode(ctx, want); err != nil {
		t.Fatal(err)
	}
	if got := m.GetMode(); got != want {
		t.Errorf("GetMode() = %+v, want %+v", got, want)
	}
}

func TestManager_SetMode_APRequiresSSID(t *testing.T) {
	m := newTestManagerForMode(t)
	ctx := context.Background()
	// No APConfig set → SSID is empty → SetMode must return error.
	err := m.SetMode(ctx, Mode{STA: false, AP: true})
	if err == nil {
		t.Error("SetMode with AP=true and no SSID should return error")
	}
}

func TestManager_APStatus_OffByDefault(t *testing.T) {
	m := newTestManagerForMode(t)
	st := m.APStatus()
	if st.Running {
		t.Error("APStatus.Running should be false when AP mode is off")
	}
}

func TestManager_SetAPConfig_Roundtrip(t *testing.T) {
	m := newTestManagerForMode(t)
	cfg := APConfig{SSID: "oioni", PSK: "abc123", Channel: 6, IP: "192.168.4.1/24"}
	if err := m.SetAPConfig(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetAPConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.SSID != cfg.SSID {
		t.Errorf("SSID = %q, want %q", got.SSID, cfg.SSID)
	}
}

// ── dhcpMsgType parser ─────────────────────────────────────────────────────────

func TestDhcpMsgType_Discover(t *testing.T) {
	opts := []byte{53, 1, 1, 255} // option 53 = DISCOVER (1)
	if got := dhcpMsgType(opts); got != 1 {
		t.Errorf("got %d, want 1 (DISCOVER)", got)
	}
}

func TestDhcpMsgType_Request(t *testing.T) {
	opts := []byte{53, 1, 3, 255} // REQUEST
	if got := dhcpMsgType(opts); got != 3 {
		t.Errorf("got %d, want 3 (REQUEST)", got)
	}
}

func TestDhcpMsgType_Missing(t *testing.T) {
	opts := []byte{255} // just END
	if got := dhcpMsgType(opts); got != 0 {
		t.Errorf("got %d, want 0 (not found)", got)
	}
}

// ── ipAfter helper ────────────────────────────────────────────────────────────

func TestIPAfter(t *testing.T) {
	a := net.ParseIP("192.168.4.101").To4()
	b := net.ParseIP("192.168.4.100").To4()
	if !ipAfter(a, b) {
		t.Error("ipAfter(.101, .100) should be true")
	}
	if ipAfter(b, a) {
		t.Error("ipAfter(.100, .101) should be false")
	}
}
