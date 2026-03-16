// tools/impacket/impacket.go
package impacket

import (
	"context"

	"github.com/oioio-space/oioni/tools/containers"
)

// ProcessStarter is the full lifecycle interface impacket needs.
// *containers.ProcManager satisfies this interface.
// Test fakes implement all three methods; Stop and Kill may be no-ops when
// the test does not exercise lifecycle management.
type ProcessStarter interface {
	Start(ctx context.Context, name, executable string, args []string) (*containers.Process, error)
	Stop(ctx context.Context, name string) error
	Kill(name string) error
}

// impacketConfig holds future configuration (image override, etc.).
type impacketConfig struct{}

// ImpacketOption is a functional option for New().
type ImpacketOption func(*impacketConfig)

// defaultImage is the arm64 impacket image built from tools/impacket/Dockerfile.
const defaultImage = "oioni/impacket:arm64"

// Impacket provides typed wrappers for impacket scripts running in a container.
type Impacket struct {
	mgr ProcessStarter
}

// New returns an Impacket backed by a real ProcManager.
// The container is not started until the first tool call.
func New(opts ...ImpacketOption) *Impacket {
	cfg := &impacketConfig{}
	for _, o := range opts {
		o(cfg)
	}
	mgr := containers.NewManager(containers.Config{
		Image:   defaultImage,
		Name:    "oioni-impacket",
		Network: "host",
		Caps:    []string{"NET_RAW", "NET_ADMIN"},
	})
	return &Impacket{mgr: mgr}
}

// NewWithManager injects a custom ProcessStarter (for tests).
func NewWithManager(mgr ProcessStarter) *Impacket {
	return &Impacket{mgr: mgr}
}
