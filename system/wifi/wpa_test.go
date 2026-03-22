package wifi

import (
	"testing"
)

func TestParseScanResults(t *testing.T) {
	raw := "bssid / frequency / signal level / flags / ssid\n" +
		"aa:bb:cc:dd:ee:ff\t2437\t-55\t[WPA2-PSK-CCMP][ESS]\tMyNet\n" +
		"11:22:33:44:55:66\t2412\t-72\t[ESS]\tOpenNet\n"
	nets := parseScanResults(raw)
	if len(nets) != 2 {
		t.Fatalf("want 2, got %d", len(nets))
	}
	if nets[0].SSID != "MyNet" {
		t.Errorf("want MyNet, got %q", nets[0].SSID)
	}
	if nets[0].Security != "WPA2" {
		t.Errorf("want WPA2, got %q", nets[0].Security)
	}
	if nets[0].Signal != -55 {
		t.Errorf("want -55, got %d", nets[0].Signal)
	}
	if nets[1].Security != "Open" {
		t.Errorf("want Open, got %q", nets[1].Security)
	}
}

func TestParseStatus(t *testing.T) {
	raw := "wpa_state=COMPLETED\nssid=MyNet\nip_address=192.168.1.10\n"
	st := parseWpaStatus(raw)
	if st.State != "COMPLETED" {
		t.Errorf("want COMPLETED, got %q", st.State)
	}
	if st.SSID != "MyNet" {
		t.Errorf("want MyNet, got %q", st.SSID)
	}
}
