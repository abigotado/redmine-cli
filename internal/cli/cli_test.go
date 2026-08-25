package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abigotado/redmine-cli/internal/auth"
	"github.com/abigotado/redmine-cli/internal/errx"
	"github.com/abigotado/redmine-cli/internal/profile"
	"github.com/abigotado/redmine-cli/internal/redmine"
	"github.com/abigotado/redmine-cli/internal/skills"
)

const cliSecretSentinel = "CLI_REDMINE_SECRET_SENTINEL"

type memoryRegistry struct{ values map[string]profile.Profile }

func (r *memoryRegistry) WithProfileLock(_ context.Context, _ string, fn func() error) error {
	return fn()
}
func (r *memoryRegistry) List(context.Context) ([]profile.Profile, error) {
	result := make([]profile.Profile, 0, len(r.values))
	for _, value := range r.values {
		result = append(result, value)
	}
	return result, nil
}
func (r *memoryRegistry) Get(_ context.Context, name string) (profile.Profile, error) {
	value, ok := r.values[name]
	if !ok {
		return profile.Profile{}, profile.ErrNotFound
	}
	return value, nil
}
func (r *memoryRegistry) Add(_ context.Context, value profile.Profile) error {
	if _, ok := r.values[value.Name]; ok {
		return profile.ErrAlreadyExists
	}
	r.values[value.Name] = value
	return nil
}
func (r *memoryRegistry) Put(_ context.Context, value profile.Profile) error {
	r.values[value.Name] = value
	return nil
}
func (r *memoryRegistry) Remove(_ context.Context, name string) error {
	if _, ok := r.values[name]; !ok {
		return profile.ErrNotFound
	}
	delete(r.values, name)
	return nil
}

type memoryStore struct {
	values map[string]auth.Credential
	loads  int
	saves  int
}

func (s *memoryStore) Load(_ context.Context, name string) (auth.Credential, error) {
	s.loads++
	value, ok := s.values[name]
	if !ok {
		return auth.Credential{}, auth.ErrNotFound
	}
	return value, nil
}
func (s *memoryStore) Save(_ context.Context, name string, value auth.Credential) error {
	s.saves++
	s.values[name] = value
	return nil
}
func (s *memoryStore) Delete(_ context.Context, name string) error {
	delete(s.values, name)
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func clientFactory(status int, body string) func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) {
	return func(selected profile.Profile, credential auth.Credential, logger *slog.Logger) (redmineReader, error) {
		transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Header.Get("X-Redmine-API-Key") != credential.Token {
				return nil, errors.New("missing credential header")
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
		})
		return redmine.New(
			redmine.Config{BaseURL: selected.BaseURL},
			redmine.Credential{Token: credential.Token},
			redmine.WithHTTPClient(&http.Client{Transport: transport}),
			redmine.WithLogger(logger),
			redmine.WithSleep(func(context.Context, time.Duration) error { return nil }),
		)
	}
}

func newTestApp() (*App, *memoryRegistry, *memoryStore, *bytes.Buffer, *bytes.Buffer) {
	registry := &memoryRegistry{values: map[string]profile.Profile{"work": {Name: "work", BaseURL: "https://redmine.test"}}}
	store := &memoryStore{values: map[string]auth.Credential{"work": {Token: cliSecretSentinel}}}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := &App{registry: registry, store: store, stdin: strings.NewReader(cliSecretSentinel + "\n"), stdout: stdout, stderr: stderr}
	return app, registry, store, stdout, stderr
}

