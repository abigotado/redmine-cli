package redmine

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/abigotado/redmine-cli/internal/errx"
)

const (
	DefaultLimit = 25
	MaxLimit     = 100
)

var (
	listIssueIncludes = map[string]struct{}{"attachments": {}, "relations": {}}
	getIssueIncludes  = map[string]struct{}{"attachments": {}, "relations": {}, "journals": {}, "watchers": {}, "children": {}}
	projectIncludes   = map[string]struct{}{
		"trackers": {}, "issue_categories": {}, "enabled_modules": {},
		"time_entry_activities": {}, "issue_custom_fields": {},
	}
)

// ProjectListOptions controls project pagination.
type ProjectListOptions struct {
	Offset  int
	Limit   int
	Include []string
}

// Query returns canonical Redmine query values.
func (options ProjectListOptions) Query() (url.Values, error) {
	if err := validatePage(options.Offset, options.Limit); err != nil {
		return nil, err
	}
	query := make(url.Values)
	query.Set("offset", strconv.Itoa(options.Offset))
	query.Set("limit", strconv.Itoa(options.Limit))
	include, err := ValidateProjectIncludes(options.Include)
	if err != nil {
		return nil, err
	}
	if len(include) > 0 {
		query.Set("include", strings.Join(include, ","))
	}
	return query, nil
}

// ValidateProjectIncludes returns a stable project association allowlist.
func ValidateProjectIncludes(values []string) ([]string, error) {
	return validateIncludes(values, projectIncludes, "project")
}

// IssueListOptions controls supported Redmine issue filters.
type IssueListOptions struct {
	Offset       int
	Limit        int
	ProjectID    string
	TrackerID    string
	StatusID     string
	AssignedToID string
	CreatedOn    string
	UpdatedOn    string
	Sort         string
	Include      []string
}

// Query validates and returns canonical Redmine query values.
func (options IssueListOptions) Query() (url.Values, error) {
	if err := validatePage(options.Offset, options.Limit); err != nil {
		return nil, err
	}
	if err := validateNumeric("project ID", options.ProjectID, false); err != nil {
		return nil, err
	}
	if err := validateNumeric("tracker ID", options.TrackerID, false); err != nil {
		return nil, err
	}
	if options.AssignedToID != "" && options.AssignedToID != "me" {
		if err := validateNumeric("assignee ID", options.AssignedToID, true); err != nil {
			return nil, err
		}
	}
	if options.StatusID == "" {
		options.StatusID = "open"
	}
	switch options.StatusID {
	case "open", "closed", "*":
	default:
		if err := validateNumeric("status ID", options.StatusID, true); err != nil {
			return nil, err
		}
	}
	if err := validateFilter("created-on filter", options.CreatedOn); err != nil {
		return nil, err
	}
	if err := validateFilter("updated-on filter", options.UpdatedOn); err != nil {
		return nil, err
	}
	if err := validateSort(options.Sort); err != nil {
		return nil, err
	}
	include, err := ValidateIncludes(options.Include, false)
	if err != nil {
		return nil, err
	}

	query := make(url.Values)
	query.Set("offset", strconv.Itoa(options.Offset))
	query.Set("limit", strconv.Itoa(options.Limit))
	query.Set("status_id", options.StatusID)
	setIf(query, "project_id", options.ProjectID)
	setIf(query, "tracker_id", options.TrackerID)
	setIf(query, "assigned_to_id", options.AssignedToID)
	setIf(query, "created_on", options.CreatedOn)
	setIf(query, "updated_on", options.UpdatedOn)
	setIf(query, "sort", options.Sort)
	if len(include) > 0 {
		query.Set("include", strings.Join(include, ","))
	}
	return query, nil
}

// ValidateIncludes returns a sorted, deduplicated command-specific allowlist.
func ValidateIncludes(values []string, single bool) ([]string, error) {
	allowed := listIssueIncludes
	if single {
		allowed = getIssueIncludes
	}
	return validateIncludes(values, allowed, "issue")
}

