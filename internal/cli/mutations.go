package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/abigotado/redmine-cli/internal/errx"
	"github.com/abigotado/redmine-cli/internal/redmine"
	"github.com/spf13/cobra"
)

type writePreview struct {
	Method      string              `json:"method"`
	Resource    string              `json:"resource"`
	Attributes  map[string]any      `json:"attributes"`
	Attachments []attachmentPreview `json:"attachments,omitempty"`
}
type attachmentPreview struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}
type fileReceipt struct {
	ProjectID int    `json:"project_id"`
	Filename  string `json:"filename"`
	Added     bool   `json:"added"`
}

func (a *App) mutationAllowed(action string) error {
	if a.out.Format == "raw" {
		return errx.Usage("--output raw is not supported for write commands")
	}
	if !a.assumeYes {
		return errx.ConfirmRequired(action)
	}
	return nil
}

func (a *App) preview(preview writePreview) error {
	if a.out.Format == "raw" {
		return errx.Usage("--output raw is not supported for write commands")
	}
	if a.profileName == "" {
		return errx.ProfileRequired()
	}
	return a.out.Success(preview)
}

func positive(value, label string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 || strconv.Itoa(n) != value {
		return 0, errx.Usage("%s must be a positive base-10 integer", label)
	}
	return n, nil
}

func changedString(cmd *cobra.Command, name, value string, attrs map[string]any) {
	if cmd.Flags().Changed(name) {
		attrs[strings.ReplaceAll(name, "-", "_")] = value
	}
}
func changedID(cmd *cobra.Command, name, value string, attrs map[string]any, nullable bool) error {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	key := strings.ReplaceAll(name, "-", "_")
	if value == "none" && nullable {
		attrs[key] = nil
		return nil
	}
	id, err := positive(value, strings.ReplaceAll(name, "-", " "))
	if err != nil {
		return err
	}
	attrs[key] = id
	return nil
}
func changedBool(cmd *cobra.Command, name, value string, attrs map[string]any) error {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	if value != "true" && value != "false" {
		return errx.Usage("--%s must be true or false", name)
	}
	attrs[strings.ReplaceAll(name, "-", "_")] = value == "true"
	return nil
}

func openSelectedRegularFile(path, label string, expected os.FileInfo) (*os.File, error) {
	file, err := openNoFollow(path)
	if err != nil {
		return nil, errx.Usage("selected %s changed before upload", label)
	}
	current, statErr := file.Stat()
	if statErr != nil || !current.Mode().IsRegular() || current.Size() != expected.Size() || !os.SameFile(expected, current) {
		closeErr := file.Close()
		if closeErr != nil {
			return nil, errx.Internal("close selected %s", label)
		}
		return nil, errx.Usage("selected %s changed before upload", label)
	}
	return file, nil
}

func issueAttachments(ctx context.Context, client redmineReader, paths []string) ([]map[string]string, error) {
	if len(paths) > 10 {
		return nil, errx.Usage("at most 10 attachments are supported")
	}
	tokens := make([]redmine.UploadToken, 0, len(paths))
	names := make([]string, 0, len(paths))
	var total int64
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errx.Usage("--attach must name a regular file")
		}
		if info.Size() > 50<<20 || total+info.Size() > 100<<20 {
			return nil, errx.Usage("attachments exceed the safety limit")
		}
		total += info.Size()
		file, err := openSelectedRegularFile(path, "attachment", info)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(path)
		token, uploadErr := client.Upload(ctx, name, file)
		closeErr := file.Close()
		if uploadErr != nil {
			return nil, uploadErr
		}
		if closeErr != nil {
			return nil, errx.Internal("close selected attachment")
		}
		tokens = append(tokens, token)
		names = append(names, name)
	}
	return redmine.IssueUploads(tokens, names), nil
}

func attachmentPreviews(paths []string) ([]attachmentPreview, error) {
	if len(paths) > 10 {
		return nil, errx.Usage("at most 10 attachments are supported")
	}
	result := make([]attachmentPreview, 0, len(paths))
	var total int64
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errx.Usage("--attach must name a regular file")
		}
		if info.Size() > 50<<20 || total+info.Size() > 100<<20 {
			return nil, errx.Usage("attachments exceed the safety limit")
		}
		total += info.Size()
		result = append(result, attachmentPreview{Filename: filepath.Base(path), Size: info.Size()})
	}
	return result, nil
}

