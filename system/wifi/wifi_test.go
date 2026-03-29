package wifi

import (
	"errors"
	"os"
	"sync"
	"testing"
)

// fakeProcess satisfies processRunner for tests.
type fakeProcess struct{ started bool }

func (f *fakeProcess) Start(_ string, _ []string) error { f.started = true; return nil }
func (f *fakeProcess) StartProcess(_ string, _ []string) (*os.Process, error) {
	return nil, nil
}

// fakeWpa satisfies wpaSocket for tests.
type fakeWpa struct {
	mu        sync.Mutex
	responses map[string]string
	commands  []string
	closed    bool
}

func (f *fakeWpa) SendCommand(cmd string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, cmd)
	if r, ok := f.responses[cmd]; ok {
		return r, nil
	}
	return "OK", nil
}
func (f *fakeWpa) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// newTestManager creates a Manager with fakeWpa pre-wired (bypasses Start).
func newTestManager(t *testing.T, wpa *fakeWpa) *Manager {
	t.Helper()
	dir := t.TempDir()
	proc := &fakeProcess{}
	m := &Manager{
		cfg:     Config{ConfDir: dir, Iface: "wlan0"},
		conf:    &confManager{dir: dir},
		proc:    proc,
		newConn: func(_, _ string) (wpaSocket, error) { return wpa, nil },
	}
	m.conn = wpa
	return m
}

// ── Existing tests ─────────────────────────────────────────────────────────

func TestManager_Scan(t *testing.T) {
	wpa := &fakeWpa{responses: map[string]string{
		"SCAN":         "OK",
		"SCAN_RESULTS": "bssid / frequency / signal level / flags / ssid\naa:bb:cc:dd:ee:ff\t2437\t-60\t[WPA2-PSK-CCMP][ESS]\tTestNet\n",
	}}
	m := newTestManager(t, wpa)
	nets, err := m.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 || nets[0].SSID != "TestNet" {
		t.Fatalf("unexpected scan results: %+v", nets)
	}
}

func TestManager_Connect_Save(t *testing.T) {
	wpa := &fakeWpa{responses: map[string]string{"ADD_NETWORK": "0"}}
	m := newTestManager(t, wpa)
	if err := m.Connect("MyNet", "mypass12", true); err != nil {
		t.Fatal(err)
	}
	nets, err := m.conf.read()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range nets {
		if n.SSID == "MyNet" {
			found = true
		}
	}
	if !found {
		t.Error("expected MyNet in saved networks")
	}
}

func TestManager_RemoveSaved(t *testing.T) {
	dir := t.TempDir()
	conf := &confManager{dir: dir}
	if err := conf.write([]savedNetwork{{SSID: "OldNet", PSK: "pw12345678"}}); err != nil {
		t.Fatal(err)
	}
	wpa := &fakeWpa{}
	m := newTestManager(t, wpa)
	m.conf = conf
	if err := m.RemoveSaved("OldNet"); err != nil {
		t.Fatal(err)
	}
	nets, _ := conf.read()
	if len(nets) != 0 {
		t.Errorf("expected empty after remove, got %+v", nets)
	}
}

func TestManager_Status(t *testing.T) {
	wpa := &fakeWpa{responses: map[string]string{
		"STATUS": "wpa_state=COMPLETED\nssid=MyNet\n",
	}}
	m := newTestManager(t, wpa)
	st, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "COMPLETED" || st.SSID != "MyNet" {
		t.Errorf("unexpected status: %+v", st)
	}
}

// ── New tests ──────────────────────────────────────────────────────────────

// TestManager_Connect_ReusesSavedNetwork verifies that Connect with an empty
// PSK uses the existing wpa_supplicant network entry (SELECT_NETWORK + RECONNECT)
// rather than adding a new open-network entry (which would overwrite WPA2 creds).
func TestManager_Connect_ReusesSavedNetwork(t *testing.T) {
	wpa := &fakeWpa{responses: map[string]string{
		// LIST_NETWORKS: header + one entry for "HomeNet" with id 0
		"LIST_NETWORKS": "network id / ssid / bssid / flags\n0\tHomeNet\tany\t[CURRENT]\n",
	}}
	m := newTestManager(t, wpa)
	if err := m.Connect("HomeNet", "", false); err != nil {
		t.Fatal(err)
	}
	// Must use SELECT_NETWORK 0, not ADD_NETWORK
	wpa.mu.Lock()
	cmds := append([]string(nil), wpa.commands...)
	wpa.mu.Unlock()
	for _, c := range cmds {
		if c == "ADD_NETWORK" {
			t.Errorf("Connect should not call ADD_NETWORK when network already exists; commands: %v", cmds)
		}
	}
	found := false
	for _, c := range cmds {
		if c == "SELECT_NETWORK 0" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SELECT_NETWORK 0; commands: %v", cmds)
	}
}

