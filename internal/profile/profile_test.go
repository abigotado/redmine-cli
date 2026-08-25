package profile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "host", input: "https://redmine.example", want: "https://redmine.example"},
		{name: "port and base path", input: "https://redmine.example:8443/redmine/", want: "https://redmine.example:8443/redmine"},
		{name: "http", input: "http://redmine.example", wantErr: true},
		{name: "userinfo", input: "https://user@redmine.example", wantErr: true},
		{name: "query", input: "https://redmine.example?token=x", wantErr: true},
		{name: "localhost", input: "https://localhost", wantErr: true},
		{name: "ip", input: "https://127.0.0.1", wantErr: true},
		{name: "dot segment", input: "https://redmine.example/a/../b", wantErr: true},
		{name: "encoded path", input: "https://redmine.example/red%6dine", wantErr: true},
		{name: "uppercase host", input: "https://Redmine.example", wantErr: true},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := NormalizeBaseURL(testCase.input)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("NormalizeBaseURL(%q) = %q, nil", testCase.input, got)
				}
				return
			}
			if err != nil || got != testCase.want {
				t.Fatalf("NormalizeBaseURL(%q) = %q, %v; want %q", testCase.input, got, err, testCase.want)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"work", "client-a", "company.profile_2"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) error = %v", name, err)
		}
	}
	for _, name := range []string{"", "-work", "with space", "../escape", "кириллица"} {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) error = nil", name)
		}
	}
}

func TestRegistryRoundTripAndPermissions(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config", "profiles.json")
	registry := NewRegistry(path)
	ctx := context.Background()
	first := Profile{Name: "work", BaseURL: "https://redmine.example"}
	second := Profile{Name: "client", BaseURL: "https://client.example/redmine"}
	if err := registry.Add(ctx, first); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	if err := registry.Add(ctx, second); err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	assertRegistryPlatformPermissions(t, path)
	profiles, err := registry.List(ctx)
	if err != nil || len(profiles) != 2 || profiles[0].Name != "client" || profiles[1].Name != "work" {
		t.Fatalf("List() = %#v, %v", profiles, err)
	}
	if err := registry.Remove(ctx, "work"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := registry.Get(ctx, "work"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(removed) error = %v", err)
	}
}

func TestRegistryFailsClosedOnCorruptOrInsecureFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		mode    os.FileMode
		want    error
	}{
		{name: "unknown field", content: `[{"name":"work","base_url":"https://redmine.example","token":"no"}]`, mode: 0o600, want: ErrCorruptRegistry},
		{name: "duplicate", content: `[{"name":"work","base_url":"https://redmine.example"},{"name":"work","base_url":"https://redmine.example"}]`, mode: 0o600, want: ErrCorruptRegistry},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0o700); err != nil {
				t.Fatalf("chmod dir: %v", err)
			}
			path := filepath.Join(dir, "profiles.json")
			if err := os.WriteFile(path, []byte(testCase.content), testCase.mode); err != nil {
				t.Fatalf("write registry: %v", err)
			}
			_, err := NewRegistry(path).List(context.Background())
			if !errors.Is(err, testCase.want) {
				t.Fatalf("List() error = %v, want %v", err, testCase.want)
			}
		})
	}
}
