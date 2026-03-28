// cmd/oioni/wifi_api.go — simple HTTP API for WiFi testing on port 8080
//
//   GET  /wifi/status         → wpa_supplicant status JSON
//   GET  /wifi/scan           → scan + return networks JSON
//   POST /wifi/connect        → body: ssid=X&psk=Y  (save=1 to persist)
//   GET  /wifi/disconnect     → disconnect
//   GET  /wifi/networks       → saved networks list
//   GET  /wifi/ap/status      → AP status JSON
//   POST /wifi/ap/config      → body: ssid=X&psk=Y&channel=Z&ip=W
//   GET  /wifi/ap/enable      → enable AP (STA+AP concurrent)
//   GET  /wifi/ap/disable     → disable AP
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	wifi "github.com/oioio-space/oioni/system/wifi"
)

func startWiFiAPI(ctx context.Context, mgr *wifi.Manager) {
	mux := http.NewServeMux()

	mux.HandleFunc("/wifi/status", func(w http.ResponseWriter, r *http.Request) {
		st, err := mgr.Status()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st)
	})

	mux.HandleFunc("/wifi/scan", func(w http.ResponseWriter, r *http.Request) {
		nets, err := mgr.Scan()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nets)
	})

	mux.HandleFunc("/wifi/connect", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		ssid := r.FormValue("ssid")
		psk := r.FormValue("psk")
		save := r.FormValue("save") == "1"
		if ssid == "" {
			http.Error(w, "ssid required", 400)
			return
		}
		if err := mgr.Connect(ssid, psk, save); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprintf(w, "connecting to %q\n", ssid)
	})

	mux.HandleFunc("/wifi/disconnect", func(w http.ResponseWriter, r *http.Request) {
		if err := mgr.Disconnect(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprintln(w, "disconnected")
	})

	mux.HandleFunc("/wifi/networks", func(w http.ResponseWriter, r *http.Request) {
		nets, err := mgr.SavedNetworks()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nets)
	})

	mux.HandleFunc("/wifi/ap/status", func(w http.ResponseWriter, r *http.Request) {
		st := mgr.APStatus()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st)
	})

	mux.HandleFunc("/wifi/ap/config", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		cfg, _ := mgr.GetAPConfig()
		if v := r.FormValue("ssid"); v != "" {
			cfg.SSID = v
		}
		if v := r.FormValue("psk"); v != "" {
			cfg.PSK = v
		}
		if v := r.FormValue("channel"); v != "" {
			fmt.Sscanf(v, "%d", &cfg.Channel)
		}
		if v := r.FormValue("ip"); v != "" {
			cfg.IP = v
		}
		if cfg.Channel == 0 {
			cfg.Channel = 6
		}
		if cfg.IP == "" {
			cfg.IP = "192.168.4.1/24"
		}
		if err := mgr.SetAPConfig(cfg); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprintf(w, "AP config saved: ssid=%q channel=%d ip=%s\n", cfg.SSID, cfg.Channel, cfg.IP)
	})

	mux.HandleFunc("/wifi/ap/enable", func(w http.ResponseWriter, r *http.Request) {
		if err := mgr.SetMode(ctx, wifi.Mode{STA: true, AP: true}); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprintln(w, "AP mode enabled (STA+AP)")
	})

	mux.HandleFunc("/wifi/debug/wpa-cmd", func(w http.ResponseWriter, r *http.Request) {
		cmd := r.FormValue("cmd")
		if cmd == "" {
			http.Error(w, "cmd required", 400)
			return
		}
		out, err := mgr.DebugCmd(cmd)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, out)
	})

	mux.HandleFunc("/wifi/debug/wpa-log", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("/perm/wifi/wpa.log")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		// Return last 64KB to avoid overwhelming the response.
		if len(data) > 65536 {
			data = data[len(data)-65536:]
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write(data)
	})

	mux.HandleFunc("/wifi/debug/wpa-conf", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("/perm/wifi/wpa_supplicant.conf")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		// Redact PSK values.
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "psk=") {
				lines[i] = "\tpsk=<REDACTED>"
			}
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, strings.Join(lines, "\n"))
	})

	mux.HandleFunc("/wifi/ap/disable", func(w http.ResponseWriter, r *http.Request) {
		if err := mgr.SetMode(ctx, wifi.Mode{STA: true, AP: false}); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprintln(w, "AP mode disabled")
	})

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Printf("wifi-api: listen :8080: %v", err)
		return
	}
	log.Printf("wifi-api: listening on :8080")

	srv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	go func() {
		if err := srv.Serve(ln); err != nil && ctx.Err() == nil {
			log.Printf("wifi-api: %v", err)
		}
	}()
}