func validateIncludes(values []string, allowed map[string]struct{}, kind string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			if _, ok := allowed[name]; !ok {
				return nil, errx.Usage("unsupported %s include %q for this command", kind, name)
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}
	sortStrings(result)
	return result, nil
}

// Myself returns the authenticated user through a secret-omitting DTO.
func (client *Client) Myself(ctx context.Context) (SafeUser, error) {
	var response safeUserResponse
	err := client.get(ctx, request{path: "/users/current.json"}, &response)
	if err == nil && response.User.ID <= 0 {
		err = errx.Internal("Redmine returned an invalid current-user response")
	}
	return response.User, err
}

// Projects returns one bounded project page.
func (client *Client) Projects(ctx context.Context, options ProjectListOptions) (ProjectPage, error) {
	query, err := options.Query()
	if err != nil {
		return ProjectPage{}, err
	}
	var page ProjectPage
	err = client.get(ctx, request{path: "/projects.json", query: query}, &page)
	if err == nil {
		err = validateUpstreamPage(page.Offset, page.Limit, page.TotalCount, len(page.Projects), options)
		if err == nil {
			err = validateProjectValues(page.Projects)
		}
	}
	return page, err
}

// Project returns one project by numeric ID or textual identifier.
func (client *Client) Project(ctx context.Context, idOrIdentifier string, includes []string) (Project, error) {
	if err := validateIdentifier(idOrIdentifier); err != nil {
		return Project{}, err
	}
	var response projectResponse
	include, err := ValidateProjectIncludes(includes)
	if err != nil {
		return Project{}, err
	}
	query := make(url.Values)
	if len(include) > 0 {
		query.Set("include", strings.Join(include, ","))
	}
	err = client.get(ctx, request{path: "/projects/" + url.PathEscape(idOrIdentifier) + ".json", query: query}, &response)
	if err == nil {
		err = validateProjectValues([]Project{response.Project})
	}
	return response.Project, err
}

// Issues returns one bounded issue page.
func (client *Client) Issues(ctx context.Context, options IssueListOptions) (IssuePage, error) {
	query, err := options.Query()
	if err != nil {
		return IssuePage{}, err
	}
	var page IssuePage
	err = client.get(ctx, request{path: "/issues.json", query: query}, &page)
	if err == nil {
		err = validateUpstreamPage(page.Offset, page.Limit, page.TotalCount, len(page.Issues), ProjectListOptions{Offset: options.Offset, Limit: options.Limit})
		if err == nil {
			err = validateIssueValues(page.Issues)
		}
	}
	return page, err
}

// Issue returns one issue by numeric ID with allowlisted associations.
func (client *Client) Issue(ctx context.Context, id int, includes []string) (Issue, error) {
	if id <= 0 {
		return Issue{}, errx.Usage("issue ID must be a positive integer")
	}
	include, err := ValidateIncludes(includes, true)
	if err != nil {
		return Issue{}, err
	}
	query := make(url.Values)
	if len(include) > 0 {
		query.Set("include", strings.Join(include, ","))
	}
	var response issueResponse
	err = client.get(ctx, request{path: "/issues/" + strconv.Itoa(id) + ".json", query: query}, &response)
	if err == nil {
		err = validateIssueValues([]Issue{response.Issue})
	}
	return response.Issue, err
}

func validateProjectValues(values []Project) error {
	for _, value := range values {
		if value.ID <= 0 || value.Name == "" || value.Identifier == "" {
			return errx.Internal("Redmine returned an invalid project response")
		}
	}
	return nil
}

func validateIssueValues(values []Issue) error {
	for _, value := range values {
		if value.ID <= 0 || value.Subject == "" || value.Project.ID <= 0 || value.Status.ID <= 0 {
			return errx.Internal("Redmine returned an invalid issue response")
		}
	}
	return nil
}

