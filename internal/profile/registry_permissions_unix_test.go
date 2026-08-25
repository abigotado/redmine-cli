//go:build !windows

package profile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func assertRegistryPlatformPermissions(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat registry: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("registry mode = %o", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat registry dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("registry dir mode = %o", dirInfo.Mode().Perm())
	}
}

func TestRegistryRejectsBroadUnixPermissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	path := filepath.Join(dir, "profiles.json")
	if err := os.WriteFile(path, []byte("[]"), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	_, err := NewRegistry(path).List(context.Background())
	if !errors.Is(err, ErrInsecurePermissions) {
		t.Fatalf("List() error = %v, want %v", err, ErrInsecurePermissions)
	}
}
