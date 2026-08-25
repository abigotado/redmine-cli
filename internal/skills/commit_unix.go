//go:build !windows

package skills

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/abigotado/redmine-cli/internal/errx"
	"golang.org/x/sys/unix"
)

func commitPreparedFile(base, target, temporary, expectedHash string, expectedExists bool) error {
	if err := assertNoSymlink(base, target); err != nil {
		return err
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errx.Internal("resolve %s under %s", target, base)
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return translateFS("open", base, err)
	}
	defer root.Close()
	parent, err := root.Open(filepath.Dir(rel))
	if err != nil {
		return translateFS("open", filepath.Dir(target), err)
	}
	defer parent.Close()
	targetName := filepath.Base(rel)
	temporaryName := filepath.Base(temporary)
	if !expectedExists {
		if err := unix.Linkat(int(parent.Fd()), temporaryName, int(parent.Fd()), targetName, 0); err != nil {
			if errors.Is(err, unix.EEXIST) {
				return changedFile(target, "the destination appeared after installer classification")
			}
			return translateFS("commit", target, err)
		}
		return nil
	}

	quarantineName, err := newQuarantineName(targetName)
	if err != nil {
		return errx.Internal("create quarantine name: %v", err)
	}
	if err := unix.Renameat(int(parent.Fd()), targetName, int(parent.Fd()), quarantineName); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return changedFile(target, "the destination disappeared after installer classification")
		}
		return translateFS("quarantine", target, err)
	}
	quarantinePath := filepath.Join(filepath.Dir(target), quarantineName)
	fileDescriptor, err := unix.Openat(int(parent.Fd()), quarantineName, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return changedFile(quarantinePath, "the previous destination could not be opened safely")
	}
	file := os.NewFile(uintptr(fileDescriptor), quarantinePath)
	data, readErr := readBoundedRegular(file)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || sum(data) != expectedHash {
		return changedFile(quarantinePath, "the destination changed after installer classification")
	}
	if err := unix.Linkat(int(parent.Fd()), temporaryName, int(parent.Fd()), targetName, 0); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return changedFile(quarantinePath, "a concurrent destination was preserved and the previous file was quarantined")
		}
		return changedFile(quarantinePath, fmt.Sprintf("commit failed: %v", err))
	}
	if err := unix.Unlinkat(int(parent.Fd()), quarantineName, 0); err != nil {
		return &errx.Error{
			Code: errx.CodeConflict, Reason: "DEST_REPLACED_CLEANUP_REQUIRED",
			Message: fmt.Sprintf("replacement committed but the previous file remains at %s", quarantinePath),
			Hint:    "inspect and remove the preserved previous file; do not retry the install unchanged",
		}
	}
	return nil
}

func readBoundedRegular(file *os.File) ([]byte, error) {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxPayloadBytes {
		return nil, errors.New("entry is not a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPayloadBytes+1))
	if err != nil || len(data) > maxPayloadBytes {
		return nil, errors.New("entry changed while it was read")
	}
	return data, nil
}
