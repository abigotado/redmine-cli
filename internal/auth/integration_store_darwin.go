//go:build darwin && cgo && keychainintegration

package auth

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// IntegrationStore targets one explicit disposable Keychain. It is compiled
// only for the opt-in cross-binary integration test.
type IntegrationStore struct {
	KeychainStore
	ref C.SecKeychainRef
}

// NewIntegrationStore creates or opens and unlocks an explicit Keychain path.
func NewIntegrationStore(path, password string, create bool) (*IntegrationStore, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	passwordBytes := []byte(password)
	var passwordPointer unsafe.Pointer
	if len(passwordBytes) > 0 {
		passwordPointer = unsafe.Pointer(&passwordBytes[0])
	}
	var keychain C.SecKeychainRef
	var status C.OSStatus
	if create {
		status = C.SecKeychainCreate(cPath, C.UInt32(len(passwordBytes)), passwordPointer, C.false, C.SecAccessRef(0), &keychain)
	} else {
		status = C.SecKeychainOpen(cPath, &keychain)
	}
	if status != C.errSecSuccess {
		return nil, &StatusError{Operation: "integration open", Status: int64(status)}
	}
	if keychain == 0 {
		return nil, errors.New("integration Keychain returned no reference")
	}
	status = C.SecKeychainUnlock(keychain, C.UInt32(len(passwordBytes)), passwordPointer, C.true)
	if status != C.errSecSuccess {
		C.CFRelease(C.CFTypeRef(keychain))
		return nil, &StatusError{Operation: "integration unlock", Status: int64(status)}
	}
	return &IntegrationStore{KeychainStore: KeychainStore{keychain: keychain}, ref: keychain}, nil
}

// Delete removes the disposable Keychain itself.
func (store *IntegrationStore) Delete() error {
	if store == nil || store.ref == 0 {
		return nil
	}
	status := C.SecKeychainDelete(store.ref)
	if status != C.errSecSuccess {
		return &StatusError{Operation: "integration delete", Status: int64(status)}
	}
	return nil
}

// Close releases the explicit Keychain reference.
func (store *IntegrationStore) Close() error {
	if store == nil || store.ref == 0 {
		return nil
	}
	C.CFRelease(C.CFTypeRef(store.ref))
	store.ref = 0
	store.KeychainStore.keychain = 0
	return nil
}

func integrationError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s disposable Keychain: %w", operation, err)
}