func (a *App) newIssuesCreateCommand() *cobra.Command {
	var projectID, subject, description, trackerID, statusID, priorityID, assignee string
	var attachments []string
	cmd := &cobra.Command{Use: "create", Short: "Create one Redmine issue", Args: usageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		if err := a.out.Validate(issueView{}); err != nil {
			return err
		}
		project, err := positive(projectID, "project ID")
		if err != nil {
			return err
		}
		if strings.TrimSpace(subject) == "" {
			return errx.Usage("--subject must not be empty")
		}
		attrs := map[string]any{"project_id": project, "subject": subject}
		changedString(cmd, "description", description, attrs)
		for _, item := range []struct {
			name, value string
			nullable    bool
		}{{"tracker-id", trackerID, false}, {"status-id", statusID, false}, {"priority-id", priorityID, false}, {"assigned-to-id", assignee, true}} {
			if err := changedID(cmd, item.name, item.value, attrs, item.nullable); err != nil {
				return err
			}
		}
		previews, err := attachmentPreviews(attachments)
		if err != nil {
			return err
		}
		preview := writePreview{Method: "POST", Resource: "issues", Attributes: attrs, Attachments: previews}
		if a.dryRun {
			return a.preview(preview)
		}
		if err := a.mutationAllowed("issue create"); err != nil {
			return err
		}
		client, _, err := a.client(cmd.Context())
		if err != nil {
			return err
		}
		if len(attachments) > 0 {
			uploads, uploadErr := issueAttachments(cmd.Context(), client, attachments)
			if uploadErr != nil {
				return uploadErr
			}
			attrs["uploads"] = uploads
		}
		issue, err := client.CreateIssue(cmd.Context(), attrs)
		if err != nil {
			return err
		}
		return a.out.Success(issueView{issue})
	}}
	flags := cmd.Flags()
	flags.StringVar(&projectID, "project-id", "", "numeric Redmine project ID")
	flags.StringVar(&subject, "subject", "", "issue subject")
	flags.StringVar(&description, "description", "", "issue description")
	flags.StringVar(&trackerID, "tracker-id", "", "numeric tracker ID")
	flags.StringVar(&statusID, "status-id", "", "numeric status ID")
	flags.StringVar(&priorityID, "priority-id", "", "numeric priority ID")
	flags.StringVar(&assignee, "assigned-to-id", "", "numeric assignee ID or none")
	flags.StringSliceVar(&attachments, "attach", nil, "regular file to attach (repeatable)")
	_ = cmd.MarkFlagRequired("project-id")
	_ = cmd.MarkFlagRequired("subject")
	return cmd
}

func (a *App) newIssuesUpdateCommand() *cobra.Command {
	var subject, description, statusID, assignee string
	var attachments []string
	cmd := &cobra.Command{Use: "update ID", Short: "Update one Redmine issue", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if err := a.out.Validate(issueView{}); err != nil {
			return err
		}
		id, err := positive(args[0], "issue ID")
		if err != nil {
			return err
		}
		attrs := map[string]any{}
		changedString(cmd, "subject", subject, attrs)
		changedString(cmd, "description", description, attrs)
		if err := changedID(cmd, "status-id", statusID, attrs, false); err != nil {
			return err
		}
		if err := changedID(cmd, "assigned-to-id", assignee, attrs, true); err != nil {
			return err
		}
		if len(attrs) == 0 && len(attachments) == 0 {
			return errx.Usage("provide at least one field to update")
		}
		previews, err := attachmentPreviews(attachments)
		if err != nil {
			return err
		}
		preview := writePreview{Method: "PUT", Resource: "issues/" + args[0], Attributes: attrs, Attachments: previews}
		if a.dryRun {
			return a.preview(preview)
		}
		if err := a.mutationAllowed("issue update"); err != nil {
			return err
		}
		client, _, err := a.client(cmd.Context())
		if err != nil {
			return err
		}
		if len(attachments) > 0 {
			uploads, uploadErr := issueAttachments(cmd.Context(), client, attachments)
			if uploadErr != nil {
				return uploadErr
			}
			attrs["uploads"] = uploads
		}
		issue, err := client.UpdateIssue(cmd.Context(), id, attrs)
		if err != nil {
			return err
		}
		return a.out.Success(issueView{issue})
	}}
	flags := cmd.Flags()
	flags.StringVar(&subject, "subject", "", "issue subject")
	flags.StringVar(&description, "description", "", "issue description (empty clears)")
	flags.StringVar(&statusID, "status-id", "", "numeric status ID")
	flags.StringVar(&assignee, "assigned-to-id", "", "numeric assignee ID or none")
	flags.StringSliceVar(&attachments, "attach", nil, "regular file to attach (repeatable)")
	return cmd
}

