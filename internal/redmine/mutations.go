package redmine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/abigotado/redmine-cli/internal/errx"
)

const (
	maxMutationBody = 128 << 10
	maxUploadBody   = 50 << 20
	maxDownloadBody = 100 << 20
)

// UploadToken is intentionally opaque and cannot be serialized or formatted.
type UploadToken struct{ value string }

func (UploadToken) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, "<redacted>") }

type uploadLimitReader struct {
	reader    io.Reader
	remaining int64
}

func (reader *uploadLimitReader) Read(buffer []byte) (int, error) {
	if reader.remaining == 0 {
		var extra [1]byte
		n, err := reader.reader.Read(extra[:])
		if n > 0 {
			return 0, errx.Usage("file exceeds the 50 MiB safety limit")
		}
		return 0, err
	}
	if int64(len(buffer)) > reader.remaining {
		buffer = buffer[:reader.remaining]
	}
	n, err := reader.reader.Read(buffer)
	reader.remaining -= int64(n)
	return n, err
}

// IssueUploads converts opaque tokens into the exact Redmine issue payload.
func IssueUploads(tokens []UploadToken, names []string) []map[string]string {
	result := make([]map[string]string, 0, len(tokens))
	for index, token := range tokens {
		result = append(result, map[string]string{"token": token.value, "filename": names[index]})
	}
	return result
}

func (client *Client) mutate(ctx context.Context, method, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return errx.Internal("encode Redmine request")
	}
	if len(body) > maxMutationBody {
		return errx.Usage("request body exceeds the %d-byte safety limit", maxMutationBody)
	}
	response, err := client.sendMethod(ctx, method, path, nil, bytes.NewReader(body), "application/json")
	if err != nil {
		return errx.WriteOutcomeUnknown("WRITE_OUTCOME_UNKNOWN", "Redmine write outcome is unknown")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return client.writeStatus(response)
	}
	if out == nil {
		return nil
	}
	limited := io.LimitReader(response.Body, maxResponseBody+1)
	data, err := io.ReadAll(limited)
	if err != nil || len(data) > maxResponseBody {
		return errx.WriteOutcomeUnknown("WRITE_APPLIED_RESULT_UNAVAILABLE", "Redmine accepted the write but its result is unavailable")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return errx.WriteOutcomeUnknown("WRITE_APPLIED_RESULT_UNAVAILABLE", "Redmine accepted the write but did not return its result")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return errx.WriteOutcomeUnknown("WRITE_APPLIED_RESULT_UNAVAILABLE", "Redmine accepted the write but returned an invalid result")
	}
	return nil
}

func (client *Client) sendMethod(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string) (*http.Response, error) {
	fullURL := client.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, errors.New("invalid Redmine request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Redmine-API-Key", client.token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client.log.Debug("Redmine request", "method", method)
	return client.http.Do(req)
}

func (client *Client) writeStatus(response *http.Response) error {
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return errx.Auth("AUTHENTICATION_FAILED", "Redmine rejected the API token")
	case http.StatusForbidden:
		return errx.Permission("PERMISSION_DENIED", "the Redmine account cannot change this resource")
	case http.StatusNotFound:
		return errx.NotFound(resourceKind(response.Request.URL.Path), "requested", nil)
	case http.StatusConflict:
		return errx.Conflict("CONFLICT", "Redmine reported a stale resource conflict")
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return errx.Usage("Redmine rejected the request parameters")
	case http.StatusTooManyRequests:
		return errx.Retryable("RATE_LIMITED", parseRetryAfter(response.Header.Get("Retry-After"), client.now()), "Redmine rate limit reached")
	default:
		if response.StatusCode >= 500 {
			return errx.WriteOutcomeUnknown("WRITE_OUTCOME_UNKNOWN", "Redmine write outcome is unknown")
		}
		return errx.Internal("Redmine returned unexpected HTTP status %d", response.StatusCode)
	}
}

// CreateIssue creates an issue from a validated attribute map.
func (client *Client) CreateIssue(ctx context.Context, attributes map[string]any) (Issue, error) {
	var response issueResponse
	err := client.mutate(ctx, http.MethodPost, "/issues.json", map[string]any{"issue": attributes}, &response)
	if err == nil {
		err = validateIssueValues([]Issue{response.Issue})
		if err != nil {
			err = errx.WriteOutcomeUnknown("WRITE_APPLIED_RESULT_UNAVAILABLE", "Redmine accepted the write but its result is unavailable")
		}
	}
	return response.Issue, err
}

// UpdateIssue updates an issue from a validated attribute map.
func (client *Client) UpdateIssue(ctx context.Context, id int, attributes map[string]any) (Issue, error) {
	if id <= 0 {
		return Issue{}, errx.Usage("issue ID must be a positive integer")
	}
	if err := client.mutate(ctx, http.MethodPut, "/issues/"+strconv.Itoa(id)+".json", map[string]any{"issue": attributes}, nil); err != nil {
		return Issue{}, err
	}
	issue, err := client.Issue(ctx, id, nil)
	if err != nil {
		return Issue{}, errx.WriteOutcomeUnknown("WRITE_APPLIED_RESULT_UNAVAILABLE", "Redmine accepted the write but its result is unavailable")
	}
	return issue, nil
}