func TestCurrentUserSecretCannotReachAnyCLIOutputMode(t *testing.T) {
	t.Parallel()
	body := `{"user":{"id":7,"login":"agent","firstname":"A","lastname":"User","api_key":"CLI_REDMINE_SECRET_SENTINEL"}}`
	for _, format := range []string{"json", "raw", "text"} {
		t.Run(format, func(t *testing.T) {
			app, _, _, stdout, stderr := newTestApp()
			app.newRedmine = clientFactory(http.StatusOK, body)
			code := app.Run(context.Background(), app.NewRootCommand(), []string{"me", "--profile", "work", "-o", format, "--verbose"})
			if code != errx.CodeOK {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			combined := stdout.String() + stderr.String()
			if strings.Contains(combined, cliSecretSentinel) || strings.Contains(combined, "api_key") {
				t.Fatalf("%s leaked: %s", format, combined)
			}
		})
	}
}

func TestLoginVerificationAndStatusCheckDoNotEmitAPIKey(t *testing.T) {
	t.Parallel()
	body := `{"user":{"id":7,"login":"agent","api_key":"CLI_REDMINE_SECRET_SENTINEL"}}`
	tests := []struct {
		name string
		args []string
	}{
		{name: "login", args: []string{"auth", "login", "--profile", "new", "--url", "https://redmine.test", "--token-stdin", "-o", "json"}},
		{name: "status check", args: []string{"auth", "status", "--profile", "work", "--check", "-o", "json"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, _, _, stdout, stderr := newTestApp()
			app.newRedmine = clientFactory(http.StatusOK, body)
			code := app.Run(context.Background(), app.NewRootCommand(), tc.args)
			if code != errx.CodeOK {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			combined := stdout.String() + stderr.String()
			if strings.Contains(combined, cliSecretSentinel) || strings.Contains(combined, "api_key") {
				t.Fatalf("leaked: %s", combined)
			}
		})
	}
}

func TestAuthFailureBodyAndVerboseLogsDoNotEmitSecret(t *testing.T) {
	t.Parallel()
	app, _, _, stdout, stderr := newTestApp()
	app.newRedmine = clientFactory(http.StatusUnauthorized, cliSecretSentinel)
	code := app.Run(context.Background(), app.NewRootCommand(), []string{"me", "--profile", "work", "--verbose"})
	if code != errx.CodeAuth {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), cliSecretSentinel) {
		t.Fatalf("leaked: %s%s", stdout.String(), stderr.String())
	}
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("stdin must not be read") }

type panicStore struct{}

func (panicStore) Load(context.Context, string) (auth.Credential, error) {
	panic("Keychain must not be read")
}
func (panicStore) Save(context.Context, string, auth.Credential) error {
	panic("Keychain must not be written")
}
func (panicStore) Delete(context.Context, string) error { panic("Keychain must not be changed") }

func TestLoginDryRunDoesNotReadTokenContactNetworkOrTouchKeychain(t *testing.T) {
	t.Parallel()
	registry := &memoryRegistry{values: map[string]profile.Profile{}}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := &App{registry: registry, store: panicStore{}, stdin: panicReader{}, stdout: stdout, stderr: stderr,
		newRedmine: func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) {
			panic("network must not be contacted")
		}}
	code := app.Run(context.Background(), app.NewRootCommand(), []string{"auth", "login", "--profile", "preview", "--url", "https://redmine.test/", "--token-stdin", "--dry-run"})
	if code != errx.CodeOK {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, ok := registry.values["preview"]; ok {
		t.Fatal("dry-run changed registry")
	}
}

func TestPanicMapsToInternalExitOne(t *testing.T) {
	t.Parallel()
	app, _, _, stdout, _ := newTestApp()
	app.newRedmine = func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) { panic("boom") }
	code := app.Run(context.Background(), app.NewRootCommand(), []string{"me", "--profile", "work"})
	if code != errx.CodeInternal {
		t.Fatalf("code=%d", code)
	}
	if strings.Contains(stdout.String(), "boom") {
		t.Fatalf("panic leaked: %s", stdout.String())
	}
}

func TestInvalidFieldsFailBeforeCredentialOrNetworkOperations(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "login", args: []string{"auth", "login", "--profile", "new", "--url", "https://redmine.test", "--token-stdin", "--fields", "typo"}},
		{name: "logout", args: []string{"auth", "logout", "--profile", "work", "--yes", "--fields", "typo"}},
		{name: "read", args: []string{"me", "--profile", "work", "--fields", "typo"}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			registry := &memoryRegistry{values: map[string]profile.Profile{"work": {Name: "work", BaseURL: "https://redmine.test"}}}
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			app := &App{
				registry: registry, store: panicStore{}, stdin: panicReader{}, stdout: stdout, stderr: stderr,
				newRedmine: func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) {
					panic("network must not be contacted")
				},
			}
			code := app.Run(context.Background(), app.NewRootCommand(), testCase.args)
			if code != errx.CodeUsage {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if _, exists := registry.values["work"]; !exists {
				t.Fatal("invalid fields changed profile metadata")
			}
		})
	}
}

func TestInvalidFieldsFailBeforeSkillUninstall(t *testing.T) {
	dest := t.TempDir()
	if _, err := skills.Install(context.Background(), skills.Options{
		Provider: skills.ProviderClaude, Scope: skills.ScopeUser, Dest: dest,
	}); err != nil {
		t.Fatalf("seed skill: %v", err)
	}
	target := filepath.Join(dest, skills.SkillName, "SKILL.md")
	app, _, _, stdout, stderr := newTestApp()
	code := app.Run(context.Background(), app.NewRootCommand(), []string{
		"skills", "uninstall", "--provider", "claude", "--scope", "user", "--dest", dest, "--yes", "--fields", "typo",
	})
	if code != errx.CodeUsage {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("invalid fields changed installed skill: %v", err)
	}
}