func (a *App) newProjectsCreateCommand() *cobra.Command {
	var name, identifier, description, homepage, public, parentID, inheritMembers string
	cmd := &cobra.Command{Use: "create", Short: "Create one Redmine project", Args: usageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		if err := a.out.Validate(projectView{}); err != nil {
			return err
		}
		if strings.TrimSpace(name) == "" || strings.TrimSpace(identifier) == "" {
			return errx.Usage("--name and --identifier must not be empty")
		}
		attrs := map[string]any{"name": name, "identifier": identifier}
		changedString(cmd, "description", description, attrs)
		changedString(cmd, "homepage", homepage, attrs)
		if err := changedBool(cmd, "public", public, attrs); err != nil {
			return err
		}
		if err := changedID(cmd, "parent-id", parentID, attrs, true); err != nil {
			return err
		}
		if err := changedBool(cmd, "inherit-members", inheritMembers, attrs); err != nil {
			return err
		}
		preview := writePreview{Method: "POST", Resource: "projects", Attributes: attrs}
		if a.dryRun {
			return a.preview(preview)
		}
		if err := a.mutationAllowed("project create"); err != nil {
			return err
		}
		client, _, err := a.client(cmd.Context())
		if err != nil {
			return err
		}
		project, err := client.CreateProject(cmd.Context(), attrs)
		if err != nil {
			return err
		}
		return a.out.Success(projectView{project})
	}}
	flags := cmd.Flags()
	flags.StringVar(&name, "name", "", "project name")
	flags.StringVar(&identifier, "identifier", "", "project identifier")
	flags.StringVar(&description, "description", "", "project description")
	flags.StringVar(&homepage, "homepage", "", "project homepage")
	flags.StringVar(&public, "public", "", "true or false")
	flags.StringVar(&parentID, "parent-id", "", "numeric parent project ID or none")
	flags.StringVar(&inheritMembers, "inherit-members", "", "true or false")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("identifier")
	return cmd
}

func (a *App) newProjectsUpdateCommand() *cobra.Command {
	var name, identifier, description, homepage, public, parentID, inheritMembers string
	cmd := &cobra.Command{Use: "update ID_OR_IDENTIFIER", Short: "Update one Redmine project", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if err := a.out.Validate(projectView{}); err != nil {
			return err
		}
		attrs := map[string]any{}
		changedString(cmd, "name", name, attrs)
		changedString(cmd, "identifier", identifier, attrs)
		changedString(cmd, "description", description, attrs)
		changedString(cmd, "homepage", homepage, attrs)
		if err := changedBool(cmd, "public", public, attrs); err != nil {
			return err
		}
		if err := changedID(cmd, "parent-id", parentID, attrs, true); err != nil {
			return err
		}
		if err := changedBool(cmd, "inherit-members", inheritMembers, attrs); err != nil {
			return err
		}
		if len(attrs) == 0 {
			return errx.Usage("provide at least one field to update")
		}
		preview := writePreview{Method: "PUT", Resource: "projects/" + args[0], Attributes: attrs}
		if a.dryRun {
			return a.preview(preview)
		}
		if err := a.mutationAllowed("project update"); err != nil {
			return err
		}
		client, _, err := a.client(cmd.Context())
		if err != nil {
			return err
		}
		existing, err := client.Project(cmd.Context(), args[0], nil)
		if err != nil {
			return err
		}
		project, err := client.UpdateProject(cmd.Context(), existing.ID, attrs)
		if err != nil {
			return err
		}
		return a.out.Success(projectView{project})
	}}
	flags := cmd.Flags()
	flags.StringVar(&name, "name", "", "project name")
	flags.StringVar(&identifier, "identifier", "", "project identifier")
	flags.StringVar(&description, "description", "", "project description (empty clears)")
	flags.StringVar(&homepage, "homepage", "", "project homepage (empty clears)")
	flags.StringVar(&public, "public", "", "true or false")
	flags.StringVar(&parentID, "parent-id", "", "numeric parent project ID or none")
	flags.StringVar(&inheritMembers, "inherit-members", "", "true or false")
	return cmd
}

