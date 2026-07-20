//go:build !darwin && !linux

package paths

// FreeBytes is unsupported on this platform; callers skip the pre-flight check.
func FreeBytes(path string) (uint64, bool) { return 0, false }
