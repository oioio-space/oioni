package wifi

import (
	"os"
	"testing"
)

// fakeProcess satisfies processRunner for tests.
type fakeProcess struct{ started bool }

func (f *fakeProcess) Start(_ string, _ []string) error { f.started = true; return nil }
func (f *fakeProcess) StartProcess(_ string, _ []string) (*os.Process, error) {
	return nil, nil
}

// fakeWpa satisfies wpaConn for tests.
type fakeWpa struct {
	responses map[string]string
	commands  []string
}

func (f *fakeWpa) SendCommand(cmd string) (string, error) {
	f.commands = append(f.commands, cmd)
	if r, ok := f.responses[cmd]; ok {
		return r, nil
	}
	return "OK", nil
}
func (f *fakeWpa) Close() error { return nil }

func newTestManager(t *testing.T, wpa *fakeWpa) *Manager {
	dir := t.TempDir()
	proc := &fakeProcess{}
	m := &Manager{
		cfg:     Config{ConfDir: dir, Iface: "wlan0"},
		conf:    &confManager{dir: dir},
		proc:    proc,
		newConn: func(_, _ string) (wpaConn, error) { return wpa, nil },
	}
	return m
}

func TestManager_Scan(t *testing.T) {
	wpa := &fakeWpa{responses: map[string]string{
		"SCAN":         "OK",
		"SCAN_RESULTS": "bssid / frequency / signal level / flags / ssid\naa:bb:cc:dd:ee:ff\t2437\t-60\t[WPA2-PSK-CCMP][ESS]\tTestNet\n",
	}}
	m := newTestManager(t, wpa)
	m.conn = wpa // bypass Start()
	nets, err := m.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 || nets[0].SSID != "TestNet" {
		t.Fatalf("unexpected scan results: %+v", nets)
	}
}

func TestManager_Connect_Save(t *testing.T) {
	wpa := &fakeWpa{responses: map[string]string{
		"ADD_NETWORK": "0",
	}}
	m := newTestManager(t, wpa)
	m.conn = wpa // bypass Start()
	if err := m.Connect("MyNet", "mypass", true); err != nil {
		t.Fatal(err)
	}
	// saved to conf
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
	if err := conf.write([]savedNetwork{{SSID: "OldNet", PSK: "pw"}}); err != nil {
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
	m.conn = wpa // bypass Start()
	st, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "COMPLETED" || st.SSID != "MyNet" {
		t.Errorf("unexpected status: %+v", st)
	}
}
