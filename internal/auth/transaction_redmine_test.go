package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/abigotado/redmine-cli/internal/profile"
)

type fakeStore struct {
	values    map[string]Credential
	saveErr   error
	deleteErr error
}

func (store *fakeStore) Load(_ context.Context, name string) (Credential, error) {
	value, ok := store.values[name]
	if !ok {
		return Credential{}, ErrNotFound
	}
	return value, nil
}

func (store *fakeStore) Save(_ context.Context, name string, credential Credential) error {
	if store.saveErr != nil {
		return store.saveErr
	}
	store.values[name] = credential
	return nil
}

func (store *fakeStore) Delete(_ context.Context, name string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	delete(store.values, name)
	return nil
}

type fakeRegistry struct {
	values      map[string]profile.Profile
	mutationErr error
	committed   bool
}

func (registry *fakeRegistry) WithProfileLock(_ context.Context, _ string, fn func() error) error {
	return fn()
}

func (registry *fakeRegistry) Get(_ context.Context, name string) (profile.Profile, error) {
	value, ok := registry.values[name]
	if !ok {
		return profile.Profile{}, profile.ErrNotFound
	}
	return value, nil
}

func (registry *fakeRegistry) Add(_ context.Context, value profile.Profile) error {
	if registry.mutationErr != nil {
		if registry.committed {
			registry.values[value.Name] = value
			return &profile.CommitError{Err: registry.mutationErr}
		}
		return registry.mutationErr
	}
	registry.values[value.Name] = value
	return nil
}

func (registry *fakeRegistry) Put(ctx context.Context, value profile.Profile) error {
	return registry.Add(ctx, value)
}

func (registry *fakeRegistry) Remove(_ context.Context, name string) error {
	if registry.mutationErr != nil {
		return registry.mutationErr
	}
	if _, ok := registry.values[name]; !ok {
		return profile.ErrNotFound
	}
	delete(registry.values, name)
	return nil
}

func TestLoginRequiresConfirmationBeforeOverwrite(t *testing.T) {
	t.Parallel()
	candidate := profile.Profile{Name: "work", BaseURL: "https://new.example"}
	store := &fakeStore{values: map[string]Credential{"work": {Token: "old"}}}
	registry := &fakeRegistry{values: map[string]profile.Profile{"work": {Name: "work", BaseURL: "https://old.example"}}}
	err := Login(context.Background(), store, registry, candidate, Credential{Token: "new"}, false)
	if !errors.Is(err, ErrOverwriteConfirmationRequired) {
		t.Fatalf("Login() error = %v", err)
	}
	if store.values["work"].Token != "old" || registry.values["work"].BaseURL != "https://old.example" {
		t.Fatal("unconfirmed login changed existing state")
	}
}

func TestLoginCompensatesCredentialWhenMetadataFails(t *testing.T) {
	t.Parallel()
	store := &fakeStore{values: map[string]Credential{}}
	registry := &fakeRegistry{values: map[string]profile.Profile{}, mutationErr: errors.New("disk full")}
	candidate := profile.Profile{Name: "work", BaseURL: "https://redmine.example"}
	err := Login(context.Background(), store, registry, candidate, Credential{Token: "new"}, true)
	if err == nil {
		t.Fatal("Login() error = nil")
	}
	if _, ok := store.values["work"]; ok {
		t.Fatal("credential was not rolled back")
	}
	if _, ok := registry.values["work"]; ok {
		t.Fatal("metadata unexpectedly committed")
	}
}

func TestLoginRestoresPreviousCredentialWhenMetadataReplacementFails(t *testing.T) {
	t.Parallel()
	store := &fakeStore{values: map[string]Credential{"work": {Token: "old"}}}
	registry := &fakeRegistry{
		values:      map[string]profile.Profile{"work": {Name: "work", BaseURL: "https://old.example"}},
		mutationErr: errors.New("write failed"),
	}
	candidate := profile.Profile{Name: "work", BaseURL: "https://new.example"}
	err := Login(context.Background(), store, registry, candidate, Credential{Token: "new"}, true)
	if err == nil {
		t.Fatal("Login() error = nil")
	}
	if store.values["work"].Token != "old" {
		t.Fatal("previous credential was not restored")
	}
	if registry.values["work"].BaseURL != "https://old.example" {
		t.Fatal("previous metadata changed")
	}
}

func TestLoginKeepsCredentialAfterCommittedDurabilityWarning(t *testing.T) {
	t.Parallel()
	store := &fakeStore{values: map[string]Credential{}}
	registry := &fakeRegistry{
		values:      map[string]profile.Profile{},
		mutationErr: errors.New("directory sync failed"),
		committed:   true,
	}
	candidate := profile.Profile{Name: "work", BaseURL: "https://redmine.example"}
	err := Login(context.Background(), store, registry, candidate, Credential{Token: "new"}, true)
	if err == nil || !profile.WasCommitted(err) {
		t.Fatalf("Login() error = %v, want committed warning", err)
	}
	if store.values["work"].Token != "new" || registry.values["work"] != candidate {
		t.Fatal("committed state was incorrectly rolled back")
	}
}

func TestCredentialFormattingIsAlwaysRedacted(t *testing.T) {
	t.Parallel()
	credential := Credential{Token: "super-secret"}
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		if got := fmt.Sprintf(format, credential); got != "<redacted>" {
			t.Fatalf("format %s = %q", format, got)
		}
	}
}
