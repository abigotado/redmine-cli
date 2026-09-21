package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abigotado/redmine-cli/internal/auth"
	"github.com/abigotado/redmine-cli/internal/errx"
	"github.com/abigotado/redmine-cli/internal/profile"
	"github.com/abigotado/redmine-cli/internal/redmine"
)

type recordingMutationClient struct {
	redmineReader
	issueAttrs    map[string]any
	projectAttrs  map[string]any
	projectID     int
	uploadName    string
	uploadBody    string
	addedName     string
	addedDesc     string
	addedVersion  int
	issueIncludes []string
}

func (client *recordingMutationClient) CreateIssue(_ context.Context, attributes map[string]any, _ []redmine.IssueUpload) (redmine.Issue, error) {
	client.issueAttrs = attributes
	return testIssue(), nil
}

func (client *recordingMutationClient) UpdateIssue(_ context.Context, _ int, attributes map[string]any, _ []redmine.IssueUpload) (redmine.Issue, error) {
	client.issueAttrs = attributes
	return testIssue(), nil
}

func (client *recordingMutationClient) Issue(_ context.Context, _ int, includes []string) (redmine.Issue, error) {
	client.issueIncludes = append([]string(nil), includes...)
	issue := testIssue()
	issue.Attachments = []redmine.Attachment{{ID: 1, Filename: "report.txt"}}
	return issue, nil
}

func (client *recordingMutationClient) CreateProject(_ context.Context, attributes map[string]any) (redmine.Project, error) {
	client.projectAttrs = attributes
	return testProject(), nil
}

func (client *recordingMutationClient) UpdateProject(_ context.Context, id int, attributes map[string]any) (redmine.Project, error) {
	client.projectID = id
	client.projectAttrs = attributes
	return testProject(), nil
}

func (client *recordingMutationClient) Project(context.Context, string, []string) (redmine.Project, error) {
	return testProject(), nil
}

func (client *recordingMutationClient) Upload(_ context.Context, filename string, _ int64, body io.Reader) (redmine.UploadToken, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return redmine.UploadToken{}, err
	}
	client.uploadName = filename
	client.uploadBody = string(data)
	return redmine.UploadToken{}, nil
}

func (client *recordingMutationClient) AddFile(_ context.Context, _ int, _ redmine.UploadToken, filename, description string, versionID int) error {
	client.addedName = filename
	client.addedDesc = description
	client.addedVersion = versionID
	return nil
}

func (client *recordingMutationClient) Files(context.Context, string) ([]redmine.File, error) {
	return []redmine.File{{ID: 1, Filename: "report.txt", Filesize: 7}}, nil
}

func (client *recordingMutationClient) DownloadAttachment(context.Context, int) ([]byte, error) {
	return []byte("report bytes"), nil
}

func testIssue() redmine.Issue {
	return redmine.Issue{ID: 7, Subject: "Ship it", Project: redmine.NamedID{ID: 1, Name: "P"}, Status: redmine.NamedID{ID: 1, Name: "New"}}
}

func testProject() redmine.Project {
	return redmine.Project{ID: 3, Name: "Ship it", Identifier: "ship-it"}
}

func TestMutationDryRunAvoidsCredentialsNetworkAndFileContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte(cliSecretSentinel), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	registry := &memoryRegistry{values: map[string]profile.Profile{}}
	stdout := &bytes.Buffer{}
	app := &App{
		registry: registry, store: panicStore{}, stdin: panicReader{}, stdout: stdout, stderr: &bytes.Buffer{},
		newRedmine: func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) {
			panic("network must not be contacted")
		},
	}
	code := app.Run(context.Background(), app.NewRootCommand(), []string{
		"issues", "create", "--profile", "preview", "--project-id", "1", "--subject", "Ship it", "--attach", path, "--dry-run",
	})
	if code != errx.CodeOK {
		t.Fatalf("code=%d stdout=%s", code, stdout.String())
	}
	if strings.Contains(stdout.String(), cliSecretSentinel) || !strings.Contains(stdout.String(), `"method":"POST"`) {
		t.Fatalf("dry-run output was not a sanitized preview: %s", stdout.String())
	}
}

