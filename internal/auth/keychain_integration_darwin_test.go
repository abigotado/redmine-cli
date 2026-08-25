//go:build darwin && cgo && keychainintegration

package auth

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDisposableKeychainIsReadableAcrossDistinctBinaries(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	temp := t.TempDir()
	first := filepath.Join(temp, "helper-a")
	second := filepath.Join(temp, "helper-b")
	helpers := []struct {
		identity string
		path     string
	}{{identity: "A", path: first}, {identity: "B", path: second}}
	for _, helper := range helpers {
		command := exec.Command("go", "build", "-tags", "keychainintegration", "-ldflags", "-X main.helperIdentity="+helper.identity, "-o", helper.path, "./internal/auth/keychainintegration/cmd/helper")
		command.Dir = root
		if raw, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build helper: %v\n%s", err, raw)
		}
	}
	firstRaw, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read helper A: %v", err)
	}
	secondRaw, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read helper B: %v", err)
	}
	if bytes.Equal(firstRaw, secondRaw) {
		t.Fatal("cross-binary helpers are not distinct")
	}
	keychainPath := filepath.Join(temp, "disposable.keychain-db")
	password := "non-secret-integration-password"
	run := func(binary, action string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, binary, action, keychainPath, password)
		if raw, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", action, err, raw)
		}
	}
	t.Cleanup(func() {
		if _, err := os.Stat(keychainPath); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = exec.CommandContext(ctx, second, "delete-keychain", keychainPath, password).Run()
		}
	})
	run(first, "create-save")
	run(second, "load-v1")
	run(second, "overwrite-v2")
	run(first, "load-v2")
	run(second, "delete-item")
	run(first, "expect-missing")
	run(second, "delete-keychain")
}
