//go:build darwin && cgo && keychainintegration

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/abigotado/redmine-cli/internal/auth"
)

const (
	profileName = "integration"
	sentinelV1  = "non-secret-cross-binary-sentinel-v1"
	sentinelV2  = "non-secret-cross-binary-sentinel-v2"
)

var helperIdentity = "unset"

func main() {
	if len(os.Args) != 4 {
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	action, path, password := os.Args[1], os.Args[2], os.Args[3]
	if helperIdentity == "" {
		os.Exit(2)
	}
	create := action == "create-save"
	store, err := auth.NewIntegrationStore(path, password, create)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "open disposable Keychain failed")
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()
	switch action {
	case "create-save":
		err = store.Save(ctx, profileName, auth.Credential{Token: sentinelV1})
	case "load-v1", "load-v2":
		var credential auth.Credential
		credential, err = store.Load(ctx, profileName)
		expected := sentinelV1
		if action == "load-v2" {
			expected = sentinelV2
		}
		if err == nil && credential.Token != expected {
			err = fmt.Errorf("sentinel mismatch")
		}
	case "overwrite-v2":
		err = store.Save(ctx, profileName, auth.Credential{Token: sentinelV2})
	case "delete-item":
		err = store.KeychainStore.Delete(ctx, profileName)
	case "expect-missing":
		_, err = store.Load(ctx, profileName)
		if errors.Is(err, auth.ErrNotFound) {
			err = nil
		} else if err == nil {
			err = fmt.Errorf("credential still exists")
		}
	case "delete-keychain":
		err = store.Delete()
	default:
		os.Exit(2)
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "disposable Keychain operation failed")
		os.Exit(1)
	}
}
