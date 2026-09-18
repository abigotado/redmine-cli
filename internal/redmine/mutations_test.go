package redmine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abigotado/redmine-cli/internal/errx"
)

type mutationRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn mutationRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type sizedReader struct{ remaining int64 }

func (reader *sizedReader) Read(buffer []byte) (int, error) {
	if reader.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(buffer)) > reader.remaining {
		buffer = buffer[:reader.remaining]
	}
	for index := range buffer {
		buffer[index] = 'x'
	}
	reader.remaining -= int64(len(buffer))
	return len(buffer), nil
}

func TestMutationMethodsPayloadsAndReadback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		run  func(context.Context, *Client) error
		path string
		want string
	}{
		{
			name: "create issue",
			path: "/issues.json",
			want: `{"issue":{"project_id":1,"subject":"Ship it"}}`,
			run: func(ctx context.Context, client *Client) error {
				_, err := client.CreateIssue(ctx, map[string]any{"project_id": 1, "subject": "Ship it"})
				return err
			},
		},
		{
			name: "update issue reads back result",
			path: "/issues/7.json",
			want: `{"issue":{"status_id":2}}`,
			run: func(ctx context.Context, client *Client) error {
				_, err := client.UpdateIssue(ctx, 7, map[string]any{"status_id": 2})
				return err
			},
		},
		{
			name: "create project",
			path: "/projects.json",
			want: `{"project":{"identifier":"ship-it","name":"Ship it"}}`,
			run: func(ctx context.Context, client *Client) error {
				_, err := client.CreateProject(ctx, map[string]any{"name": "Ship it", "identifier": "ship-it"})
				return err
			},
		},
		{
			name: "update project reads back result",
			path: "/projects/3.json",
			want: `{"project":{"name":"Renamed"}}`,
			run: func(ctx context.Context, client *Client) error {
				_, err := client.UpdateProject(ctx, 3, map[string]any{"name": "Renamed"})
				return err
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var requests int
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests++
				if request.Header.Get("X-Redmine-API-Key") != secretSentinel {
					t.Errorf("missing API key")
				}
				if requests == 1 {
					if request.Method != http.MethodPost && request.Method != http.MethodPut {
						t.Errorf("method=%s", request.Method)
					}
					if request.URL.Path != testCase.path {
						t.Errorf("path=%q want %q", request.URL.Path, testCase.path)
					}
					body, err := io.ReadAll(request.Body)
					if err != nil {
						t.Errorf("read request: %v", err)
					}
					if string(body) != testCase.want {
						t.Errorf("body=%s want %s", body, testCase.want)
					}
					if request.Method == http.MethodPut {
						writer.WriteHeader(http.StatusNoContent)
						return
					}
					if strings.HasPrefix(request.URL.Path, "/issues") {
						_, _ = io.WriteString(writer, `{"issue":{"id":7,"subject":"Ship it","project":{"id":1,"name":"P"},"status":{"id":1,"name":"New"}}}`)
						return
					}
					_, _ = io.WriteString(writer, `{"project":{"id":3,"name":"Ship it","identifier":"ship-it"}}`)
					return
				}
				if request.Method != http.MethodGet || request.URL.Path != testCase.path {
					t.Errorf("readback method=%s path=%q", request.Method, request.URL.Path)
				}
				if strings.HasPrefix(request.URL.Path, "/issues") {
					_, _ = io.WriteString(writer, `{"issue":{"id":7,"subject":"Ship it","project":{"id":1,"name":"P"},"status":{"id":2,"name":"In progress"}}}`)
					return
				}
				_, _ = io.WriteString(writer, `{"project":{"id":3,"name":"Renamed","identifier":"ship-it"}}`)
			}))
			defer server.Close()

			if err := testCase.run(context.Background(), newForTest(server.URL, Credential{Token: secretSentinel})); err != nil {
				t.Fatalf("mutation error: %v", err)
			}
			if strings.Contains(testCase.name, "reads back") && requests != 2 {
				t.Fatalf("requests=%d want 2", requests)
			}
		})
	}
}

