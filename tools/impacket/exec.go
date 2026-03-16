// tools/impacket/exec.go
package impacket

import (
	"context"
	"fmt"

	"github.com/oioio-space/oioni/tools/containers"
)

// ExecMethod selects the remote execution backend.
type ExecMethod string

const (
	// ExecWMI uses wmiexec.py — WMI-based execution, semi-stealthy, no service install.
	ExecWMI ExecMethod = "wmi"
	// ExecSMB uses psexec.py — SMB service-based execution, noisier, creates a service.
	ExecSMB ExecMethod = "smb"
	// ExecSMBExec uses smbexec.py — SMB + cmd.exe, no binary upload required.
	ExecSMBExec ExecMethod = "smbexec"
)

// ExecConfig configures a remote execution invocation.
type ExecConfig struct {
	Target   string     // host IP or hostname
	Username string
	Password string
	Hash     string     // pass-the-hash: LMHASH:NTHASH
	Domain   string     // optional
	Command  string     // command to run; if empty the process streams an interactive shell
	Method   ExecMethod // default: ExecWMI
}

// Exec launches a remote command via wmiexec, psexec, or smbexec and returns
// the streaming process. Lines() yields the command output.
// If cfg.Command is empty, an interactive shell session is started.
// Use Stop() or Kill() to terminate it.
func (i *Impacket) Exec(ctx context.Context, name string, cfg ExecConfig) (*containers.Process, error) {
	if cfg.Target == "" {
		return nil, fmt.Errorf("exec: Target is required")
	}

	tool := execTool(cfg.Method)
	args := authArgs(cfg.Domain, cfg.Username, cfg.Password, cfg.Hash, cfg.Target)
	if cfg.Command != "" {
		args = append(args, cfg.Command)
	}

	return i.mgr.Start(ctx, name, tool, args)
}

func execTool(m ExecMethod) string {
	switch m {
	case ExecSMB:
		return "psexec.py"
	case ExecSMBExec:
		return "smbexec.py"
	default:
		return "wmiexec.py"
	}
}