// TestManager_Connect_SavePreservesExistingPSK ensures that calling Connect with
// psk="" and save=true does not overwrite a previously saved PSK with an empty string.
func TestManager_Connect_SavePreservesExistingPSK(t *testing.T) {
	wpa := &fakeWpa{responses: map[string]string{
		"LIST_NETWORKS": "", // not found → falls back to ADD_NETWORK path
		"ADD_NETWORK":   "0",
	}}
	m := newTestManager(t, wpa)
	// Seed a saved PSK — write stores it as hex PMK.
	if err := m.conf.write([]savedNetwork{{SSID: "HomeNet", PSK: "originalPSK"}}); err != nil {
		t.Fatal(err)
	}
	wantPMK := wpa2PMK("originalPSK", "HomeNet")
	// Connect with empty PSK and save — should preserve the hex PMK.
	if err := m.Connect("HomeNet", "", true); err != nil {
		t.Fatal(err)
	}
	nets, _ := m.conf.read()
	for _, n := range nets {
		if n.SSID == "HomeNet" && n.PSK != wantPMK {
			t.Errorf("PSK overwritten: got %q, want %q", n.PSK, wantPMK)
		}
	}
}

// TestManager_SetAPConfig_ValidatesPSK checks WPA2 passphrase length rules.
func TestManager_SetAPConfig_ValidatesPSK(t *testing.T) {
	m := newTestManager(t, &fakeWpa{})
	// Too short (< 8 chars)
	if err := m.SetAPConfig(APConfig{SSID: "test", PSK: "short"}); err == nil {
		t.Error("expected error for PSK < 8 chars")
	}
	// Too long (> 63 chars)
	if err := m.SetAPConfig(APConfig{SSID: "test", PSK: string(make([]byte, 64))}); err == nil {
		t.Error("expected error for PSK > 63 chars")
	}
	// Exactly 8 chars — valid
	if err := m.SetAPConfig(APConfig{SSID: "test", PSK: "12345678"}); err != nil {
		t.Errorf("unexpected error for 8-char PSK: %v", err)
	}
	// Empty PSK (open network) — valid
	if err := m.SetAPConfig(APConfig{SSID: "open"}); err != nil {
		t.Errorf("unexpected error for open network: %v", err)
	}
}

// TestManager_SetAPConfig_ValidatesChannel checks 2.4 GHz channel range.
func TestManager_SetAPConfig_ValidatesChannel(t *testing.T) {
	m := newTestManager(t, &fakeWpa{})
	if err := m.SetAPConfig(APConfig{SSID: "x", Channel: 0}); err != nil {
		t.Errorf("channel 0 (auto) should be valid: %v", err)
	}
	if err := m.SetAPConfig(APConfig{SSID: "x", Channel: 15}); err == nil {
		t.Error("expected error for channel 15 (out of range)")
	}
	if err := m.SetAPConfig(APConfig{SSID: "x", Channel: 11}); err != nil {
		t.Errorf("channel 11 should be valid: %v", err)
	}
}

// TestManager_STAChannel_ParsesFreq verifies frequency→channel conversion.
func TestManager_STAChannel_ParsesFreq(t *testing.T) {
	tests := []struct {
		status  string
		want    int
	}{
		{"freq=2462\nwpa_state=COMPLETED\n", 11},   // 2.4 GHz ch 11
		{"freq=2412\nwpa_state=COMPLETED\n", 1},    // ch 1
		{"freq=2437\nwpa_state=COMPLETED\n", 6},    // ch 6
		{"wpa_state=SCANNING\n", 0},                 // not connected
	}
	for _, tc := range tests {
		wpa := &fakeWpa{responses: map[string]string{"STATUS": tc.status}}
		m := newTestManager(t, wpa)
		got := m.staChannel()
		if got != tc.want {
			t.Errorf("staChannel(%q) = %d, want %d", tc.status, got, tc.want)
		}
	}
}

// TestManager_Send_ConcurrentSafe verifies that send() via connMu does not
// race with concurrent conn replacement (connMu guards both paths).
func TestManager_Send_ConcurrentSafe(t *testing.T) {
	wpa := &fakeWpa{responses: map[string]string{"STATUS": "wpa_state=SCANNING\n"}}
	m := newTestManager(t, wpa)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = m.send("STATUS")
		}()
	}
	// Concurrently replace conn (simulates reconnect).
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.connMu.Lock()
			old := m.conn
			m.conn = &fakeWpa{responses: map[string]string{"STATUS": "wpa_state=SCANNING\n"}}
			m.connMu.Unlock()
			if old != nil {
				_ = old.Close()
			}
		}()
	}
	wg.Wait()
}

// TestManager_DebugCmd_NotStarted verifies that calling DebugCmd on a manager
// whose Start has not been called returns ErrNotStarted.
func TestManager_DebugCmd_NotStarted(t *testing.T) {
	dir := t.TempDir()
	m := &Manager{
		cfg:  Config{ConfDir: dir, Iface: "wlan0"},
		conf: &confManager{dir: dir},
	}
	_, err := m.DebugCmd("STATUS")
	if !errors.Is(err, ErrNotStarted) {
		t.Errorf("expected ErrNotStarted, got %v", err)
	}
}

