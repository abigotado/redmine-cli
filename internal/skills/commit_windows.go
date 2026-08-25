//go:build windows

package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func commitPreparedFile(base, target, temporary, expectedHash string, expectedExists bool) error {
	if err := assertNoSymlink(base, target); err != nil {
		return err
	}
	if !expectedExists {
		if err := os.Link(temporary, target); err != nil {
			if errors.Is(err, os.ErrExist) {
				return changedFile(target, "the destination appeared after installer classification")
			}
			return translateFS("commit", target, err)
		}
		return nil
	}
	quarantineName, err := newQuarantineName(filepath.Base(target))
	if err != nil {
		return err
	}
	quarantinePath := filepath.Join(filepath.Dir(target), quarantineName)
	if err := os.Rename(target, quarantinePath); err != nil {
		return changedFile(target, "the destination changed after installer classification")
	}
	data, exists, err := readRegularBounded(base, quarantinePath, maxPayloadBytes)
	if err != nil || !exists || sum(data) != expectedHash {
		return changedFile(quarantinePath, "the destination changed after installer classification")
	}
	if err := os.Link(temporary, target); err != nil {
		return changedFile(quarantinePath, fmt.Sprintf("commit failed: %v", err))
	}
	if err := os.Remove(quarantinePath); err != nil {
		return changedFile(quarantinePath, "replacement committed but the previous file could not be removed")
	}
	return nil
}
