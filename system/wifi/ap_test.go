// system/wifi/ap_test.go — unit tests for AP mode (APManager, DHCP server, vif helpers)
package wifi

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
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

func TestAssignIP_BadIface(t *testing.T) {
	// assignIP uses netlink; on dev machine, "uap0" won't exist → expect error, not panic.
	err := assignIP("uap0-notexist", "192.168.4.1/24")
	if err == nil {
		t.Error("expected error for non-existent interface")
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
	ap := newAPManager(cfg, "wlan0", conf, proc, "/user/hostapd", "/user/iw")
	ap.assignIPFn = func(_, _ string) error { return nil } // no netlink in tests
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

	// Expect: createVirtualAP (iw), StartProcess (hostapd)
	// assignIP now uses netlink directly (no subprocess)
	if len(proc.calls) < 2 {
		t.Errorf("expected ≥2 subprocess calls, got %d: %v", len(proc.calls), proc.calls)
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
		newConn: func(_, _ string) (wpaSocket, error) {
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
	cfg := APConfig{SSID: "oioni", PSK: "abc12345", Channel: 6, IP: "192.168.4.1/24"}
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

// ── Bug fix tests ─────────────────────────────────────────────────────────────

// TestAPDHCPServer_AssignIP_PoolEnd255_NoOverflow verifies that assignIP does
// not enter an infinite loop when the pool ends at last-octet 255 and is
// exhausted. Without the fix, ip[3]++ wraps 255→0, ipAfter returns false, and
// the loop never terminates.
func TestAPDHCPServer_AssignIP_PoolEnd255_NoOverflow(t *testing.T) {
	// Pool: 192.168.4.250 – 192.168.4.255 (6 addresses).
	s := &apDHCPServer{
		gw:     net.IP{192, 168, 4, 1},
		start:  net.IP{192, 168, 4, 250},
		end:    net.IP{192, 168, 4, 255},
		leases: make(map[[6]byte]net.IP),
		taken:  make(map[[4]byte]bool),
	}
	// Fill all 6 slots.
	for i := 0; i < 6; i++ {
		s.assignIP([6]byte{0, 0, 0, 0, 0, byte(i)})
	}
	// 7th request: pool exhausted. Should return gw fallback, not loop forever.
	done := make(chan net.IP, 1)
	go func() {
		done <- s.assignIP([6]byte{0, 0, 0, 0, 0, 10})
	}()
	select {
	case ip := <-done:
		if !ip.Equal(s.gw) {
			t.Errorf("expected gw fallback %v, got %v", s.gw, ip)
		}
	case <-time.After(time.Second):
		t.Fatal("assignIP hung on exhausted pool with end[3]==255 (byte overflow bug)")
	}
}

// stallRunner is a processRunner where the Nth StartProcess call (0-based)
// blocks until unblocked via stallCh. It returns a long-running sleep process
// from the stall call so that the goroutine blocks in Wait() if it doesn't
// perform the ctx.Err() check after the restart.
type stallRunner struct {
	mu        sync.Mutex
	callCount int
	stallAt   int
	stalledCh chan struct{} // closed when stalling begins
	stallCh   chan struct{} // closed to unblock
}

func (r *stallRunner) Start(_ string, _ []string) error { return nil }

func (r *stallRunner) StartProcess(_ string, _ []string) (*os.Process, error) {
	r.mu.Lock()
	n := r.callCount
	r.callCount++
	r.mu.Unlock()
	if n == r.stallAt {
		close(r.stalledCh)
		<-r.stallCh
		// Return a long-running process: goroutine blocks in Wait() without the fix.
		return os.StartProcess("/bin/sleep", []string{"/bin/sleep", "3600"}, &os.ProcAttr{})
	}
	return fakeExitedProcess(), nil
}

// TestAPManager_SupervisorExitsAfterContextCancel verifies that the hostapd
// supervisor goroutine exits cleanly when the context is cancelled while it is
// blocked inside StartProcess (the restart race window). Without the fix, the
// goroutine registers the new process and blocks on Wait() indefinitely.
func TestAPManager_SupervisorExitsAfterContextCancel(t *testing.T) {
	r := &stallRunner{
		stallAt:   1, // stall on 2nd StartProcess (first is in Start())
		stalledCh: make(chan struct{}),
		stallCh:   make(chan struct{}),
	}
	dir := t.TempDir()
	ap := newAPManager(APConfig{
		SSID:    "test",
		IP:      "192.168.4.1/24",
		Channel: 6,
	}, "wlan0", &confManager{dir: dir}, r, "/bin/true", "/usr/bin/iw")
	ap.assignIPFn = func(_, _ string) error { return nil }
	ap.restartDelay = time.Millisecond // fast restart for test

	ctx, cancel := context.WithCancel(context.Background())
	if err := ap.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Wait until goroutine is stalling inside second StartProcess.
	select {
	case <-r.stalledCh:
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor goroutine never reached restart")
	}

	// Goroutine is counted here (stalling in StartProcess).
	before := runtime.NumGoroutine()

	cancel()         // context cancelled while goroutine is inside StartProcess
	close(r.stallCh) // unblock StartProcess → goroutine receives sleep proc

	// With fix: goroutine checks ctx.Err() → kills sleep proc → goroutine exits.
	// Without fix: goroutine stores sleep proc, calls proc.Wait() → blocks.
	// Poll for goroutine to exit (count to drop below baseline).
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() < before {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	after := runtime.NumGoroutine()
	if after >= before {
		t.Errorf("supervisor goroutine still running after context cancel+restart "+
			"(before=%d after=%d) — missing ctx.Err() check after StartProcess",
			before, after)
	}
	ap.Stop() // cleanup (kills sleep proc if fix is missing)
}

// TestAPDHCPServer_Stop_NoGoroutineLeak verifies that Stop() cleans up the
// context-watcher goroutine even when the context is never cancelled. Without
// the fix, the watcher goroutine blocks on <-ctx.Done() forever after Stop().
func TestAPDHCPServer_Stop_NoGoroutineLeak(t *testing.T) {
	// Open a random UDP port (no root needed — avoids requiring :67).
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn := pc.(*net.UDPConn)

	srv := &apDHCPServer{
		leases: make(map[[6]byte]net.IP),
		taken:  make(map[[4]byte]bool),
		conn:   conn,
		stopCh: make(chan struct{}),
	}

	ctx := context.Background() // never cancelled
	before := runtime.NumGoroutine()

	// Start the two goroutines exactly as Start() does.
	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()
		defer conn.Close()
		buf := make([]byte, 1500)
		for {
			_, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
		}
	}()
	// Watcher goroutine — buggy version (not in wg, no stopCh).
	leaked := make(chan struct{})
	go func() {
		defer close(leaked)
		select {
		case <-ctx.Done():
		case <-srv.stopCh:
		}
		conn.Close()
	}()

	// Stop closes the connection and should drain both goroutines.
	srv.Stop()

	// Wait for watcher goroutine to finish (it exits via stopCh).
	select {
	case <-leaked:
	case <-time.After(time.Second):
		t.Fatal("watcher goroutine did not exit after Stop() — goroutine leak")
	}

	time.Sleep(20 * time.Millisecond) // let scheduler reap
	after := runtime.NumGoroutine()
	if after > before {
		t.Errorf("goroutine leak: before=%d after=%d", before, after)
	}
}
