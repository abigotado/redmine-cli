// Package redmine is a hardened read-only Redmine REST client.
package redmine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/abigotado/redmine-cli/internal/errx"
)

const (
	maxAttempts     = 3
	maxResponseBody = 4 << 20
	maxDrainBody    = 64 << 10
	maxRetryAfter   = time.Duration(1<<63 - 1)
)

var safePathSegment = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)

// Config contains non-secret Redmine connection metadata.
type Config struct {
	BaseURL string
}

// Credential contains one Redmine API token and always formats redacted.
type Credential struct {
	Token string `json:"-"`
}

// Format prevents accidental token formatting.
func (Credential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "<redacted>")
}

// Client talks only to one validated Redmine HTTPS base URL.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	log     *slog.Logger
	sleep   func(context.Context, time.Duration) error
	now     func() time.Time
}

// Option customizes testable boundaries without changing the production URL.
type Option func(*Client)

// WithHTTPClient replaces the transport while retaining redirect refusal.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient == nil {
			return
		}
		clone := *httpClient
		clone.CheckRedirect = refuseRedirect
		clone.Jar = nil
		client.http = &clone
	}
}

// WithLogger sets a logger that receives methods and retry metadata only.
func WithLogger(logger *slog.Logger) Option {
	return func(client *Client) {
		if logger != nil {
			client.log = logger
		}
	}
}

// WithSleep replaces retry sleeping for deterministic tests.
func WithSleep(sleep func(context.Context, time.Duration) error) Option {
	return func(client *Client) {
		if sleep != nil {
			client.sleep = sleep
		}
	}
}

// New constructs a client after re-validating the stored profile URL.
func New(config Config, credential Credential, options ...Option) (*Client, error) {
	if !validBaseURL(config.BaseURL) {
		return nil, errx.Usage("Redmine base URL is invalid")
	}
	if credential.Token == "" {
		return nil, errx.Auth("MISSING_TOKEN", "Redmine API token is missing")
	}
	client := &Client{
		baseURL: config.BaseURL,
		token:   credential.Token,
		http: &http.Client{
			CheckRedirect: refuseRedirect,
		},
		log:   slog.New(discardHandler{}),
		sleep: sleepContext,
		now:   time.Now,
	}
	for _, option := range options {
		option(client)
	}
	return client, nil
}

func validBaseURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Opaque != "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		parsed.EscapedPath() != parsed.Path || strings.TrimRight(parsed.Path, "/") != parsed.Path {
		return false
	}
	host := parsed.Hostname()
	if host == "" || host != strings.ToLower(host) || net.ParseIP(host) != nil ||
		host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	if parsed.Port() != "" {
		port, err := strconv.Atoi(parsed.Port())
		if err != nil || port < 1 || port > 65535 {
			return false
		}
	}
	if parsed.Path != "" {
		for _, segment := range strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/") {
			if segment == "" || segment == "." || segment == ".." || !safePathSegment.MatchString(segment) {
				return false
			}
		}
	}
	return true
}

// newForTest constructs an HTTP test client without weakening New.
func newForTest(baseURL string, credential Credential, options ...Option) *Client {
	client := &Client{
		baseURL: baseURL,
		token:   credential.Token,
		http: &http.Client{
			CheckRedirect: refuseRedirect,
		},
		log:   slog.New(discardHandler{}),
		sleep: sleepContext,
		now:   time.Now,
	}
	for _, option := range options {
		option(client)
	}
	return client
}

func refuseRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

type request struct {
	path  string
	query url.Values
}

func (client *Client) get(ctx context.Context, request request, out any) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		response, err := client.send(ctx, request)
		if err != nil {
			if ctx.Err() != nil {
				return errx.Translate(ctx.Err())
			}
			lastErr = errx.Retryable("NETWORK", 0, "could not reach Redmine")
			if attempt == maxAttempts {
				return lastErr
			}
			if err := client.sleep(ctx, retryBackoff(attempt)); err != nil {
				return translateSleepError(ctx, err)
			}
			continue
		}
		delay, retry, translated := client.handle(response, request, out)
		if translated == nil {
			return nil
		}
		lastErr = translated
		if !retry || attempt == maxAttempts {
			return translated
		}
		if delay <= 0 {
			delay = retryBackoff(attempt)
		}
		if advertisedRetryAfter(translated) > 0 && !delayFitsDeadline(ctx, delay) {
			if err := ctx.Err(); err != nil {
				return errx.Translate(err)
			}
			return translated
		}
		client.log.Debug("retrying Redmine request", "method", http.MethodGet, "attempt", attempt, "delay", delay)
		if err := client.sleep(ctx, delay); err != nil {
			return translateSleepError(ctx, err)
		}
	}
	return lastErr
}

