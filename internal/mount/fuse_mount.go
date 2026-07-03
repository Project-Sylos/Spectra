package mount

// FUSEMount represents an active FUSE mount.
type FUSEMount struct {
	mountPoint string
	wait       func()
	unmount    func() error
}

// MountPoint returns the OS path where the filesystem is mounted.
func (m *FUSEMount) MountPoint() string {
	return m.mountPoint
}

// Wait blocks until the mount is unmounted.
func (m *FUSEMount) Wait() {
	if m != nil && m.wait != nil {
		m.wait()
	}
}

// Unmount detaches the filesystem from the mount point.
func (m *FUSEMount) Unmount() error {
	if m != nil && m.unmount != nil {
		return m.unmount()
	}
	return nil
}
