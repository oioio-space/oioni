// tools/impacket/ntlmrelayx.go
package impacket

import (
	"context"
	"regexp"
	"sync"

	"github.com/oioio-space/oioni/tools/containers"
)

// NTLMRelayConfig configures ntlmrelayx.py.
type NTLMRelayConfig struct {
	Target      string // relay target, e.g. "smb://192.168.1.1"
	SMB2Support bool   // pass -smb2support
	OutputFile  string // pass -of <file>; optional
}

// NTLMRelayEvent holds a single parsed relay capture.
type NTLMRelayEvent struct {
	Username string
	Domain   string
	Hash     string // NTLMv2 hash blob
	Target   string // relay target
}

// NTLMRelayProcess wraps the underlying container process with a parsed event stream.
// Does NOT embed *containers.Process to avoid ambiguous Kill()/Wait() methods.
type NTLMRelayProcess struct {
	proc   *containers.Process
	mgr    ProcessStarter
	name   string
	events chan NTLMRelayEvent

	mu  sync.Mutex
	err error
}

// Process returns the underlying container process for Wait()/Running()/Lines() access.
func (p *NTLMRelayProcess) Process() *containers.Process { return p.proc }

// Events returns a buffered channel (capacity 16) of parsed NTLMRelayEvent values.
// Closed when the process exits.
func (p *NTLMRelayProcess) Events() <-chan NTLMRelayEvent { return p.events }

// Err returns the exit error, if any. Safe for concurrent use.
// Returns nil while the process is running. Call after Events() is closed.
func (p *NTLMRelayProcess) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// Stop terminates the ntlmrelayx daemon gracefully and waits for exit.
func (p *NTLMRelayProcess) Stop(ctx context.Context) error {
	return p.mgr.Stop(ctx, p.name)
}

// Kill sends SIGKILL immediately. Deregistration is asynchronous.
func (p *NTLMRelayProcess) Kill() error {
	return p.mgr.Kill(p.name)
}

// ntlmHashRe matches lines like:
//
//	[*] DOMAIN\user::TARGET:challenge:nthash
//
// emitted by ntlmrelayx when it captures a hash.
var ntlmHashRe = regexp.MustCompile(`^\[.*?\]\s+(\w+)\\(\w+)::[^:]+:[0-9a-fA-F]+:[0-9a-fA-F:]+$`)

// NTLMRelay starts ntlmrelayx as a background daemon. name must be unique.
func (i *Impacket) NTLMRelay(ctx context.Context, name string, cfg NTLMRelayConfig) (*NTLMRelayProcess, error) {
	args := ntlmRelayArgs(cfg)
	proc, err := i.mgr.Start(ctx, name, "ntlmrelayx.py", args)
	if err != nil {
		return nil, err
	}

	rp := &NTLMRelayProcess{
		proc:   proc,
		mgr:    i.mgr,
		name:   name,
		events: make(chan NTLMRelayEvent, 16),
	}

	// Parsing goroutine: read Lines(), emit events, store exit error.
	go func() {
		for line := range proc.Lines() {
			if e, ok := parseNTLMRelayLine(line, cfg.Target); ok {
				select {
				case rp.events <- e:
				default: // drop on full
				}
			}
		}
		exitErr := proc.Wait()
		rp.mu.Lock()
		rp.err = exitErr
		close(rp.events)
		rp.mu.Unlock()
	}()

	return rp, nil
}

func ntlmRelayArgs(cfg NTLMRelayConfig) []string {
	args := []string{"-t", cfg.Target}
	if cfg.SMB2Support {
		args = append(args, "-smb2support")
	}
	if cfg.OutputFile != "" {
		args = append(args, "-of", cfg.OutputFile)
	}
	return args
}

func parseNTLMRelayLine(line, target string) (NTLMRelayEvent, bool) {
	m := ntlmHashRe.FindStringSubmatch(line)
	if m == nil {
		return NTLMRelayEvent{}, false
	}
	return NTLMRelayEvent{
		Domain:   m[1],
		Username: m[2],
		Hash:     line, // full line preserved as hash blob
		Target:   target,
	}, true
}
