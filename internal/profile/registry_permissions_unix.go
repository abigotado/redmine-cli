//go:build !windows

package profile

import (
	"errors"
	"fmt"
	"os"
)

func validateRegistryFileInfo(info os.FileInfo, path string) error {
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return fmt.Errorf("%w: %s must be a regular 0600 file", ErrInsecurePermissions, path)
	}
	return nil
}

func validateRegistryDirectoryInfo(info os.FileInfo, path string) error {
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("%w: %s must be a 0700 directory", ErrInsecurePermissions, path)
	}
	return nil
}

func secureRegistryDirectory(path string) error {
	return os.Chmod(path, 0o700)
}

func secureRegistryFile(file *os.File) error {
	return file.Chmod(0o600)
}

func syncRegistryDirectory(dir string, openDir func(string) (*os.File, error)) error {
	if openDir == nil {
		openDir = os.Open
	}
	directory, err := openDir(dir)
	if err != nil {
		return &CommitError{Err: fmt.Errorf("open profile registry directory for sync: %w", err)}
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return &CommitError{Err: errors.Join(
			wrapIfError("sync profile registry directory", syncErr),
			wrapIfError("close profile registry directory", closeErr),
		)}
	}
	return nil
}
