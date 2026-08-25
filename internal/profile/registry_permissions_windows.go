//go:build windows

package profile

import (
	"fmt"
	"os"
)

// Windows exposes access control through ACLs and synthesizes os.FileMode
// permission bits. The registry contains profile names and base URLs only;
// credentials remain outside this file in the platform credential backend.
func validateRegistryFileInfo(info os.FileInfo, path string) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s must be a regular file", ErrInsecurePermissions, path)
	}
	return nil
}

func validateRegistryDirectoryInfo(info os.FileInfo, path string) error {
	if !info.IsDir() {
		return fmt.Errorf("%w: %s must be a directory", ErrInsecurePermissions, path)
	}
	return nil
}

func secureRegistryDirectory(string) error {
	return nil
}

func secureRegistryFile(*os.File) error {
	return nil
}

// FlushFileBuffers does not provide the POSIX directory-fsync durability
// primitive. The registry file itself is synced before the atomic rename.
func syncRegistryDirectory(string, func(string) (*os.File, error)) error {
	return nil
}
