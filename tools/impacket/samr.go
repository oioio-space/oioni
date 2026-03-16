// tools/impacket/samr.go
package impacket

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// SAMUser is a user entry returned by samrdump.py.
type SAMUser struct {
	Username string
	UID      int // Unix-style UID mapped from RID
}

// SAMRDumpConfig configures a samrdump.py invocation.
type SAMRDumpConfig struct {
	Target   string // host IP or hostname
	Username string
	Password string
	Hash     string // pass-the-hash: LMHASH:NTHASH
	Domain   string // optional
}

// samrUserRe matches samrdump.py output lines:
//
//	Found user: Administrator, uid = 500
var samrUserRe = regexp.MustCompile(`^Found user:\s+(.+),\s+uid\s*=\s*(\d+)`)

// SAMRDump runs samrdump.py and returns the list of enumerated users.
func (i *Impacket) SAMRDump(ctx context.Context, name string, cfg SAMRDumpConfig) ([]SAMUser, error) {
	if cfg.Target == "" {
		return nil, fmt.Errorf("samrdump: Target is required")
	}
	args := authArgs(cfg.Domain, cfg.Username, cfg.Password, cfg.Hash, cfg.Target)

	proc, err := i.mgr.Start(ctx, name, "samrdump.py", args)
	if err != nil {
		return nil, err
	}

	done := make(chan []SAMUser, 1)
	go func() {
		var users []SAMUser
		for line := range proc.Lines() {
			if u, ok := parseSAMRLine(line); ok {
				users = append(users, u)
			}
		}
		done <- users
	}()

	select {
	case users := <-done:
		return users, errors.Join(proc.Wait())
	case <-ctx.Done():
		_ = i.mgr.Kill(name)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			return nil, errors.Join(ctx.Err(), errors.New("samrdump: process did not exit within 5s after kill"))
		}
		return nil, ctx.Err()
	}
}

func parseSAMRLine(line string) (SAMUser, bool) {
	m := samrUserRe.FindStringSubmatch(line)
	if m == nil {
		return SAMUser{}, false
	}
	uid, err := strconv.Atoi(m[2])
	if err != nil {
		return SAMUser{}, false
	}
	return SAMUser{Username: m[1], UID: uid}, true
}
