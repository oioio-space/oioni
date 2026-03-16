// cmd/oioni/impacket.go — impacket tool integration (for testing on device)
package main

import (
	"context"
	"flag"
	"log"

	"github.com/oioio-space/oioni/tools/impacket"
)

type impacketFlags struct {
	// tool selection
	secretsdump bool
	ntlmrelay   bool
	kerberoast  bool
	asreproast  bool
	lookupsid   bool
	samrdump    bool
	exec        bool

	// common credential flags
	target  string
	domain  string
	user    string
	pass    string
	hash    string // pass-the-hash: LMHASH:NTHASH

	// tool-specific
	relayTarget string // ntlmrelay
	command     string // exec
	execMethod  string // exec: wmi|smb|smbexec
}

func defineImpacketFlags() *impacketFlags {
	f := &impacketFlags{}
	// tool selection
	flag.BoolVar(&f.secretsdump, "impacket-secretsdump", false, "run secretsdump.py")
	flag.BoolVar(&f.ntlmrelay, "impacket-ntlmrelay", false, "run ntlmrelayx.py (daemon, stops on SIGTERM)")
	flag.BoolVar(&f.kerberoast, "impacket-kerberoast", false, "run GetUserSPNs.py -request (Kerberoasting)")
	flag.BoolVar(&f.asreproast, "impacket-asreproast", false, "run GetNPUsers.py -request (AS-REP Roasting)")
	flag.BoolVar(&f.lookupsid, "impacket-lookupsid", false, "run lookupsid.py (SID enumeration)")
	flag.BoolVar(&f.samrdump, "impacket-samrdump", false, "run samrdump.py (user enumeration)")
	flag.BoolVar(&f.exec, "impacket-exec", false, "run wmiexec/psexec/smbexec (remote command)")
	// credentials
	flag.StringVar(&f.target, "impacket-target", "", "target host IP or hostname")
	flag.StringVar(&f.domain, "impacket-domain", "", "domain name (e.g. corp.local)")
	flag.StringVar(&f.user, "impacket-user", "", "username")
	flag.StringVar(&f.pass, "impacket-pass", "", "password")
	flag.StringVar(&f.hash, "impacket-hash", "", "pass-the-hash LMHASH:NTHASH")
	// tool-specific
	flag.StringVar(&f.relayTarget, "impacket-relay-target", "", "relay target URL (ntlmrelay), e.g. smb://192.168.1.1")
	flag.StringVar(&f.command, "impacket-command", "", "command to run (exec), e.g. 'whoami'")
	flag.StringVar(&f.execMethod, "impacket-exec-method", "wmi", "exec method: wmi|smb|smbexec")
	return f
}

// runImpacket executes the requested impacket tool and blocks until done.
func runImpacket(ctx context.Context, f *impacketFlags) {
	imp := impacket.New()

	switch {
	case f.secretsdump:
		runSecretsDump(ctx, imp, f)
	case f.ntlmrelay:
		runNTLMRelay(ctx, imp, f)
	case f.kerberoast:
		runKerberoast(ctx, imp, f)
	case f.asreproast:
		runASREPRoast(ctx, imp, f)
	case f.lookupsid:
		runLookupSID(ctx, imp, f)
	case f.samrdump:
		runSAMRDump(ctx, imp, f)
	case f.exec:
		runExec(ctx, imp, f)
	}
}

func runSecretsDump(ctx context.Context, imp *impacket.Impacket, f *impacketFlags) {
	if f.target == "" {
		log.Printf("impacket: -impacket-target required for secretsdump")
		return
	}
	log.Printf("impacket: secretsdump → %s", f.target)
	creds, err := imp.SecretsDump(ctx, "secretsdump", impacket.SecretsDumpConfig{
		Target:   f.target,
		Domain:   f.domain,
		Username: f.user,
		Password: f.pass,
		Hash:     f.hash,
	})
	if err != nil {
		log.Printf("impacket: secretsdump: %v", err)
		return
	}
	log.Printf("impacket: secretsdump found %d credential(s)", len(creds))
	for i, c := range creds {
		log.Printf("  [%d] %s\\%s (%s) hash=%s", i, c.Domain, c.Username, c.Type, c.Hash)
	}
}