func validatePage(offset, limit int) error {
	if offset < 0 {
		return errx.Usage("pagination offset cannot be negative")
	}
	if limit < 1 || limit > MaxLimit {
		return errx.Usage("--limit must be between 1 and %d", MaxLimit)
	}
	return nil
}

func validateUpstreamPage(offset, limit, total, count int, requested ProjectListOptions) error {
	if offset != requested.Offset || limit <= 0 || limit > MaxLimit || total < 0 || count > limit || offset+count > total {
		return errx.Internal("Redmine returned inconsistent pagination metadata")
	}
	if total > offset && count == 0 {
		return errx.Internal("Redmine returned a non-advancing collection page")
	}
	return nil
}

func validateNumeric(label, value string, required bool) error {
	if value == "" {
		if required {
			return errx.Usage("%s is required", label)
		}
		return nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number <= 0 || strconv.Itoa(number) != value {
		return errx.Usage("%s must be a positive base-10 integer", label)
	}
	return nil
}

func validateIdentifier(value string) error {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value || strings.ContainsAny(value, "/\\?#\x00\r\n\t ") {
		return errx.Usage("project ID or identifier is invalid")
	}
	return nil
}

func validateFilter(label, value string) error {
	if len(value) > 256 || strings.ContainsAny(value, "\x00\r\n\t") {
		return errx.Usage("%s is invalid", label)
	}
	return nil
}

func validateSort(value string) error {
	if value == "" {
		return nil
	}
	allowed := map[string]struct{}{
		"id": {}, "project": {}, "tracker": {}, "status": {}, "priority": {},
		"subject": {}, "author": {}, "assigned_to": {}, "created_on": {}, "updated_on": {},
	}
	for _, part := range strings.Split(value, ",") {
		bits := strings.Split(part, ":")
		if len(bits) > 2 {
			return errx.Usage("invalid --sort value")
		}
		if _, ok := allowed[bits[0]]; !ok {
			return errx.Usage("unsupported --sort field %q", bits[0])
		}
		if len(bits) == 2 && bits[1] != "asc" && bits[1] != "desc" {
			return errx.Usage("sort direction must be asc or desc")
		}
	}
	return nil
}

func setIf(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// Fingerprint binds a cursor to profile, base URL, resource, filters, and sort.
func Fingerprint(profileName, baseURL, resource string, query url.Values) string {
	canonical := make(url.Values, len(query))
	for key, values := range query {
		if key == "offset" || key == "limit" {
			continue
		}
		canonical[key] = append([]string(nil), values...)
	}
	sum := sha256.Sum256([]byte(profileName + "\n" + baseURL + "\n" + resource + "\n" + canonical.Encode()))
	return hex.EncodeToString(sum[:])
}

type cursorPayload struct {
	V           int    `json:"v"`
	Resource    string `json:"resource"`
	Offset      int    `json:"offset"`
	Fingerprint string `json:"fingerprint"`
}

// EncodeCursor creates an opaque request-bound cursor.
func EncodeCursor(resource string, offset int, fingerprint string) (string, error) {
	if resource == "" || offset <= 0 || len(fingerprint) != sha256.Size*2 {
		return "", errx.Internal("cannot encode invalid pagination cursor")
	}
	raw, err := json.Marshal(cursorPayload{V: 1, Resource: resource, Offset: offset, Fingerprint: fingerprint})
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// DecodeCursor validates an opaque cursor for the current request.
func DecodeCursor(value, resource, fingerprint string) (int, error) {
	if value == "" {
		return 0, nil
	}
	if len(value) > 2048 {
		return 0, errx.Usage("pagination cursor is invalid")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, errx.Usage("pagination cursor is invalid")
	}
	var payload cursorPayload
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil || payload.V != 1 || payload.Resource != resource || payload.Offset <= 0 || payload.Fingerprint != fingerprint {
		return 0, errx.Usage("pagination cursor does not match this profile and query")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return 0, errx.Usage("pagination cursor is invalid")
	}
	return payload.Offset, nil
}
