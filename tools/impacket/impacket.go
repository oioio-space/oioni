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

// impacketConfig holds configuration for the impacket container.
type impacketConfig struct {
	localImagePath string // path to a .tar/.tar.gz to load instead of pulling
}

// ImpacketOption is a functional option for New().
type ImpacketOption func(*impacketConfig)

// WithLocalImage instructs New() to load the container image from a local
// tar/tar.gz file instead of pulling from a registry.
// On gokrazy, ship the image via ExtraFilePaths and point to its path here.
func WithLocalImage(path string) ImpacketOption {
	return func(c *impacketConfig) { c.localImagePath = path }
}

// defaultImage is the arm64 impacket image built from tools/impacket/Dockerfile.
const defaultImage = "oioni/impacket:arm64"

// defaultLocalImagePath is the gokrazy path where the image tar.gz is shipped
// via ExtraFilePaths in config.json.
const defaultLocalImagePath = "/usr/share/oioni/impacket-arm64.tar.gz"

// Impacket provides typed wrappers for impacket scripts running in a container.
type Impacket struct {
	mgr ProcessStarter
}

// New returns an Impacket backed by a real ProcManager.
// The container is not started until the first tool call.
func New(opts ...ImpacketOption) *Impacket {
	cfg := &impacketConfig{localImagePath: defaultLocalImagePath}
	for _, o := range opts {
		o(cfg)
	}
	mgr := containers.NewManager(containers.Config{
		Image:          defaultImage,
		Name:           "oioni-impacket",
		Network:        "host",
		Caps:           []string{"NET_RAW", "NET_ADMIN"},
		LocalImagePath: cfg.localImagePath,
	})
	return &Impacket{mgr: mgr}
}

// NewWithManager injects a custom ProcessStarter (for tests).
func NewWithManager(mgr ProcessStarter) *Impacket {
	return &Impacket{mgr: mgr}
}