// CreateProject creates a project from a validated attribute map.
func (client *Client) CreateProject(ctx context.Context, attributes map[string]any) (Project, error) {
	var response projectResponse
	err := client.mutate(ctx, http.MethodPost, "/projects.json", map[string]any{"project": attributes}, &response)
	if err == nil {
		err = validateProjectValues([]Project{response.Project})
		if err != nil {
			err = errx.WriteOutcomeUnknown("WRITE_APPLIED_RESULT_UNAVAILABLE", "Redmine accepted the write but its result is unavailable")
		}
	}
	return response.Project, err
}

// UpdateProject updates a project identified by its immutable numeric ID.
func (client *Client) UpdateProject(ctx context.Context, id int, attributes map[string]any) (Project, error) {
	if id <= 0 {
		return Project{}, errx.Usage("project ID must be a positive integer")
	}
	if err := client.mutate(ctx, http.MethodPut, "/projects/"+strconv.Itoa(id)+".json", map[string]any{"project": attributes}, nil); err != nil {
		return Project{}, err
	}
	project, err := client.Project(ctx, strconv.Itoa(id), nil)
	if err != nil {
		return Project{}, errx.WriteOutcomeUnknown("WRITE_APPLIED_RESULT_UNAVAILABLE", "Redmine accepted the write but its result is unavailable")
	}
	return project, nil
}

// Files lists the project files visible to the selected profile.
func (client *Client) Files(ctx context.Context, project string) ([]File, error) {
	if err := validateIdentifier(project); err != nil {
		return nil, err
	}
	var page filePage
	err := client.get(ctx, request{path: "/projects/" + url.PathEscape(project) + "/files.json"}, &page)
	return page.Files, err
}

// Upload sends one bounded file and returns an opaque upload token.
func (client *Client) Upload(ctx context.Context, filename string, body io.Reader) (UploadToken, error) {
	if filename == "" || len(filename) > 255 {
		return UploadToken{}, errx.Usage("filename is invalid")
	}
	if body == nil {
		return UploadToken{}, errx.Usage("file content is required")
	}
	limited := &uploadLimitReader{reader: body, remaining: maxUploadBody}
	response, err := client.sendMethod(ctx, http.MethodPost, "/uploads.json", url.Values{"filename": {filename}}, limited, "application/octet-stream")
	if err != nil {
		return UploadToken{}, errx.WriteOutcomeUnknown("WRITE_OUTCOME_UNKNOWN", "Redmine upload outcome is unknown")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return UploadToken{}, client.writeStatus(response)
	}
	var result struct {
		Upload struct {
			Token string `json:"token"`
		} `json:"upload"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBody)).Decode(&result); err != nil || result.Upload.Token == "" {
		return UploadToken{}, errx.WriteOutcomeUnknown("WRITE_APPLIED_RESULT_UNAVAILABLE", "Redmine accepted the upload but did not return a usable result")
	}
	return UploadToken{value: result.Upload.Token}, nil
}

// AddFile adds one uploaded file to a project and returns a token-free receipt.
func (client *Client) AddFile(ctx context.Context, projectID int, token UploadToken, filename, description string, versionID int) error {
	file := map[string]any{"token": token.value, "filename": filename}
	if description != "" {
		file["description"] = description
	}
	if versionID > 0 {
		file["version_id"] = versionID
	}
	return client.mutate(ctx, http.MethodPost, "/projects/"+strconv.Itoa(projectID)+"/files.json", map[string]any{"file": file}, nil)
}

// DownloadAttachment obtains bounded bytes only from a same-origin attachment URL.
func (client *Client) DownloadAttachment(ctx context.Context, id int) ([]byte, error) {
	if id <= 0 {
		return nil, errx.Usage("attachment ID must be a positive integer")
	}
	var response struct {
		Attachment Attachment `json:"attachment"`
	}
	if err := client.get(ctx, request{path: "/attachments/" + strconv.Itoa(id) + ".json"}, &response); err != nil {
		return nil, err
	}
	u, err := url.Parse(response.Attachment.ContentURL)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "https" {
		return nil, errx.Internal("Redmine returned an unsafe attachment URL")
	}
	base, _ := url.Parse(client.baseURL)
	basePath := strings.TrimSuffix(base.EscapedPath(), "/")
	prefix := basePath + "/attachments/download/" + strconv.Itoa(id)
	escaped := u.EscapedPath()
	if u.Host != base.Host || (escaped != prefix && !strings.HasPrefix(escaped, prefix+"/")) {
		return nil, errx.Internal("Redmine returned an unsafe attachment URL")
	}
	relative := strings.TrimPrefix(escaped, basePath)
	responseHTTP, err := client.sendMethod(ctx, http.MethodGet, relative, nil, nil, "")
	if err != nil {
		return nil, errx.Retryable("NETWORK", 0, "could not download attachment")
	}
	defer responseHTTP.Body.Close()
	if responseHTTP.StatusCode < 200 || responseHTTP.StatusCode >= 300 {
		_, _, statusErr := client.translateStatus(responseHTTP, request{path: relative})
		return nil, statusErr
	}
	data, err := io.ReadAll(io.LimitReader(responseHTTP.Body, maxDownloadBody+1))
	if err != nil || len(data) > maxDownloadBody {
		return nil, errx.Internal("attachment exceeds the 100 MiB safety limit")
	}
	return data, nil
}