func TestMutationDryRunIgnoresResultFieldSelection(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "issue create", args: []string{"issues", "create", "--project-id", "1", "--subject", "Ship it", "--fields", "attachments"}},
		{name: "issue update", args: []string{"issues", "update", "7", "--subject", "Ship it", "--fields", "attachments"}},
		{name: "project create", args: []string{"projects", "create", "--name", "Ship it", "--identifier", "ship-it", "--fields", "trackers"}},
		{name: "project update", args: []string{"projects", "update", "ship-it", "--name", "Ship it", "--fields", "trackers"}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			app := &App{registry: &memoryRegistry{values: map[string]profile.Profile{}}, store: panicStore{}, stdin: panicReader{}, stdout: stdout, stderr: &bytes.Buffer{}, newRedmine: func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) {
				panic("network must not be contacted")
			}}
			args := append([]string{"--profile", "preview"}, testCase.args...)
			args = append(args, "--dry-run")
			code := app.Run(context.Background(), app.NewRootCommand(), args)
			if code != errx.CodeOK {
				t.Fatalf("code=%d stdout=%s", code, stdout.String())
			}
			if !strings.Contains(stdout.String(), `"method"`) {
				t.Fatalf("preview was not rendered: %s", stdout.String())
			}
		})
	}
}

func TestSelectedRegularFileRejectsPathSwap(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "selected.txt")
	secret := filepath.Join(directory, "secret.txt")
	if err := os.WriteFile(path, []byte("selected"), 0o600); err != nil {
		t.Fatalf("write selected file: %v", err)
	}
	if err := os.WriteFile(secret, []byte(cliSecretSentinel), 0o600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove selected file: %v", err)
	}
	if err := os.Symlink(secret, path); err != nil {
		t.Fatalf("swap selected file for symlink: %v", err)
	}
	type result struct {
		file *os.File
		err  error
	}
	results := make(chan result, 1)
	go func() {
		file, openErr := openSelectedRegularFile(path, "attachment", info)
		results <- result{file: file, err: openErr}
	}()
	var file *os.File
	select {
	case outcome := <-results:
		file, err = outcome.file, outcome.err
	case <-time.After(time.Second):
		t.Fatal("swapped symlink blocked before identity validation")
	}
	if file != nil {
		_ = file.Close()
		t.Fatal("swapped path opened")
	}
	if errx.ExitCode(err) != errx.CodeUsage {
		t.Fatalf("error=%v", err)
	}
}

