//go:build darwin || linux

package paths

import "syscall"

// FreeBytes returns available bytes on the filesystem holding path, and true.
func FreeBytes(path string) (uint64, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, false
	}
	return st.Bavail * uint64(st.Bsize), true
}
