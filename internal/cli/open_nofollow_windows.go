//go:build windows

package cli

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// openNoFollow opens the reparse-point itself, so the caller's regular-file
// check rejects a swapped symlink before any target is read.
func openNoFollow(path string) (*os.File, error) {
	value, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("encode selected file path: %w", err)
	}
	handle, err := windows.CreateFile(
		value,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		if closeErr := windows.CloseHandle(handle); closeErr != nil {
			return nil, fmt.Errorf("close selected file handle: %w", closeErr)
		}
		return nil, fmt.Errorf("wrap selected file handle")
	}
	return file, nil
}
