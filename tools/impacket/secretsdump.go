// tools/impacket/secretsdump.go
package impacket

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"time"
)

// SecretsDumpConfig configures a secretsdump.py invocation.
type SecretsDumpConfig struct {
	Target   string // host IP or hostname
	Username string
	Password string // one of Password or Hash required
	Hash     string // pass-the-hash, format "LMHASH:NTHASH"
	Domain   string // optional; defaults to target hostname
}

// Credential is a single credential extracted by secretsdump.
type Credential struct {
	Username string
	Domain   string
	Hash     string
	Type     string // "NTLM", "Kerberos", "Plaintext"
}

// samHashRe matches SAM hash dump lines:
//
//	username:RID:lmhash:nthash:::
var samHashRe = regexp.MustCompile(`^([^:]+):(\d+):([a-fA-F0-9]{32}):([a-fA-F0-9]{32}):::`)

type dumpResult struct {
	creds []Credential
	err   error
}

// SecretsDump runs secretsdump.py and returns parsed credentials.
// name must be unique among currently running procs.
// Respects ctx cancellation; always deregisters the process name before returning.
func (i *Impacket) SecretsDump(ctx context.Context, name string, cfg SecretsDumpConfig) ([]Credential, error) {
	proc, err := i.mgr.Start(ctx, name, "secretsdump.py", secretsDumpArgs(cfg))
	if err != nil {
		return nil, err
	}

	// Capacity 1 is load-bearing: prevents goroutine leak on cancellation path.
	done := make(chan dumpResult, 1)
	go func() {
		var creds []Credential
		for line := range proc.Lines() {
			if c, ok := parseSecretsDumpLine(line); ok {
				creds = append(creds, c)
			}
		}
		done <- dumpResult{creds, nil}
	}()

	select {
	case r := <-done:
		return r.creds, errors.Join(r.err, proc.Wait())

	case <-ctx.Done():
		_ = i.mgr.Kill(name)
		// Wait up to 5 s for goroutine to finish.
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			return nil, errors.Join(ctx.Err(),
				errors.New("secretsdump: process did not exit within 5s after kill"))
		}
		return nil, ctx.Err()
	}
}

func secretsDumpArgs(cfg SecretsDumpConfig) []string {
	domain := cfg.Domain
	if domain == "" {
		domain = cfg.Target
	}
	target := cfg.Username + "@" + cfg.Target
	if domain != cfg.Target {
		target = domain + "/" + cfg.Username + "@" + cfg.Target
	}
	args := []string{target}
	if cfg.Hash != "" {
		args = append(args, "-hashes", cfg.Hash)
	} else {
		args = append(args, "-p", cfg.Password)
	}
	return args
}

func parseSecretsDumpLine(line string) (Credential, bool) {
	m := samHashRe.FindStringSubmatch(line)
	if m == nil {
		return Credential{}, false
	}
	if _, err := strconv.Atoi(m[2]); err != nil {
		return Credential{}, false
	}
	return Credential{
		Username: m[1],
		Hash:     m[3] + ":" + m[4],
		Type:     "NTLM",
	}, true
}