func TestMutationStatusErrorsNeverExposeUpstreamBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		code   errx.Code
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, code: errx.CodeAuth},
		{name: "forbidden", status: http.StatusForbidden, code: errx.CodePermission},
		{name: "not found", status: http.StatusNotFound, code: errx.CodeNotFound},
		{name: "invalid request", status: http.StatusUnprocessableEntity, code: errx.CodeUsage},
		{name: "conflict", status: http.StatusConflict, code: errx.CodeConflict},
		{name: "rate limited", status: http.StatusTooManyRequests, code: errx.CodeRetryable},
		{name: "server error may have applied", status: http.StatusBadGateway, code: errx.CodeWriteOutcomeUnknown},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.status)
				_, _ = io.WriteString(writer, secretSentinel)
			}))
			defer server.Close()

			_, err := newForTest(server.URL, Credential{Token: secretSentinel}).CreateIssue(context.Background(), map[string]any{"project_id": 1, "subject": "Ship it"})
			if errx.ExitCode(err) != testCase.code {
				t.Fatalf("code=%d want %d, error=%v", errx.ExitCode(err), testCase.code, err)
			}
			if strings.Contains(err.Error(), secretSentinel) {
				t.Fatalf("error exposed upstream body: %v", err)
			}
		})
	}
}

func TestUploadAndFilesUseBoundedSafeRepresentations(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/uploads.json":
			if request.Method != http.MethodPost || request.URL.Query().Get("filename") != "notes.txt" {
				t.Errorf("upload request %s %s", request.Method, request.URL.String())
			}
			data, err := io.ReadAll(request.Body)
			if err != nil || string(data) != "hello" {
				t.Errorf("upload body=%q err=%v", data, err)
			}
			_, _ = io.WriteString(writer, `{"upload":{"token":"`+secretSentinel+`"}}`)
		case "/projects/demo/files.json":
			_, _ = io.WriteString(writer, `{"files":[{"id":1,"filename":"notes.txt","filesize":5}]}`)
		case "/projects/3/files.json":
			if request.Method != http.MethodPost {
				t.Errorf("add file method=%s", request.Method)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("read add-file body: %v", err)
			}
			want := `{"file":{"description":"notes","filename":"notes.txt","token":"` + secretSentinel + `","version_id":6}}`
			if string(body) != want {
				t.Errorf("add-file body=%s want %s", body, want)
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := newForTest(server.URL, Credential{Token: secretSentinel})
	token, err := client.Upload(context.Background(), "notes.txt", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("Upload() error=%v", err)
	}
	if rendered := fmt.Sprintf("%v", token); strings.Contains(rendered, secretSentinel) {
		t.Fatalf("upload token leaked through formatting: %s", rendered)
	}
	if encoded, err := json.Marshal(token); err != nil || strings.Contains(string(encoded), secretSentinel) {
		t.Fatalf("upload token leaked through JSON: %s err=%v", encoded, err)
	}
	if err := client.AddFile(context.Background(), 3, token, "notes.txt", "notes", 6); err != nil {
		t.Fatalf("AddFile() error=%v", err)
	}
	files, err := client.Files(context.Background(), "demo")
	if err != nil || len(files) != 1 || files[0].Filename != "notes.txt" {
		t.Fatalf("Files()=%#v err=%v", files, err)
	}
}

func TestUploadEnforcesBoundaryAtHTTPClient(t *testing.T) {
	tests := []struct {
		name    string
		size    int64
		wantErr errx.Code
	}{
		{name: "at limit", size: maxUploadBody},
		{name: "above limit", size: maxUploadBody + 1, wantErr: errx.CodeWriteOutcomeUnknown},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			counts := make(chan int64, 1)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				count, _ := io.Copy(io.Discard, request.Body)
				counts <- count
				_, _ = io.WriteString(writer, `{"upload":{"token":"opaque"}}`)
			}))
			defer server.Close()

			_, err := newForTest(server.URL, Credential{Token: secretSentinel}).Upload(context.Background(), "large.bin", &sizedReader{remaining: testCase.size})
			if errx.ExitCode(err) != testCase.wantErr {
				t.Fatalf("error=%v code=%d", err, errx.ExitCode(err))
			}
			if received := <-counts; received != maxUploadBody && !(testCase.size < maxUploadBody && received == testCase.size) {
				t.Fatalf("received=%d", received)
			}
		})
	}
}

