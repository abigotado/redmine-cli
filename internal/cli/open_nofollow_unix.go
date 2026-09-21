//go:build !windows

package cli

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// openNoFollow opens one filesystem entry without resolving a final symlink.
// The caller verifies both type and identity before reading its contents.
func openNoFollow(path string) (*os.File, error) {
	descriptor, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		if closeErr := unix.Close(descriptor); closeErr != nil {
			return nil, fmt.Errorf("close selected file descriptor: %w", closeErr)
		}
		return nil, fmt.Errorf("wrap selected file descriptor")
	}
	return file, nil
}
