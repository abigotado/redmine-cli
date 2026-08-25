//go:build darwin && cgo

package auth

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>

static CFMutableDictionaryRef redmine_query(CFStringRef service, CFStringRef account, SecKeychainRef keychain, Boolean search) {
	CFMutableDictionaryRef query = CFDictionaryCreateMutable(
		kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks
	);
	if (query == NULL) return NULL;
	CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
	CFDictionarySetValue(query, kSecAttrService, service);
	CFDictionarySetValue(query, kSecAttrAccount, account);
	CFDictionarySetValue(query, kSecUseAuthenticationUI, kSecUseAuthenticationUIFail);
	if (keychain != NULL && search) {
		const void *values[] = {keychain};
		CFArrayRef searchList = CFArrayCreate(kCFAllocatorDefault, values, 1, &kCFTypeArrayCallBacks);
		if (searchList == NULL) {
			CFRelease(query);
			return NULL;
		}
		CFDictionarySetValue(query, kSecMatchSearchList, searchList);
		CFRelease(searchList);
	} else if (keychain != NULL) {
		CFDictionarySetValue(query, kSecUseKeychain, keychain);
	}
	return query;
}

static OSStatus redmine_load(CFStringRef service, CFStringRef account, SecKeychainRef keychain, CFTypeRef *result) {
	CFMutableDictionaryRef query = redmine_query(service, account, keychain, true);
	if (query == NULL) return errSecAllocate;
	CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
	CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
	OSStatus status = SecItemCopyMatching(query, result);
	CFRelease(query);
	return status;
}

static OSStatus redmine_resolve(CFStringRef service, CFStringRef account, SecKeychainRef keychain, CFTypeRef *result) {
	CFMutableDictionaryRef query = redmine_query(service, account, keychain, true);
	if (query == NULL) return errSecAllocate;
	CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
	CFDictionarySetValue(query, kSecReturnRef, kCFBooleanTrue);
	OSStatus status = SecItemCopyMatching(query, result);
	CFRelease(query);
	return status;
}

static OSStatus redmine_create_allow_any_access(CFStringRef description, SecAccessRef *access) {
	CFArrayRef trusted = CFArrayCreate(kCFAllocatorDefault, NULL, 0, &kCFTypeArrayCallBacks);
	if (trusted == NULL) return errSecAllocate;
	OSStatus status = SecAccessCreate(description, trusted, access);
	CFRelease(trusted);
	if (status != errSecSuccess) return status;

	CFArrayRef acls = SecAccessCopyMatchingACLList(*access, kSecACLAuthorizationDecrypt);
	if (acls == NULL || CFArrayGetCount(acls) != 1) {
		if (acls != NULL) CFRelease(acls);
		return errSecInternalComponent;
	}
	SecACLRef acl = (SecACLRef)CFArrayGetValueAtIndex(acls, 0);
	CFArrayRef applications = NULL;
	CFStringRef aclDescription = NULL;
	SecKeychainPromptSelector prompt = 0;
	status = SecACLCopyContents(acl, &applications, &aclDescription, &prompt);
	if (applications != NULL) CFRelease(applications);
	if (status == errSecSuccess) {
		prompt &= ~kSecKeychainPromptRequirePassphase;
		status = SecACLSetContents(acl, NULL, aclDescription, prompt);
	}
	if (aclDescription != NULL) CFRelease(aclDescription);
	CFRelease(acls);
	return status;
}

static OSStatus redmine_add(CFStringRef service, CFStringRef account, SecKeychainRef keychain, CFDataRef value, SecAccessRef access) {
	CFMutableDictionaryRef attributes = redmine_query(service, account, keychain, false);
	if (attributes == NULL) return errSecAllocate;
	CFDictionarySetValue(attributes, kSecValueData, value);
	CFDictionarySetValue(attributes, kSecAttrAccess, access);
	OSStatus status = SecItemAdd(attributes, NULL);
	CFRelease(attributes);
	return status;
}

