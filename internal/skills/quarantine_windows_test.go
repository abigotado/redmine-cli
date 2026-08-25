//go:build windows

package skills

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/abigotado/redmine-cli/internal/errx"
)

func TestRemoveIfHashMatchesWindowsPreservesConcurrentTarget(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "SKILL.md")
	owned := []byte("owned\n")
	if err := os.WriteFile(target, owned, 0o600); err != nil {
		t.Fatalf("write owned file: %v", err)
	}
	replacement := []byte("concurrent replacement\n")
	err := removeIfHashMatchesWithHook(base, target, sum(owned), func(string) {
		if writeErr := os.WriteFile(target, replacement, 0o600); writeErr != nil {
			t.Errorf("write concurrent target: %v", writeErr)
		}
	})
	if err != nil {
		t.Fatalf("removeIfHashMatchesWithHook() error = %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read concurrent target: %v", err)
	}
	if string(got) != string(replacement) {
		t.Fatalf("target = %q, want %q", got, replacement)
	}
}

func TestRemoveIfHashMatchesWindowsPreservesChangedQuarantine(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "SKILL.md")
	owned := []byte("owned\n")
	if err := os.WriteFile(target, owned, 0o600); err != nil {
		t.Fatalf("write owned file: %v", err)
	}
	var quarantinePath string
	err := removeIfHashMatchesWithHook(base, target, sum(owned), func(path string) {
		quarantinePath = path
		if removeErr := os.Remove(path); removeErr != nil {
			t.Errorf("replace quarantined file: %v", removeErr)
			return
		}
		if writeErr := os.WriteFile(path, []byte("changed\n"), 0o600); writeErr != nil {
			t.Errorf("write changed quarantine: %v", writeErr)
		}
	})
	var appError *errx.Error
	if !errors.As(err, &appError) || appError.Code != errx.CodeConflict {
		t.Fatalf("error = %v, want conflict", err)
	}
	if _, err := os.Stat(quarantinePath); err != nil {
		t.Fatalf("changed quarantine was not preserved: %v", err)
	}
}