func (client *Client) send(ctx context.Context, request request) (*http.Response, error) {
	fullURL := client.baseURL + request.path
	if len(request.query) > 0 {
		fullURL += "?" + request.query.Encode()
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, errors.New("invalid Redmine request")
	}
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("X-Redmine-API-Key", client.token)
	client.log.Debug("Redmine request", "method", http.MethodGet)
	return client.http.Do(httpRequest)
}

func (client *Client) handle(response *http.Response, request request, out any) (time.Duration, bool, error) {
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxDrainBody))
		_ = response.Body.Close()
	}()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		if out == nil {
			return 0, false, nil
		}
		limited := &io.LimitedReader{R: response.Body, N: maxResponseBody + 1}
		decoder := json.NewDecoder(limited)
		if err := decoder.Decode(out); err != nil {
			if limited.N == 0 {
				return 0, false, errx.Internal("Redmine response exceeds the %d-byte safety limit", maxResponseBody)
			}
			return 0, false, errx.Internal("Redmine returned an invalid JSON response")
		}
		var extra json.RawMessage
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			if limited.N == 0 {
				return 0, false, errx.Internal("Redmine response exceeds the %d-byte safety limit", maxResponseBody)
			}
			return 0, false, errx.Internal("Redmine returned trailing JSON data")
		}
		if limited.N == 0 {
			return 0, false, errx.Internal("Redmine response exceeds the %d-byte safety limit", maxResponseBody)
		}
		return 0, false, nil
	}
	return client.translateStatus(response, request)
}

func (client *Client) translateStatus(response *http.Response, request request) (time.Duration, bool, error) {
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return 0, false, errx.Auth("AUTHENTICATION_FAILED", "Redmine rejected the API token")
	case http.StatusForbidden:
		return 0, false, errx.Permission("PERMISSION_DENIED", "the Redmine account cannot read this resource")
	case http.StatusNotFound:
		return 0, false, errx.NotFound(resourceKind(request.path), "requested", nil)
	case http.StatusConflict:
		return 0, false, errx.Conflict("CONFLICT", "Redmine reported a stale resource conflict")
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return 0, false, errx.Usage("Redmine rejected the request parameters")
	case http.StatusTooManyRequests:
		delay := parseRetryAfter(response.Header.Get("Retry-After"), client.now())
		return delay, true, errx.Retryable("RATE_LIMITED", delay, "Redmine rate limit reached")
	default:
		if response.StatusCode >= http.StatusInternalServerError {
			return 0, true, errx.Retryable("SERVER_ERROR", 0, "Redmine is temporarily unavailable")
		}
		return 0, false, errx.Internal("Redmine returned unexpected HTTP status %d", response.StatusCode)
	}
}

func resourceKind(path string) string {
	switch {
	case strings.Contains(path, "/issues/"):
		return "issue"
	case strings.Contains(path, "/projects/"):
		return "project"
	default:
		return "resource"
	}
}

func retryBackoff(attempt int) time.Duration {
	return time.Duration(1<<(attempt-1)) * 250 * time.Millisecond
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	trimmed := strings.TrimSpace(value)
	if seconds, err := strconv.ParseUint(trimmed, 10, 64); err == nil && seconds > 0 {
		if seconds > uint64(maxRetryAfter/time.Second) {
			return maxRetryAfter
		}
		return time.Duration(seconds) * time.Second
	} else if errors.Is(err, strconv.ErrRange) && decimalDigits(trimmed) {
		return maxRetryAfter
	}
	when, err := http.ParseTime(trimmed)
	if err != nil || !when.After(now) {
		return 0
	}
	return when.Sub(now)
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func advertisedRetryAfter(err error) time.Duration {
	var typed *errx.Error
	if errors.As(err, &typed) {
		return typed.RetryAfter
	}
	return 0
}

func delayFitsDeadline(ctx context.Context, delay time.Duration) bool {
	deadline, ok := ctx.Deadline()
	return !ok || delay <= time.Until(deadline)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func translateSleepError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return errx.Translate(ctx.Err())
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errx.Translate(err)
	}
	return errx.Internal("Redmine retry delay failed")
}

type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool   { return false }
func (discardHandler) Handle(context.Context, slog.Record) error  { return nil }
func (handler discardHandler) WithAttrs([]slog.Attr) slog.Handler { return handler }
func (handler discardHandler) WithGroup(string) slog.Handler      { return handler }