static OSStatus redmine_update(CFStringRef service, CFStringRef account, SecKeychainRef keychain, CFDataRef value) {
	CFMutableDictionaryRef query = redmine_query(service, account, keychain, true);
	if (query == NULL) return errSecAllocate;
	const void *keys[] = { kSecValueData };
	const void *values[] = { value };
	CFDictionaryRef attributes = CFDictionaryCreate(
		kCFAllocatorDefault, keys, values, 1,
		&kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks
	);
	if (attributes == NULL) {
		CFRelease(query);
		return errSecAllocate;
	}
	OSStatus status = SecItemUpdate(query, attributes);
	CFRelease(attributes);
	CFRelease(query);
	return status;
}

static OSStatus redmine_normalize_item_access(SecKeychainItemRef item, CFStringRef description) {
	SecAccessRef access = NULL;
	OSStatus status = redmine_create_allow_any_access(description, &access);
	if (status != errSecSuccess) return status;
	status = SecKeychainItemSetAccess(item, access);
	CFRelease(access);
	return status;
}

static OSStatus redmine_item_has_allow_any_access(SecKeychainItemRef item, Boolean *allowAny) {
	*allowAny = false;
	SecAccessRef access = NULL;
	OSStatus status = SecKeychainItemCopyAccess(item, &access);
	if (status != errSecSuccess) return status;
	CFArrayRef acls = SecAccessCopyMatchingACLList(access, kSecACLAuthorizationDecrypt);
	if (acls == NULL || CFArrayGetCount(acls) != 1) {
		if (acls != NULL) CFRelease(acls);
		CFRelease(access);
		return errSecInternalComponent;
	}
	SecACLRef acl = (SecACLRef)CFArrayGetValueAtIndex(acls, 0);
	CFArrayRef applications = NULL;
	CFStringRef aclDescription = NULL;
	SecKeychainPromptSelector prompt = 0;
	status = SecACLCopyContents(acl, &applications, &aclDescription, &prompt);
	if (status == errSecSuccess) {
		*allowAny = applications == NULL && (prompt & kSecKeychainPromptRequirePassphase) == 0;
	}
	if (applications != NULL) CFRelease(applications);
	if (aclDescription != NULL) CFRelease(aclDescription);
	CFRelease(acls);
	CFRelease(access);
	return status;
}

static OSStatus redmine_delete(CFStringRef service, CFStringRef account, SecKeychainRef keychain) {
	CFMutableDictionaryRef query = redmine_query(service, account, keychain, true);
	if (query == NULL) return errSecAllocate;
	OSStatus status = SecItemDelete(query);
	CFRelease(query);
	return status;
}
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/abigotado/redmine-cli/internal/profile"
)

// KeychainStore stores Redmine tokens as generic-password items in macOS
// Keychain. Ordinary reads and deletes disallow authentication UI.
type KeychainStore struct {
	keychain C.SecKeychainRef
}

// Load retrieves the token for the exact profile account.
func (store KeychainStore) Load(ctx context.Context, profileName string) (Credential, error) {
	if err := ctx.Err(); err != nil {
		return Credential{}, err
	}
	if err := profile.ValidateName(profileName); err != nil {
		return Credential{}, err
	}
	service, releaseService, err := makeCFString(KeychainService)
	if err != nil {
		return Credential{}, err
	}
	defer releaseService()
	account, releaseAccount, err := makeCFString(profileName)
	if err != nil {
		return Credential{}, err
	}
	defer releaseAccount()

	var result C.CFTypeRef
	status := C.redmine_load(service, account, store.keychain, &result)
	if status != C.errSecSuccess {
		return Credential{}, translateStatus("load", status)
	}
	if result == 0 || C.CFGetTypeID(result) != C.CFDataGetTypeID() {
		if result != 0 {
			C.CFRelease(result)
		}
		return Credential{}, fmt.Errorf("stored credential has an invalid Keychain type: %w", ErrInvalidToken)
	}
	defer C.CFRelease(result)
	data := C.CFDataRef(result)
	length := C.CFDataGetLength(data)
	if length <= 0 || length > C.CFIndex(MaxTokenBytes) {
		return Credential{}, fmt.Errorf("stored credential is invalid: %w", ErrInvalidToken)
	}
	bytes := C.CFDataGetBytePtr(data)
	credential := Credential{Token: C.GoStringN((*C.char)(unsafe.Pointer(bytes)), C.int(length))}
	if err := credential.Validate(); err != nil {
		return Credential{}, fmt.Errorf("stored credential is invalid: %w", err)
	}
	return credential, nil
}

