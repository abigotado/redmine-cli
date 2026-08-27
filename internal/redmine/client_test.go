package redmine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/abigotado/redmine-cli/internal/errx"
)

const secretSentinel = "REDMINE_TOKEN_SENTINEL_DO_NOT_EMIT"

func TestCredentialFormattingAndJSONAreRedacted(t *testing.T) {
	t.Parallel()
	credential := Credential{Token: secretSentinel}
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		if got := fmt.Sprintf(format, credential); strings.Contains(got, secretSentinel) {
			t.Fatalf("format %q exposed the credential", format)
		}
	}
	raw, err := json.Marshal(credential)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(raw), secretSentinel) {
		t.Fatal("JSON exposed the credential")
	}
}

func TestMyselfOmitsAPIKey(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("X-Redmine-API-Key"); got != secretSentinel {
			t.Errorf("credential header = %q", got)
		}
		_, _ = io.WriteString(writer, `{"user":{"id":7,"login":"agent","firstname":"A","lastname":"User","api_key":"REDMINE_TOKEN_SENTINEL_DO_NOT_EMIT"}}`)
	}))
	defer server.Close()

	client := newForTest(
		server.URL,
		Credential{Token: secretSentinel},
		WithLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))),
	)
	user, err := client.Myself(context.Background())
	if err != nil {
		t.Fatalf("Myself() error = %v", err)
	}
	raw, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal SafeUser: %v", err)
	}
	combined := string(raw) + logs.String() + user.Login + user.FirstName + user.LastName
	if strings.Contains(combined, secretSentinel) || strings.Contains(combined, "api_key") {
		t.Fatalf("secret-bearing field escaped SafeUser boundary: %s", combined)
	}
}

func TestClientRefusesRedirectBeforeCredentialReachesSecondOrigin(t *testing.T) {
	t.Parallel()
	var reached bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reached = true
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+"/users/current.json", http.StatusFound)
	}))
	defer source.Close()

	client := newForTest(source.URL, Credential{Token: secretSentinel})
	_, err := client.Myself(context.Background())
	if err == nil {
		t.Fatal("Myself() error = nil, want redirect refusal")
	}
	if reached {
		t.Fatal("redirect target was reached")
	}
	if strings.Contains(err.Error(), secretSentinel) || strings.Contains(err.Error(), target.URL) {
		t.Fatalf("redirect error leaked sensitive request data: %v", err)
	}
}

func TestStatusTranslationDoesNotExposeBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		code int
		exit errx.Code
	}{
		{name: "unauthorized", code: http.StatusUnauthorized, exit: errx.CodeAuth},
		{name: "forbidden", code: http.StatusForbidden, exit: errx.CodePermission},
		{name: "not found", code: http.StatusNotFound, exit: errx.CodeNotFound},
		{name: "bad request", code: http.StatusBadRequest, exit: errx.CodeUsage},
		{name: "conflict", code: http.StatusConflict, exit: errx.CodeConflict},
		{name: "server", code: http.StatusServiceUnavailable, exit: errx.CodeRetryable},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.code)
				_, _ = io.WriteString(writer, secretSentinel)
			}))
			defer server.Close()
			client := newForTest(server.URL, Credential{Token: secretSentinel}, WithSleep(func(context.Context, time.Duration) error { return nil }))
			_, err := client.Myself(context.Background())
			if errx.ExitCode(err) != testCase.exit {
				t.Fatalf("exit = %d, want %d (err %v)", errx.ExitCode(err), testCase.exit, err)
			}
			if strings.Contains(err.Error(), secretSentinel) {
				t.Fatalf("error leaked upstream body: %v", err)
			}
		})
	}
}

func TestRateLimitUsesRetryAfter(t *testing.T) {
	t.Parallel()
	var attempts int
	var delays []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			writer.Header().Set("Retry-After", "2")
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(writer, `{"user":{"id":1,"login":"ok"}}`)
	}))
	defer server.Close()
	client := newForTest(server.URL, Credential{Token: secretSentinel}, WithSleep(func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}))
	if _, err := client.Myself(context.Background()); err != nil {
		t.Fatalf("Myself() error = %v", err)
	}
	if attempts != 2 || len(delays) != 1 || delays[0] != 2*time.Second {
		t.Fatalf("attempts=%d delays=%v", attempts, delays)
	}
}

