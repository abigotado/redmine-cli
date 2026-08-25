package redmine

import "encoding/json"

// NamedID is Redmine's stable id/name reference shape.
type NamedID struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Module is a Redmine enabled-module reference whose ID is textual.
type Module struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CustomField preserves a modeled custom-field value without exposing unknown
// top-level user fields.
type CustomField struct {
	ID       int             `json:"id"`
	Name     string          `json:"name"`
	Multiple bool            `json:"multiple,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
}

// SafeUser is the only current-user type allowed outside the HTTP decoder.
// Redmine may return api_key from /users/current.json; that field is
// intentionally absent.
type SafeUser struct {
	ID           int           `json:"id"`
	Login        string        `json:"login,omitempty"`
	FirstName    string        `json:"firstname,omitempty"`
	LastName     string        `json:"lastname,omitempty"`
	Mail         string        `json:"mail,omitempty"`
	CreatedOn    string        `json:"created_on,omitempty"`
	UpdatedOn    string        `json:"updated_on,omitempty"`
	LastLoginOn  string        `json:"last_login_on,omitempty"`
	Status       int           `json:"status,omitempty"`
	AvatarURL    string        `json:"avatar_url,omitempty"`
	CustomFields []CustomField `json:"custom_fields,omitempty"`
}

type safeUserResponse struct {
	User SafeUser `json:"user"`
}

// Project is the stable modeled subset of a Redmine project.
type Project struct {
	ID                  int           `json:"id"`
	Name                string        `json:"name"`
	Identifier          string        `json:"identifier"`
	Description         string        `json:"description,omitempty"`
	Homepage            string        `json:"homepage,omitempty"`
	Status              int           `json:"status,omitempty"`
	IsPublic            bool          `json:"is_public,omitempty"`
	Parent              *NamedID      `json:"parent,omitempty"`
	DefaultVersion      *NamedID      `json:"default_version,omitempty"`
	DefaultAssignee     *NamedID      `json:"default_assignee,omitempty"`
	CreatedOn           string        `json:"created_on,omitempty"`
	UpdatedOn           string        `json:"updated_on,omitempty"`
	Trackers            []NamedID     `json:"trackers,omitempty"`
	IssueCategories     []NamedID     `json:"issue_categories,omitempty"`
	EnabledModules      []Module      `json:"enabled_modules,omitempty"`
	TimeEntryActivities []NamedID     `json:"time_entry_activities,omitempty"`
	CustomFields        []CustomField `json:"issue_custom_fields,omitempty"`
}

type projectResponse struct {
	Project Project `json:"project"`
}

// ProjectPage is Redmine's offset-paginated project response.
type ProjectPage struct {
	Projects   []Project `json:"projects"`
	TotalCount int       `json:"total_count"`
	Offset     int       `json:"offset"`
	Limit      int       `json:"limit"`
}

// Attachment is a safe modeled attachment reference. The API token is never
// appended to ContentURL.
type Attachment struct {
	ID          int      `json:"id"`
	Filename    string   `json:"filename"`
	Filesize    int64    `json:"filesize,omitempty"`
	ContentType string   `json:"content_type,omitempty"`
	Description string   `json:"description,omitempty"`
	ContentURL  string   `json:"content_url,omitempty"`
	Author      *NamedID `json:"author,omitempty"`
	CreatedOn   string   `json:"created_on,omitempty"`
}

// JournalDetail models a Redmine journal change.
type JournalDetail struct {
	Property string          `json:"property,omitempty"`
	Name     string          `json:"name,omitempty"`
	OldValue json.RawMessage `json:"old_value,omitempty"`
	NewValue json.RawMessage `json:"new_value,omitempty"`
}

// Journal models issue history without unmodeled user fields.
type Journal struct {
	ID           int             `json:"id"`
	User         NamedID         `json:"user"`
	Notes        string          `json:"notes,omitempty"`
	CreatedOn    string          `json:"created_on,omitempty"`
	PrivateNotes bool            `json:"private_notes,omitempty"`
	Details      []JournalDetail `json:"details,omitempty"`
}

// Relation models one issue relation.
type Relation struct {
	ID           int    `json:"id"`
	IssueID      int    `json:"issue_id"`
	IssueToID    int    `json:"issue_to_id"`
	RelationType string `json:"relation_type"`
	Delay        int    `json:"delay,omitempty"`
}

// Issue is the stable modeled subset of a Redmine issue.
type Issue struct {
	ID             int           `json:"id"`
	Project        NamedID       `json:"project"`
	Tracker        NamedID       `json:"tracker"`
	Status         NamedID       `json:"status"`
	Priority       NamedID       `json:"priority"`
	Author         NamedID       `json:"author"`
	AssignedTo     *NamedID      `json:"assigned_to,omitempty"`
	Parent         *NamedID      `json:"parent,omitempty"`
	Subject        string        `json:"subject"`
	Description    string        `json:"description,omitempty"`
	StartDate      string        `json:"start_date,omitempty"`
	DueDate        string        `json:"due_date,omitempty"`
	DoneRatio      int           `json:"done_ratio,omitempty"`
	IsPrivate      bool          `json:"is_private,omitempty"`
	EstimatedHours *float64      `json:"estimated_hours,omitempty"`
	SpentHours     *float64      `json:"spent_hours,omitempty"`
	CreatedOn      string        `json:"created_on,omitempty"`
	UpdatedOn      string        `json:"updated_on,omitempty"`
	ClosedOn       string        `json:"closed_on,omitempty"`
	CustomFields   []CustomField `json:"custom_fields,omitempty"`
	Attachments    []Attachment  `json:"attachments,omitempty"`
	Journals       []Journal     `json:"journals,omitempty"`
	Relations      []Relation    `json:"relations,omitempty"`
	Watchers       []NamedID     `json:"watchers,omitempty"`
	Children       []Issue       `json:"children,omitempty"`
}

type issueResponse struct {
	Issue Issue `json:"issue"`
}

// IssuePage is Redmine's offset-paginated issue response.
type IssuePage struct {
	Issues     []Issue `json:"issues"`
	TotalCount int     `json:"total_count"`
	Offset     int     `json:"offset"`
	Limit      int     `json:"limit"`
}
