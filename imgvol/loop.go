package imgvol

// loop.go — loopback device management.
// Implemented in Task 5. Stubs present to allow package compilation during Task 4.

func detectFSType(path string) (FSType, error) {
	panic("imgvol: loop.go not yet implemented")
}

func attach(path, mountpoint, fstype string) (string, error) {
	panic("imgvol: loop.go not yet implemented")
}

func detach(mountpoint, loopDev string) error {
	panic("imgvol: loop.go not yet implemented")
}