func TestRateLimitBeyondDeadlineReturnsAdvertisedErrorWithoutSleeping(t *testing.T) {
	t.Parallel()
	var attempts int
	var sleeps int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		writer.Header().Set("Retry-After", "120")
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := newForTest(server.URL, Credential{Token: secretSentinel}, WithSleep(func(context.Context, time.Duration) error {
		sleeps++
		return nil
	}))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_, err := client.Myself(ctx)

	var typed *errx.Error
	if !errors.As(err, &typed) {
		t.Fatalf("error = %v, want *errx.Error", err)
	}
	if typed.Reason != "RATE_LIMITED" || typed.RetryAfter != 2*time.Minute {
		t.Fatalf("error reason=%q retry_after=%s", typed.Reason, typed.RetryAfter)
	}
	if attempts != 1 || sleeps != 0 {
		t.Fatalf("attempts=%d sleeps=%d", attempts, sleeps)
	}
}

func TestParseRetryAfterClampsDeltaSecondsWithoutOverflow(t *testing.T) {
	t.Parallel()
	maxWholeSeconds := uint64(maxRetryAfter / time.Second)
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "largest safe duration", value: strconv.FormatUint(maxWholeSeconds, 10), want: time.Duration(maxWholeSeconds) * time.Second},
		{name: "duration overflow", value: strconv.FormatUint(maxWholeSeconds+1, 10), want: maxRetryAfter},
		{name: "maximum uint64", value: strconv.FormatUint(^uint64(0), 10), want: maxRetryAfter},
		{name: "uint64 overflow", value: strings.Repeat("9", 64), want: maxRetryAfter},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := parseRetryAfter(testCase.value, time.Time{}); got != testCase.want {
				t.Fatalf("parseRetryAfter(%q) = %s, want %s", testCase.value, got, testCase.want)
			}
		})
	}
}

func TestOversizedAndInvalidJSONAreSafe(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{name: "oversized", body: `{"user":{"login":"` + strings.Repeat("x", maxResponseBody) + `"}}`},
		{name: "invalid", body: `{"user":` + secretSentinel},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(writer, testCase.body)
			}))
			defer server.Close()
			client := newForTest(server.URL, Credential{Token: secretSentinel})
			_, err := client.Myself(context.Background())
			if err == nil || errx.ExitCode(err) != errx.CodeInternal {
				t.Fatalf("error = %v, want internal", err)
			}
			if strings.Contains(err.Error(), secretSentinel) {
				t.Fatalf("error leaked body: %v", err)
			}
		})
	}
}