func TestAttachmentPreviewsRespectCountAndSizeBoundaries(t *testing.T) {
	file := func(t *testing.T, size int64, name string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		handle, err := os.Create(path)
		if err != nil {
			t.Fatalf("create file: %v", err)
		}
		if err := handle.Truncate(size); err != nil {
			_ = handle.Close()
			t.Fatalf("truncate file: %v", err)
		}
		if err := handle.Close(); err != nil {
			t.Fatalf("close file: %v", err)
		}
		return path
	}
	max := int64(50 << 20)
	tests := []struct {
		name  string
		paths []string
		ok    bool
	}{
		{name: "ten attachments", paths: func() []string {
			paths := make([]string, 10)
			for index := range paths {
				paths[index] = file(t, 1, "file")
			}
			return paths
		}(), ok: true},
		{name: "eleven attachments", paths: func() []string {
			paths := make([]string, 11)
			for index := range paths {
				paths[index] = file(t, 1, "file")
			}
			return paths
		}()},
		{name: "one attachment at limit", paths: []string{file(t, max, "at-limit")}, ok: true},
		{name: "one attachment above limit", paths: []string{file(t, max+1, "above-limit")}},
		{name: "total at limit", paths: []string{file(t, max, "first"), file(t, max, "second")}, ok: true},
		{name: "total above limit", paths: []string{file(t, max, "first"), file(t, max, "second"), file(t, 1, "extra")}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := attachmentPreviews(testCase.paths)
			if (err == nil) != testCase.ok {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestFileAddDryRunIncludesEveryAppliedAttribute(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(path, []byte("report"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	stdout := &bytes.Buffer{}
	app := &App{registry: &memoryRegistry{values: map[string]profile.Profile{}}, store: panicStore{}, stdin: panicReader{}, stdout: stdout, stderr: &bytes.Buffer{}, newRedmine: func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) { panic("network") }}
	code := app.Run(context.Background(), app.NewRootCommand(), []string{"files", "add", "ship-it", "--profile", "preview", "--path", path, "--description", "release notes", "--version-id", "6", "--dry-run"})
	if code != errx.CodeOK {
		t.Fatalf("code=%d stdout=%s", code, stdout.String())
	}
	var response struct {
		Data writePreview `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if response.Data.Attributes["project"] != "ship-it" || response.Data.Attributes["description"] != "release notes" || response.Data.Attributes["version_id"] != float64(6) {
		t.Fatalf("preview attributes=%#v", response.Data.Attributes)
	}
}

func TestMutationsRequireConfirmationBeforeCredentialAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(path, []byte("report"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	tests := []struct {
		name string
		args []string
	}{
		{name: "issue create", args: []string{"issues", "create", "--profile", "work", "--project-id", "1", "--subject", "Ship it"}},
		{name: "issue update", args: []string{"issues", "update", "7", "--profile", "work", "--status-id", "2"}},
		{name: "project create", args: []string{"projects", "create", "--profile", "work", "--name", "Ship it", "--identifier", "ship-it"}},
		{name: "project update", args: []string{"projects", "update", "ship-it", "--profile", "work", "--description", "changed"}},
		{name: "file add", args: []string{"files", "add", "ship-it", "--profile", "work", "--path", path}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			app, _, store, stdout, stderr := newTestApp()
			factoryCalls := 0
			app.newRedmine = func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) {
				factoryCalls++
				return &recordingMutationClient{}, nil
			}
			code := app.Run(context.Background(), app.NewRootCommand(), testCase.args)
			if code != errx.CodeConfirm {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if store.loads != 0 || factoryCalls != 0 {
				t.Fatalf("credential loads=%d client factories=%d", store.loads, factoryCalls)
			}
		})
	}
}

func TestMutationCommandsForwardOnlyRequestedAttributesAndBoundedFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(path, []byte("report"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, *recordingMutationClient)
	}{
		{
			name: "issue update",
			args: []string{"issues", "update", "7", "--profile", "work", "--status-id", "2", "--assigned-to-id", "none", "--yes"},
			check: func(t *testing.T, client *recordingMutationClient) {
				if len(client.issueAttrs) != 2 || client.issueAttrs["status_id"] != 2 || client.issueAttrs["assigned_to_id"] != nil {
					t.Fatalf("issue attributes=%#v", client.issueAttrs)
				}
			},
		},
		{
			name: "project create",
			args: []string{"projects", "create", "--profile", "work", "--name", "Ship it", "--identifier", "ship-it", "--public", "true", "--yes"},
			check: func(t *testing.T, client *recordingMutationClient) {
				if len(client.projectAttrs) != 3 || client.projectAttrs["name"] != "Ship it" || client.projectAttrs["is_public"] != true {
					t.Fatalf("project attributes=%#v", client.projectAttrs)
				}
			},
		},
		{
			name: "project update by identifier resolves immutable ID",
			args: []string{"projects", "update", "ship-it", "--profile", "work", "--description=", "--yes"},
			check: func(t *testing.T, client *recordingMutationClient) {
				if client.projectID != 3 || len(client.projectAttrs) != 1 || client.projectAttrs["description"] != "" {
					t.Fatalf("project ID=%d attributes=%#v", client.projectID, client.projectAttrs)
				}
			},
		},
		{
			name: "file add",
			args: []string{"files", "add", "ship-it", "--profile", "work", "--path", path, "--description", "release notes", "--version-id", "6", "--yes"},
			check: func(t *testing.T, client *recordingMutationClient) {
				if client.uploadName != "report.txt" || client.uploadBody != "report" || client.addedName != "report.txt" || client.addedDesc != "release notes" || client.addedVersion != 6 {
					t.Fatalf("upload=%q/%q add=%q/%q/%d", client.uploadName, client.uploadBody, client.addedName, client.addedDesc, client.addedVersion)
				}
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			app, _, _, stdout, stderr := newTestApp()
			client := &recordingMutationClient{}
			app.newRedmine = func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) { return client, nil }
			code := app.Run(context.Background(), app.NewRootCommand(), testCase.args)
			if code != errx.CodeOK {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			testCase.check(t, client)
		})
	}
}

func TestMutationLocalValidationUsesUsageEnvelope(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "auth login URL", args: []string{"auth", "login", "--profile", "work"}},
		{name: "issue project", args: []string{"issues", "create", "--profile", "work", "--subject", "Ship it"}},
		{name: "issue subject", args: []string{"issues", "create", "--profile", "work", "--project-id", "1"}},
		{name: "project name", args: []string{"projects", "create", "--profile", "work", "--identifier", "ship-it"}},
		{name: "project identifier", args: []string{"projects", "create", "--profile", "work", "--name", "Ship it"}},
		{name: "file path", args: []string{"files", "add", "ship-it", "--profile", "work"}},
		{name: "immutable project identifier", args: []string{"projects", "update", "ship-it", "--profile", "work", "--identifier", "other", "--yes"}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			app, _, store, stdout, stderr := newTestApp()
			code := app.Run(context.Background(), app.NewRootCommand(), testCase.args)
			if code != errx.CodeUsage || store.loads != 0 || !strings.Contains(stdout.String(), `"code":"USAGE"`) {
				t.Fatalf("code=%d loads=%d stdout=%s stderr=%s", code, store.loads, stdout.String(), stderr.String())
			}
		})
	}
}

func TestIssueAttachmentKeepsCommaInFilenameAndReadsRequestedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a,b.txt")
	if err := os.WriteFile(path, []byte("report"), 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	app, _, _, stdout, stderr := newTestApp()
	client := &recordingMutationClient{}
	app.newRedmine = func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) { return client, nil }
	code := app.Run(context.Background(), app.NewRootCommand(), []string{"issues", "create", "--profile", "work", "--project-id", "1", "--subject", "Ship it", "--attach", path, "--fields", "attachments", "--yes"})
	if code != errx.CodeOK || client.uploadName != "a,b.txt" || len(client.issueIncludes) != 1 || client.issueIncludes[0] != "attachments" || !strings.Contains(stdout.String(), "report.txt") {
		t.Fatalf("code=%d upload=%q includes=%v stdout=%s stderr=%s", code, client.uploadName, client.issueIncludes, stdout.String(), stderr.String())
	}
}

func TestFilesBoundariesValidateBeforeCredentialAccessAndDownloadRawOnly(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code errx.Code
	}{
		{name: "list invalid field", args: []string{"files", "list", "ship-it", "--profile", "work", "--fields", "nope"}, code: errx.CodeUsage},
		{name: "download requires raw", args: []string{"files", "download", "1", "--profile", "work"}, code: errx.CodeUsage},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			app, _, store, stdout, stderr := newTestApp()
			calls := 0
			app.newRedmine = func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) {
				calls++
				return &recordingMutationClient{}, nil
			}
			code := app.Run(context.Background(), app.NewRootCommand(), testCase.args)
			if code != testCase.code || store.loads != 0 || calls != 0 {
				t.Fatalf("code=%d loads=%d calls=%d stdout=%s stderr=%s", code, store.loads, calls, stdout.String(), stderr.String())
			}
		})
	}

	app, _, _, stdout, stderr := newTestApp()
	client := &recordingMutationClient{}
	app.newRedmine = func(profile.Profile, auth.Credential, *slog.Logger) (redmineReader, error) { return client, nil }
	code := app.Run(context.Background(), app.NewRootCommand(), []string{"files", "download", "1", "--profile", "work", "--output", "raw"})
	if code != errx.CodeOK || stdout.String() != "report bytes" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%s", code, stdout.String(), stderr.String())
	}
}
