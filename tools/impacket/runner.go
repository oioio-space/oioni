// tools/impacket/runner.go
package impacket

import (
	"context"

	"github.com/oioio-space/oioni/tools/containers"
)

// Run launches any impacket script by name with raw args.
// name identifies the process in the registry (must be unique among running procs).
// tool is the impacket script name, e.g. "samrdump", "lookupsid".
func (i *Impacket) Run(ctx context.Context, name, tool string, args []string) (*containers.Process, error) {
	return i.mgr.Start(ctx, name, tool, args)
}