func TestResourceQueriesAndPagination(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/projects.json":
			if got := request.URL.Query().Get("limit"); got != "2" {
				t.Errorf("project limit = %q", got)
			}
			_, _ = io.WriteString(writer, `{"projects":[{"id":1,"identifier":"one","name":"One"},{"id":2,"identifier":"two","name":"Two"}],"total_count":3,"offset":0,"limit":2}`)
		case "/issues.json":
			query := request.URL.Query()
			want := map[string]string{
				"project_id": "12", "assigned_to_id": "me", "status_id": "open",
				"include": "attachments,relations", "sort": "updated_on:desc",
			}
			for key, value := range want {
				if query.Get(key) != value {
					t.Errorf("%s = %q, want %q", key, query.Get(key), value)
				}
			}
			_, _ = io.WriteString(writer, `{"issues":[{"id":9,"subject":"Nine","project":{"id":12,"name":"P"},"status":{"id":1,"name":"New"}}],"total_count":1,"offset":0,"limit":25}`)
		case "/issues/9.json":
			if request.URL.Query().Get("include") != "journals,watchers" {
				t.Errorf("include = %q", request.URL.Query().Get("include"))
			}
			_, _ = io.WriteString(writer, `{"issue":{"id":9,"subject":"Nine","project":{"id":12,"name":"P"},"status":{"id":1,"name":"New"}}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := newForTest(server.URL, Credential{Token: secretSentinel})
	projects, err := client.Projects(context.Background(), ProjectListOptions{Limit: 2})
	if err != nil || len(projects.Projects) != 2 {
		t.Fatalf("Projects() = %#v, %v", projects, err)
	}
	issues, err := client.Issues(context.Background(), IssueListOptions{
		Limit: 25, ProjectID: "12", AssignedToID: "me", StatusID: "open",
		Sort: "updated_on:desc", Include: []string{"relations", "attachments"},
	})
	if err != nil || len(issues.Issues) != 1 {
		t.Fatalf("Issues() = %#v, %v", issues, err)
	}
	if _, err := client.Issue(context.Background(), 9, []string{"watchers", "journals"}); err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
}

func TestCursorIsBoundToProfileURLResourceAndFilters(t *testing.T) {
	t.Parallel()
	query := url.Values{"status_id": {"open"}, "limit": {"25"}, "offset": {"0"}}
	fingerprint := Fingerprint("work", "https://redmine.example", "issues", query)
	cursor, err := EncodeCursor("issues", 25, fingerprint)
	if err != nil {
		t.Fatalf("EncodeCursor() error = %v", err)
	}
	offset, err := DecodeCursor(cursor, "issues", fingerprint)
	if err != nil || offset != 25 {
		t.Fatalf("DecodeCursor() = %d, %v", offset, err)
	}
	if err := ValidateCursor(cursor, "issues"); err != nil {
		t.Fatalf("ValidateCursor() error = %v", err)
	}
	if err := ValidateCursor("not-base64!", "issues"); err == nil || errx.ExitCode(err) != errx.CodeUsage {
		t.Fatalf("ValidateCursor() malformed error = %v", err)
	}
	if err := ValidateCursor(cursor, "projects"); err == nil || errx.ExitCode(err) != errx.CodeUsage {
		t.Fatalf("ValidateCursor() resource error = %v", err)
	}
	mismatches := []struct {
		resource    string
		fingerprint string
	}{
		{resource: "projects", fingerprint: fingerprint},
		{resource: "issues", fingerprint: Fingerprint("other", "https://redmine.example", "issues", query)},
		{resource: "issues", fingerprint: Fingerprint("work", "https://other.example", "issues", query)},
		{resource: "issues", fingerprint: Fingerprint("work", "https://redmine.example", "issues", url.Values{"status_id": {"closed"}})},
	}
	for _, mismatch := range mismatches {
		if _, err := DecodeCursor(cursor, mismatch.resource, mismatch.fingerprint); err == nil || errx.ExitCode(err) != errx.CodeUsage {
			t.Fatalf("DecodeCursor mismatch error = %v", err)
		}
	}
}

func TestValidationRejectsUnsupportedIssueInputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		options IssueListOptions
	}{
		{name: "text project identifier", options: IssueListOptions{Limit: 25, ProjectID: "delivery"}},
		{name: "unsupported include", options: IssueListOptions{Limit: 25, Include: []string{"journals"}}},
		{name: "unsupported sort", options: IssueListOptions{Limit: 25, Sort: "password:desc"}},
		{name: "oversized limit", options: IssueListOptions{Limit: 101}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := testCase.options.Query(); err == nil || errx.ExitCode(err) != errx.CodeUsage {
				t.Fatalf("Query() error = %v", err)
			}
		})
	}
}

func TestContextCancellationStopsRetry(t *testing.T) {
	t.Parallel()
	client := newForTest("http://127.0.0.1:1", Credential{Token: secretSentinel}, WithSleep(func(ctx context.Context, _ time.Duration) error {
		<-ctx.Done()
		return ctx.Err()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Myself(ctx)
	if !errors.Is(err, context.Canceled) && errx.ExitCode(err) != errx.CodeRetryable {
		t.Fatalf("Myself() error = %v", err)
	}
}