func (a *App) newFilesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "files", Short: "Read and add Redmine project files", Args: usageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error { return errx.Usage("%s needs a command", cmd.CommandPath()) }}
	cmd.AddCommand(a.newFilesListCommand(), a.newFilesDownloadCommand(), a.newFilesAddCommand())
	return cmd
}
func (a *App) newFilesListCommand() *cobra.Command {
	return &cobra.Command{Use: "list PROJECT_ID_OR_IDENTIFIER", Short: "List project files", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if err := a.out.Validate(fileViews(nil)); err != nil {
			return err
		}
		client, _, err := a.client(cmd.Context())
		if err != nil {
			return err
		}
		files, err := client.Files(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return a.out.Success(fileViews(files))
	}}
}
func (a *App) newFilesDownloadCommand() *cobra.Command {
	return &cobra.Command{Use: "download ATTACHMENT_ID", Short: "Download one attachment", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if a.out.Format != "raw" || len(a.fields) > 0 {
			return errx.Usage("files download requires --output raw without --fields")
		}
		id, err := positive(args[0], "attachment ID")
		if err != nil {
			return err
		}
		client, _, err := a.client(cmd.Context())
		if err != nil {
			return err
		}
		data, err := client.DownloadAttachment(cmd.Context(), id)
		if err != nil {
			return err
		}
		_, err = io.Copy(a.stdout, bytes.NewReader(data))
		if err != nil {
			return errx.Internal("write attachment output")
		}
		return nil
	}}
}
func (a *App) newFilesAddCommand() *cobra.Command {
	var path, filename, description string
	var versionID string
	cmd := &cobra.Command{Use: "add PROJECT_ID_OR_IDENTIFIER", Short: "Add one file to a project", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if a.out.Format == "raw" || len(a.fields) > 0 {
			return errx.Usage("--output raw is not supported for write commands")
		}
		version := 0
		if cmd.Flags().Changed("version-id") {
			parsed, parseErr := positive(versionID, "version ID")
			if parseErr != nil {
				return parseErr
			}
			version = parsed
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return errx.Usage("--path must name a regular file")
		}
		if info.Size() > 50<<20 {
			return errx.Usage("file exceeds the 50 MiB safety limit")
		}
		name := filename
		if name == "" {
			name = filepath.Base(path)
		}
		if len([]byte(name)) > 255 {
			return errx.Usage("filename exceeds 255 bytes")
		}
		attributes := map[string]any{"project": args[0]}
		if description != "" {
			attributes["description"] = description
		}
		if version > 0 {
			attributes["version_id"] = version
		}
		preview := writePreview{Method: "POST", Resource: "project files", Attributes: attributes, Attachments: []attachmentPreview{{Filename: name, Size: info.Size()}}}
		if a.dryRun {
			return a.preview(preview)
		}
		if err := a.mutationAllowed("file add"); err != nil {
			return err
		}
		client, _, err := a.client(cmd.Context())
		if err != nil {
			return err
		}
		project, err := client.Project(cmd.Context(), args[0], nil)
		if err != nil {
			return err
		}
		file, err := openSelectedRegularFile(path, "file", info)
		if err != nil {
			return err
		}
		token, uploadErr := client.Upload(cmd.Context(), name, file)
		closeErr := file.Close()
		if uploadErr != nil {
			return uploadErr
		}
		if closeErr != nil {
			return errx.Internal("close selected file")
		}
		if err := client.AddFile(cmd.Context(), project.ID, token, name, description, version); err != nil {
			return err
		}
		return a.out.Success(fileReceipt{ProjectID: project.ID, Filename: name, Added: true})
	}}
	flags := cmd.Flags()
	flags.StringVar(&path, "path", "", "local regular file")
	flags.StringVar(&filename, "filename", "", "uploaded filename")
	flags.StringVar(&description, "description", "", "file description")
	flags.StringVar(&versionID, "version-id", "", "numeric project version ID")
	_ = cmd.MarkFlagRequired("path")
	return cmd
}
