package storage

import (
	"context"
	"log"
	"path/filepath"
	"sync"

	"awesomeProject/storage/mount"
	"awesomeProject/storage/usbdetect"
	"github.com/spf13/afero"
)

// detector is satisfied by *usbdetect.Detector and by fakes in tests.
type detector interface {
	Start(ctx context.Context) (<-chan usbdetect.Event, error)
}

// mounter abstracts mount operations for testability.
type mounter interface {
	DetectFSType(device string) (string, error)
	Mount(device, mountpoint, fstype string) error
	Unmount(mountpoint string) error
}

// realMounter delegates to the mount package.
type realMounter struct{}

func (realMounter) DetectFSType(d string) (string, error) { return mount.DetectFSType(d) }
func (realMounter) Mount(d, mp, fs string) error          { return mount.Mount(d, mp, fs) }
func (realMounter) Unmount(mp string) error               { return mount.Unmount(mp) }

// Option configures a Manager.
type Option func(*Manager)

// WithPermPath sets the gokrazy persistent partition path. Default: "/perm".
func WithPermPath(path string) Option { return func(m *Manager) { m.permPath = path } }

// WithMountBase sets the directory under which USB volumes are mounted.
// Default: "/tmp/storage".
func WithMountBase(path string) Option { return func(m *Manager) { m.mountBase = path } }

// WithOnMount sets the callback invoked after a volume is mounted.
func WithOnMount(fn func(*Volume)) Option { return func(m *Manager) { m.OnMount = fn } }

// WithOnUnmount sets the callback invoked just before a volume is unmounted.
func WithOnUnmount(fn func(*Volume)) Option { return func(m *Manager) { m.OnUnmount = fn } }

// Manager orchestrates /perm and USB volumes.
type Manager struct {
	permPath  string
	mountBase string
	OnMount   func(*Volume)
	OnUnmount func(*Volume)

	mnt mounter
	src detector

	mu      sync.Mutex
	volumes map[string]*Volume // keyed by MountPath
}

// New returns a Manager using the real USB detector and mounter.
func New(opts ...Option) *Manager {
	m := newManager(usbdetect.New(), realMounter{}, "/perm", "/tmp/storage")
	for _, o := range opts {
		o(m)
	}
	return m
}

// newManager is the internal constructor used by New and tests.
func newManager(det detector, mnt mounter, permPath, mountBase string) *Manager {
	return &Manager{
		permPath:  permPath,
		mountBase: mountBase,
		mnt:       mnt,
		src:       det,
		volumes:   make(map[string]*Volume),
	}
}

// Start mounts /perm, performs an initial USB scan, then processes hotplug
// events until ctx is cancelled. Blocks until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) error {
	perm := permVolume(m.permPath)
	m.add(perm)

	events, err := m.src.Start(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			m.unmountAll()
			return nil
		case ev, ok := <-events:
			if !ok {
				m.unmountAll()
				return nil
			}
			switch ev.Action {
			case "add":
				m.handleAdd(ev.Device)
			case "remove":
				m.handleRemove(ev.Device)
			}
		}
	}
}

// Volumes returns a snapshot of currently mounted volumes.
func (m *Manager) Volumes() []*Volume {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Volume, 0, len(m.volumes))
	for _, v := range m.volumes {
		out = append(out, v)
	}
	return out
}

// Volume returns the volume with the given name, or false if not found.
func (m *Manager) Volume(name string) (*Volume, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.volumes {
		if v.Name == name {
			return v, true
		}
	}
	return nil, false
}

func (m *Manager) handleAdd(device string) {
	fstype, err := m.mnt.DetectFSType(device)
	if err != nil {
		log.Printf("storage: detect fstype %s: %v (skipping)", device, err)
		return
	}
	name := filepath.Base(device)
	mp := filepath.Join(m.mountBase, name)

	if err := m.mnt.Mount(device, mp, fstype); err != nil {
		log.Printf("storage: mount %s: %v", device, err)
		return
	}

	v := &Volume{
		Name:       name,
		Device:     device,
		MountPath:  mp,
		FSType:     fstype,
		Persistent: false,
		FS:         afero.NewBasePathFs(afero.NewOsFs(), mp),
	}
	m.add(v)
}

func (m *Manager) handleRemove(device string) {
	name := filepath.Base(device)
	mp := filepath.Join(m.mountBase, name)

	m.mu.Lock()
	v, ok := m.volumes[mp]
	if ok {
		delete(m.volumes, mp)
	}
	m.mu.Unlock()

	if !ok {
		return
	}
	if m.OnUnmount != nil {
		m.OnUnmount(v)
	}
	if err := m.mnt.Unmount(mp); err != nil {
		log.Printf("storage: unmount %s: %v", mp, err)
	}
}

func (m *Manager) add(v *Volume) {
	m.mu.Lock()
	m.volumes[v.MountPath] = v
	m.mu.Unlock()
	if m.OnMount != nil {
		m.OnMount(v)
	}
}

func (m *Manager) unmountAll() {
	m.mu.Lock()
	vols := make([]*Volume, 0, len(m.volumes))
	for _, v := range m.volumes {
		vols = append(vols, v)
	}
	m.mu.Unlock()

	for _, v := range vols {
		if v.Persistent {
			continue
		}
		if m.OnUnmount != nil {
			m.OnUnmount(v)
		}
		m.mnt.Unmount(v.MountPath)
	}
}