// Save creates or replaces a token. Existing items have their decrypt ACL
// normalized during explicit login so a source-built binary remains usable
// after its executable identity changes.
func (store KeychainStore) Save(ctx context.Context, profileName string, credential Credential) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := profile.ValidateName(profileName); err != nil {
		return err
	}
	if err := credential.Validate(); err != nil {
		return err
	}
	service, releaseService, err := makeCFString(KeychainService)
	if err != nil {
		return err
	}
	defer releaseService()
	account, releaseAccount, err := makeCFString(profileName)
	if err != nil {
		return err
	}
	defer releaseAccount()
	value := C.CFDataCreate(
		C.kCFAllocatorDefault,
		(*C.UInt8)(unsafe.Pointer(unsafe.StringData(credential.Token))),
		C.CFIndex(len(credential.Token)),
	)
	if value == 0 {
		return &StatusError{Operation: "save", Status: int64(C.errSecAllocate)}
	}
	defer C.CFRelease(C.CFTypeRef(value))

	description, releaseDescription, err := makeCFString("redmine-cli profile " + profileName)
	if err != nil {
		return err
	}
	defer releaseDescription()
	var access C.SecAccessRef
	status := C.redmine_create_allow_any_access(description, &access)
	if status != C.errSecSuccess {
		return translateStatus("create access", status)
	}
	defer C.CFRelease(C.CFTypeRef(access))
	status = C.redmine_add(service, account, store.keychain, value, access)
	if status == C.errSecDuplicateItem {
		var item C.CFTypeRef
		status = C.redmine_resolve(service, account, store.keychain, &item)
		if status != C.errSecSuccess {
			return translateStatus("resolve", status)
		}
		if item == 0 || C.CFGetTypeID(item) != C.SecKeychainItemGetTypeID() {
			if item != 0 {
				C.CFRelease(item)
			}
			return &StatusError{Operation: "resolve", Status: int64(C.errSecInvalidItemRef)}
		}
		var allowAny C.Boolean
		status = C.redmine_item_has_allow_any_access(C.SecKeychainItemRef(item), &allowAny)
		if status == C.errSecSuccess && allowAny == C.false {
			status = C.redmine_normalize_item_access(C.SecKeychainItemRef(item), description)
		}
		C.CFRelease(item)
		if status != C.errSecSuccess {
			return translateStatus("normalize access", status)
		}
		status = C.redmine_update(service, account, store.keychain, value)
	}
	if status != C.errSecSuccess {
		return translateStatus("save", status)
	}
	return nil
}

// Delete removes the token for the exact profile account.
func (store KeychainStore) Delete(ctx context.Context, profileName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := profile.ValidateName(profileName); err != nil {
		return err
	}
	service, releaseService, err := makeCFString(KeychainService)
	if err != nil {
		return err
	}
	defer releaseService()
	account, releaseAccount, err := makeCFString(profileName)
	if err != nil {
		return err
	}
	defer releaseAccount()
	status := C.redmine_delete(service, account, store.keychain)
	if status == C.errSecItemNotFound {
		return nil
	}
	if status != C.errSecSuccess {
		return translateStatus("delete", status)
	}
	return nil
}

func makeCFString(value string) (C.CFStringRef, func(), error) {
	result := C.CFStringCreateWithBytes(
		C.kCFAllocatorDefault,
		(*C.UInt8)(unsafe.Pointer(unsafe.StringData(value))),
		C.CFIndex(len(value)),
		C.kCFStringEncodingUTF8,
		C.false,
	)
	if result == 0 {
		return 0, func() {}, &StatusError{Operation: "allocate", Status: int64(C.errSecAllocate)}
	}
	return result, func() { C.CFRelease(C.CFTypeRef(result)) }, nil
}

func translateStatus(operation string, status C.OSStatus) error {
	switch status {
	case C.errSecItemNotFound:
		return ErrNotFound
	case C.errSecInteractionNotAllowed, C.errSecAuthFailed:
		return ErrInteractionNotAllowed
	default:
		return &StatusError{Operation: operation, Status: int64(status)}
	}
}
