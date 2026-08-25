//go:build windows

package profile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func assertRegistryPlatformPermissions(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat registry: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("registry mode = %v, want regular file", info.Mode())
	}
	dirInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("lstat registry dir: %v", err)
	}
	if !dirInfo.IsDir() {
		t.Fatalf("registry dir mode = %v, want directory", dirInfo.Mode())
	}
}

func TestRegistryAcceptsWindowsSynthesizedPermissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	if err := os.WriteFile(path, []byte("[]"), 0o600); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	if _, err := NewRegistry(path).List(context.Background()); err != nil {
		t.Fatalf("List() error = %v", err)
	}
}
