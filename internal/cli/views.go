package cli

import (
	"fmt"
	"strings"

	"github.com/abigotado/redmine-cli/internal/output"
	"github.com/abigotado/redmine-cli/internal/profile"
	"github.com/abigotado/redmine-cli/internal/redmine"
)

type versionView struct {
	Version    string `json:"version"`
	Commit     string `json:"commit,omitempty"`
	CommitTime string `json:"commit_time,omitempty"`
	Go         string `json:"go"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
}

type profileView struct{ profile.Profile }

func (view profileView) Fields() []output.Field {
	return []output.Field{
		{Name: "name", Value: view.Name, Raw: view.Name},
		{Name: "base_url", Value: view.BaseURL, Raw: view.BaseURL},
	}
}

type userView struct{ redmine.SafeUser }

func (view userView) Fields() []output.Field {
	name := strings.TrimSpace(view.FirstName + " " + view.LastName)
	return []output.Field{
		{Name: "id", Value: fmt.Sprint(view.ID), Raw: view.ID},
		{Name: "login", Value: view.Login, Raw: view.Login},
		{Name: "name", Value: name, Raw: name},
		{Name: "firstname", Value: view.FirstName, Raw: view.FirstName, OnRequest: true},
		{Name: "lastname", Value: view.LastName, Raw: view.LastName, OnRequest: true},
		{Name: "mail", Value: view.Mail, Raw: view.Mail, OnRequest: true},
		{Name: "created_on", Value: view.CreatedOn, Raw: view.CreatedOn, OnRequest: true},
		{Name: "updated_on", Value: view.UpdatedOn, Raw: view.UpdatedOn, OnRequest: true},
		{Name: "last_login_on", Value: view.LastLoginOn, Raw: view.LastLoginOn, OnRequest: true},
		{Name: "status", Value: fmt.Sprint(view.Status), Raw: view.Status, OnRequest: true},
		{Name: "avatar_url", Value: view.AvatarURL, Raw: view.AvatarURL, OnRequest: true},
		{Name: "custom_fields", Raw: view.CustomFields, OnRequest: true},
	}
}

type projectView struct{ redmine.Project }

func (view projectView) Fields() []output.Field {
	return []output.Field{
		{Name: "id", Value: fmt.Sprint(view.ID), Raw: view.ID},
		{Name: "identifier", Value: view.Identifier, Raw: view.Identifier},
		{Name: "name", Value: view.Name, Raw: view.Name},
		{Name: "status", Value: fmt.Sprint(view.Status), Raw: view.Status},
		{Name: "description", Value: view.Description, Raw: view.Description, OnRequest: true},
		{Name: "homepage", Value: view.Homepage, Raw: view.Homepage, OnRequest: true},
		{Name: "is_public", Value: fmt.Sprint(view.IsPublic), Raw: view.IsPublic, OnRequest: true},
		{Name: "parent", Value: namedValue(view.Parent), Raw: view.Parent, OnRequest: true},
		{Name: "default_version", Value: namedValue(view.DefaultVersion), Raw: view.DefaultVersion, OnRequest: true},
		{Name: "default_assignee", Value: namedValue(view.DefaultAssignee), Raw: view.DefaultAssignee, OnRequest: true},
		{Name: "created_on", Value: view.CreatedOn, Raw: view.CreatedOn, OnRequest: true},
		{Name: "updated_on", Value: view.UpdatedOn, Raw: view.UpdatedOn, OnRequest: true},
		{Name: "trackers", Raw: view.Trackers, OnRequest: true},
		{Name: "issue_categories", Raw: view.IssueCategories, OnRequest: true},
		{Name: "enabled_modules", Raw: view.EnabledModules, OnRequest: true},
		{Name: "time_entry_activities", Raw: view.TimeEntryActivities, OnRequest: true},
		{Name: "issue_custom_fields", Raw: view.CustomFields, OnRequest: true},
	}
}

type issueView struct{ redmine.Issue }

func (view issueView) Fields() []output.Field {
	return []output.Field{
		{Name: "id", Value: fmt.Sprint(view.ID), Raw: view.ID},
		{Name: "subject", Value: view.Subject, Raw: view.Subject},
		{Name: "project", Value: view.Project.Name, Raw: view.Project},
		{Name: "status", Value: view.Status.Name, Raw: view.Status},
		{Name: "assigned_to", Value: namedValue(view.AssignedTo), Raw: view.AssignedTo},
		{Name: "updated_on", Value: view.UpdatedOn, Raw: view.UpdatedOn},
		{Name: "tracker", Value: view.Tracker.Name, Raw: view.Tracker, OnRequest: true},
		{Name: "priority", Value: view.Priority.Name, Raw: view.Priority, OnRequest: true},
		{Name: "author", Value: view.Author.Name, Raw: view.Author, OnRequest: true},
		{Name: "parent", Value: namedValue(view.Parent), Raw: view.Parent, OnRequest: true},
		{Name: "description", Value: view.Description, Raw: view.Description, OnRequest: true},
		{Name: "start_date", Value: view.StartDate, Raw: view.StartDate, OnRequest: true},
		{Name: "due_date", Value: view.DueDate, Raw: view.DueDate, OnRequest: true},
		{Name: "done_ratio", Value: fmt.Sprint(view.DoneRatio), Raw: view.DoneRatio, OnRequest: true},
		{Name: "is_private", Value: fmt.Sprint(view.IsPrivate), Raw: view.IsPrivate, OnRequest: true},
		{Name: "estimated_hours", Raw: view.EstimatedHours, OnRequest: true},
		{Name: "spent_hours", Raw: view.SpentHours, OnRequest: true},
		{Name: "created_on", Value: view.CreatedOn, Raw: view.CreatedOn, OnRequest: true},
		{Name: "closed_on", Value: view.ClosedOn, Raw: view.ClosedOn, OnRequest: true},
		{Name: "custom_fields", Raw: view.CustomFields, OnRequest: true},
		{Name: "attachments", Raw: view.Attachments, OnRequest: true},
		{Name: "journals", Raw: view.Journals, OnRequest: true},
		{Name: "relations", Raw: view.Relations, OnRequest: true},
		{Name: "watchers", Raw: view.Watchers, OnRequest: true},
		{Name: "children", Raw: view.Children, OnRequest: true},
	}
}

func namedValue(value *redmine.NamedID) string {
	if value == nil {
		return ""
	}
	return value.Name
}

func profileViews(values []profile.Profile) []profileView {
	result := make([]profileView, 0, len(values))
	for _, value := range values {
		result = append(result, profileView{value})
	}
	return result
}

func projectViews(values []redmine.Project) []projectView {
	result := make([]projectView, 0, len(values))
	for _, value := range values {
		result = append(result, projectView{value})
	}
	return result
}

func issueViews(values []redmine.Issue) []issueView {
	result := make([]issueView, 0, len(values))
	for _, value := range values {
		result = append(result, issueView{value})
	}
	return result
}