func TestAcceptedWritesWithoutUsableResultAreOutcomeUnknown(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *Client) error
	}{
		{
			name: "create issue",
			run: func(ctx context.Context, client *Client) error {
				_, err := client.CreateIssue(ctx, map[string]any{"project_id": 1, "subject": "Ship it"})
				return err
			},
		},
		{
			name: "create project",
			run: func(ctx context.Context, client *Client) error {
				_, err := client.CreateProject(ctx, map[string]any{"name": "Ship it", "identifier": "ship-it"})
				return err
			},
		},
		{
			name: "upload",
			run: func(ctx context.Context, client *Client) error {
				_, err := client.Upload(ctx, "notes.txt", strings.NewReader("notes"))
				return err
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(writer, `{}`)
			}))
			defer server.Close()
			if err := testCase.run(context.Background(), newForTest(server.URL, Credential{Token: secretSentinel})); errx.ExitCode(err) != errx.CodeWriteOutcomeUnknown {
				t.Fatalf("error=%v code=%d", err, errx.ExitCode(err))
			}
		})
	}
}

func TestDownloadAttachmentRequiresSameOriginPathAndBoundsResponse(t *testing.T) {
	t.Parallel()
	var downloads int
	transport := mutationRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := ""
		switch request.URL.Path {
		case "/redmine/attachments/1.json":
			body = `{"attachment":{"id":1,"content_url":"https://redmine.test/redmine/attachments/download/1/report.txt"}}`
		case "/redmine/attachments/download/1/report.txt":
			downloads++
			body = "report bytes"
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("nope")), Request: request}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	client, err := New(Config{BaseURL: "https://redmine.test/redmine"}, Credential{Token: secretSentinel}, WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatalf("New() error=%v", err)
	}
	data, err := client.DownloadAttachment(context.Background(), 1)
	if err != nil || !bytes.Equal(data, []byte("report bytes")) || downloads != 1 {
		t.Fatalf("DownloadAttachment()=%q err=%v downloads=%d", data, err, downloads)
	}
}

func TestDownloadAttachmentRejectsUnsafeContentURLBeforeSecondRequest(t *testing.T) {
	t.Parallel()
	unsafe := []string{
		"https://elsewhere.test/attachments/download/1/x",
		"https://redmine.test/attachments/download/1/x",
		"https://redmine.test/redmine/attachments/download/2/x",
		"https://redmine.test/redmine/attachments/download/1/x?token=bad",
	}
	for _, contentURL := range unsafe {
		t.Run(contentURL, func(t *testing.T) {
			calls := 0
			transport := mutationRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				body := `{"attachment":{"id":1,"content_url":` + fmt.Sprintf("%q", contentURL) + `}}`
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
			})
			client, err := New(Config{BaseURL: "https://redmine.test/redmine"}, Credential{Token: secretSentinel}, WithHTTPClient(&http.Client{Transport: transport}))
			if err != nil {
				t.Fatalf("New() error=%v", err)
			}
			_, err = client.DownloadAttachment(context.Background(), 1)
			if errx.ExitCode(err) != errx.CodeInternal || calls != 1 {
				t.Fatalf("error=%v calls=%d", err, calls)
			}
		})
	}
}

func TestDownloadAttachmentRejectsOversizedResponse(t *testing.T) {
	t.Parallel()
	transport := mutationRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/attachments/1.json" {
			body := `{"attachment":{"id":1,"content_url":"https://redmine.test/attachments/download/1/report.txt"}}`
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(&sizedReader{remaining: maxDownloadBody + 1}), Request: request}, nil
	})
	client, err := New(Config{BaseURL: "https://redmine.test"}, Credential{Token: secretSentinel}, WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatalf("New() error=%v", err)
	}
	_, err = client.DownloadAttachment(context.Background(), 1)
	if errx.ExitCode(err) != errx.CodeInternal {
		t.Fatalf("error=%v code=%d", err, errx.ExitCode(err))
	}
}
