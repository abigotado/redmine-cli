//go:build windows

package skills

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/abigotado/redmine-cli/internal/errx"
	"golang.org/x/sys/windows"
)

func removeIfHashMatches(base, target, expectedHash string) error {
	return removeIfHashMatchesWithHook(base, target, expectedHash, nil)
}

// removeIfHashMatchesWithHook atomically detaches the target before hashing.
// The hook exists only so Windows tests can deterministically simulate a
// concurrent directory-entry replacement after detachment.
func removeIfHashMatchesWithHook(base, target, expectedHash string, afterQuarantine func(string)) error {
	if expectedHash == "" {
		return changedFile(target, "the ownership manifest has no hash for this file")
	}
	if err := assertNoSymlink(base, target); err != nil {
		return err
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errx.Internal("resolve %s under %s", target, base)
	}
	quarantineName, err := newQuarantineName(filepath.Base(rel))
	if err != nil {
		return errx.Internal("create quarantine name: %v", err)
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return translateFS("open", base, err)
	}
	defer root.Close()
	quarantineRel := filepath.Join(filepath.Dir(rel), quarantineName)
	quarantinePath := filepath.Join(filepath.Dir(target), quarantineName)
	if err := root.Rename(rel, quarantineRel); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return translateFS("quarantine", target, err)
	}
	if afterQuarantine != nil {
		afterQuarantine(quarantinePath)
	}

	path, err := windows.UTF16PtrFromString(quarantinePath)
	if err != nil {
		return changedFile(quarantinePath, "the quarantined path could not be represented safely")
	}
	handle, err := windows.CreateFile(
		path,
		windows.GENERIC_READ|windows.DELETE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return changedFile(quarantinePath, "the quarantined file could not be opened safely")
	}
	file := os.NewFile(uintptr(handle), quarantinePath)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return changedFile(quarantinePath, "the quarantined handle could not be managed safely")
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxPayloadBytes {
		return changedFile(quarantinePath, "the detached entry is not a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPayloadBytes+1))
	if err != nil || len(data) > maxPayloadBytes {
		return changedFile(quarantinePath, "the detached entry could not be read within the safety limit")
	}
	if sum(data) != expectedHash {
		return changedFile(quarantinePath, "the file changed after installer classification")
	}
	deleteFile := byte(1)
	if err := windows.SetFileInformationByHandle(handle, windows.FileDispositionInfo, &deleteFile, 1); err != nil {
		return &errx.Error{
			Code:    errx.CodeConflict,
			Reason:  "DEST_CHANGED",
			Message: fmt.Sprintf("owned file remains quarantined at %s", quarantinePath),
			Hint:    "close processes using the quarantined file and retry cleanup",
		}
	}
	closeErr := file.Close()
	closed = true
	if closeErr != nil {
		return translateFS("close deleted quarantine", quarantinePath, closeErr)
	}
	return nil
}