func runNTLMRelay(ctx context.Context, imp *impacket.Impacket, f *impacketFlags) {
	if f.relayTarget == "" {
		log.Printf("impacket: -impacket-relay-target required for ntlmrelay")
		return
	}
	log.Printf("impacket: ntlmrelayx → %s (SIGTERM to stop)", f.relayTarget)
	relay, err := imp.NTLMRelay(ctx, "ntlmrelay", impacket.NTLMRelayConfig{
		Target:      f.relayTarget,
		SMB2Support: true,
	})
	if err != nil {
		log.Printf("impacket: ntlmrelay: %v", err)
		return
	}
	go func() { <-ctx.Done(); _ = relay.Kill() }()
	for e := range relay.Events() {
		log.Printf("impacket: captured %s\\%s hash=%s", e.Domain, e.Username, e.Hash)
	}
	if err := relay.Err(); err != nil {
		log.Printf("impacket: ntlmrelay exited: %v", err)
	}
}

func runKerberoast(ctx context.Context, imp *impacket.Impacket, f *impacketFlags) {
	log.Printf("impacket: kerberoast → %s (%s)", f.target, f.domain)
	hashes, err := imp.Kerberoast(ctx, "kerberoast", impacket.KerberoastConfig{
		Target:   f.target,
		Domain:   f.domain,
		Username: f.user,
		Password: f.pass,
		Hash:     f.hash,
	})
	if err != nil {
		log.Printf("impacket: kerberoast: %v", err)
		return
	}
	log.Printf("impacket: kerberoast found %d hash(es)", len(hashes))
	for i, h := range hashes {
		log.Printf("  [%d] %s\\%s SPN=%s", i, h.Domain, h.Username, h.SPN)
		log.Printf("       %s", h.Hash)
	}
}

func runASREPRoast(ctx context.Context, imp *impacket.Impacket, f *impacketFlags) {
	log.Printf("impacket: asreproast → %s (%s)", f.target, f.domain)
	hashes, err := imp.ASREPRoast(ctx, "asreproast", impacket.ASREPRoastConfig{
		Target:   f.target,
		Domain:   f.domain,
		Username: f.user,
		Password: f.pass,
		Hash:     f.hash,
	})
	if err != nil {
		log.Printf("impacket: asreproast: %v", err)
		return
	}
	log.Printf("impacket: asreproast found %d hash(es)", len(hashes))
	for i, h := range hashes {
		log.Printf("  [%d] %s\\%s", i, h.Domain, h.Username)
		log.Printf("       %s", h.Hash)
	}
}

func runLookupSID(ctx context.Context, imp *impacket.Impacket, f *impacketFlags) {
	log.Printf("impacket: lookupsid → %s", f.target)
	objs, err := imp.LookupSID(ctx, "lookupsid", impacket.SIDLookupConfig{
		Target:   f.target,
		Domain:   f.domain,
		Username: f.user,
		Password: f.pass,
		Hash:     f.hash,
	})
	if err != nil {
		log.Printf("impacket: lookupsid: %v", err)
		return
	}
	log.Printf("impacket: lookupsid found %d object(s)", len(objs))
	for _, o := range objs {
		log.Printf("  %d: %s\\%s (%s)", o.RID, o.Domain, o.Name, o.Type)
	}
}

func runSAMRDump(ctx context.Context, imp *impacket.Impacket, f *impacketFlags) {
	log.Printf("impacket: samrdump → %s", f.target)
	users, err := imp.SAMRDump(ctx, "samrdump", impacket.SAMRDumpConfig{
		Target:   f.target,
		Domain:   f.domain,
		Username: f.user,
		Password: f.pass,
		Hash:     f.hash,
	})
	if err != nil {
		log.Printf("impacket: samrdump: %v", err)
		return
	}
	log.Printf("impacket: samrdump found %d user(s)", len(users))
	for _, u := range users {
		log.Printf("  uid=%d %s", u.UID, u.Username)
	}
}

func runExec(ctx context.Context, imp *impacket.Impacket, f *impacketFlags) {
	log.Printf("impacket: exec (%s) → %s $ %s", f.execMethod, f.target, f.command)
	proc, err := imp.Exec(ctx, "exec", impacket.ExecConfig{
		Target:   f.target,
		Domain:   f.domain,
		Username: f.user,
		Password: f.pass,
		Hash:     f.hash,
		Command:  f.command,
		Method:   impacket.ExecMethod(f.execMethod),
	})
	if err != nil {
		log.Printf("impacket: exec: %v", err)
		return
	}
	go func() { <-ctx.Done(); _ = proc.Kill() }()
	for line := range proc.Lines() {
		log.Printf("  %s", line)
	}
	if err := proc.Wait(); err != nil {
		log.Printf("impacket: exec exited: %v", err)
	}
}
