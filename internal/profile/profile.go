// Package profile stores non-secret Redmine connection metadata.
package profile

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const maxNameLength = 64

var (
	ErrProfileRequired     = errors.New("a profile is required for every invocation")
	ErrInvalidProfile      = errors.New("invalid profile")
	ErrNotFound            = errors.New("profile not found")
	ErrAlreadyExists       = errors.New("profile already exists")
	ErrCorruptRegistry     = errors.New("profile registry is corrupt")
	ErrInsecurePermissions = errors.New("profile registry has insecure permissions")

	namePattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	pathSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
)

// CommitError reports that metadata was atomically renamed before a later
// durability operation failed.
type CommitError struct{ Err error }

func (e *CommitError) Error() string {
	return fmt.Sprintf("profile metadata committed but durability check failed: %v", e.Err)
}

func (e *CommitError) Unwrap() error { return e.Err }

// WasCommitted reports whether a registry error happened after atomic rename.
func WasCommitted(err error) bool {
	var committed *CommitError
	return errors.As(err, &committed)
}

// Profile contains non-secret connection metadata only.
type Profile struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
}

// RequireName enforces explicit per-invocation profile selection.
func RequireName(name string) error {
	if name == "" {
		return ErrProfileRequired
	}
	return ValidateName(name)
}

// ValidateName validates names used as exact Keychain account identifiers.
func ValidateName(name string) error {
	if name == "" || len(name) > maxNameLength || !namePattern.MatchString(name) {
		return fmt.Errorf("%w: name must be 1-%d ASCII letters, digits, dot, dash, or underscore and start with a letter or digit", ErrInvalidProfile, maxNameLength)
	}
	return nil
}

// Validate checks all profile metadata.
func (p Profile) Validate() error {
	if err := ValidateName(p.Name); err != nil {
		return err
	}
	normalized, err := NormalizeBaseURL(p.BaseURL)
	if err != nil {
		return err
	}
	if normalized != p.BaseURL {
		return fmt.Errorf("%w: base_url must be canonical %q", ErrInvalidProfile, normalized)
	}
	return nil
}

// NormalizeBaseURL validates a self-hosted Redmine HTTPS origin and optional
// canonical base path. Credentials, queries, fragments, IPs, localhost, encoded
// paths, and dot segments are rejected before any token is read.
func NormalizeBaseURL(raw string) (string, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\x00\r\n\t") {
		return "", fmt.Errorf("%w: base URL is empty or contains whitespace", ErrInvalidProfile)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: parse base URL: %v", ErrInvalidProfile, err)
	}
	if parsed.Scheme != "https" || parsed.Opaque != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", fmt.Errorf("%w: base URL must be HTTPS with no credentials, query, or fragment", ErrInvalidProfile)
	}
	host := parsed.Hostname()
	if host == "" || host != strings.ToLower(host) || net.ParseIP(host) != nil || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return "", fmt.Errorf("%w: base URL must use a lowercase DNS host, not localhost or an IP address", ErrInvalidProfile)
	}
	for _, label := range strings.Split(host, ".") {
		if !validDNSLabel(label) {
			return "", fmt.Errorf("%w: base URL contains an invalid DNS host", ErrInvalidProfile)
		}
	}
	port := parsed.Port()
	if port != "" {
		value, parseErr := strconv.Atoi(port)
		if parseErr != nil || value < 1 || value > 65535 {
			return "", fmt.Errorf("%w: base URL port is invalid", ErrInvalidProfile)
		}
	}
	if parsed.EscapedPath() != parsed.Path {
		return "", fmt.Errorf("%w: encoded base paths are not supported", ErrInvalidProfile)
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path != "" {
		for _, segment := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
			if segment == "" || segment == "." || segment == ".." || !pathSegmentPattern.MatchString(segment) {
				return "", fmt.Errorf("%w: base path must contain canonical URL-safe segments", ErrInvalidProfile)
			}
		}
	}
	hostPort := host
	if port != "" {
		hostPort += ":" + port
	}
	return "https://" + hostPort + path, nil
}

func validDNSLabel(label string) bool {
	if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, ch := range label {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return false
		}
	}
	return true
}
