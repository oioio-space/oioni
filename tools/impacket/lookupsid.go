// tools/impacket/lookupsid.go
package impacket

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// DomainObject is a resolved SID entry returned by lookupsid.py.
type DomainObject struct {
	RID    int    // relative identifier, e.g. 500 for Administrator
	Domain string // NetBIOS domain name, e.g. "WORKGROUP"
	Name   string // account name, e.g. "Administrator" or "Domain Admins"
	Type   string // "SidTypeUser", "SidTypeGroup", "SidTypeAlias", etc.
}

// SIDLookupConfig configures a lookupsid.py invocation.
type SIDLookupConfig struct {
	Target   string // host IP or hostname
	Username string
	Password string
	Hash     string // pass-the-hash: LMHASH:NTHASH
	Domain   string // optional
	MaxRID   int    // brute-force up to this RID (default 4000)
}

// sidLineRe matches lookupsid.py output lines:
//
//	500: WORKGROUP\Administrator (SidTypeUser)
//	512: WORKGROUP\Domain Admins (SidTypeGroup)
var sidLineRe = regexp.MustCompile(`^(\d+):\s+([^\\\s]+)\\(.+?)\s+\((\w+)\)`)

// LookupSID brute-forces SIDs on the target via lookupsid.py and returns
// the resolved domain objects (users and groups).
func (i *Impacket) LookupSID(ctx context.Context, name string, cfg SIDLookupConfig) ([]DomainObject, error) {
	if cfg.Target == "" {
		return nil, fmt.Errorf("lookupsid: Target is required")
	}
	args := authArgs(cfg.Domain, cfg.Username, cfg.Password, cfg.Hash, cfg.Target)
	if cfg.MaxRID > 0 {
		args = append(args, strconv.Itoa(cfg.MaxRID))
	}

	proc, err := i.mgr.Start(ctx, name, "lookupsid.py", args)
	if err != nil {
		return nil, err
	}

	done := make(chan []DomainObject, 1)
	go func() {
		var objs []DomainObject
		for line := range proc.Lines() {
			if obj, ok := parseSIDLine(line); ok {
				objs = append(objs, obj)
			}
		}
		done <- objs
	}()

	select {
	case objs := <-done:
		return objs, errors.Join(proc.Wait())
	case <-ctx.Done():
		_ = i.mgr.Kill(name)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			return nil, errors.Join(ctx.Err(), errors.New("lookupsid: process did not exit within 5s after kill"))
		}
		return nil, ctx.Err()
	}
}

func parseSIDLine(line string) (DomainObject, bool) {
	m := sidLineRe.FindStringSubmatch(line)
	if m == nil {
		return DomainObject{}, false
	}
	rid, err := strconv.Atoi(m[1])
	if err != nil {
		return DomainObject{}, false
	}
	return DomainObject{
		RID:    rid,
		Domain: m[2],
		Name:   m[3],
		Type:   m[4],
	}, true
}
