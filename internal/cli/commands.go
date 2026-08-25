package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/abigotado/redmine-cli/internal/auth"
	"github.com/abigotado/redmine-cli/internal/errx"
	"github.com/abigotado/redmine-cli/internal/output"
	"github.com/abigotado/redmine-cli/internal/profile"
	"github.com/abigotado/redmine-cli/internal/redmine"
	"github.com/abigotado/redmine-cli/internal/skills"
	"github.com/spf13/cobra"
)

func (a *App) newContractCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "contract",
		Short: "Print the machine-readable envelope and exit-code contract",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := a.out.Validate(errx.Contract{}); err != nil {
				return err
			}
			return a.out.Success(errx.Describe())
		},
	}
}

func (a *App) newAuthCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "auth",
		Short: "Manage named Redmine profiles and Keychain credentials",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return errx.Usage("%s needs a command", cmd.CommandPath())
		},
	}
	command.AddCommand(
		a.newAuthLoginCommand(),
		a.newAuthListCommand(),
		a.newAuthStatusCommand(),
		a.newAuthLogoutCommand(),
	)
	return command
}

func (a *App) newAuthLoginCommand() *cobra.Command {
	var baseURL string
	var tokenStdin bool
	command := &cobra.Command{
		Use:   "login",
		Short: "Verify and store one profile token",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.out.Validate(profileView{}); err != nil {
				return err
			}
			if a.profileName == "" {
				return errx.ProfileRequired()
			}
			if err := profile.ValidateName(a.profileName); err != nil {
				return translateLocal(err, a.profileName)
			}
			normalized, err := profile.NormalizeBaseURL(baseURL)
			if err != nil {
				return translateLocal(err, a.profileName)
			}
			candidate := profile.Profile{Name: a.profileName, BaseURL: normalized}
			if a.registry == nil {
				return errx.Internal("profile registry is unavailable")
			}
			if a.dryRun {
				return a.out.Success(profileView{candidate})
			}
			if !tokenStdin {
				return errx.Usage("auth login requires --token-stdin")
			}
			if err := a.requireLoginConfirmation(cmd.Context(), candidate.Name); err != nil {
				return err
			}
			credential, err := auth.ReadToken(cmd.Context(), a.stdin)
			if err != nil {
				return translateLocal(err, candidate.Name)
			}
			client, err := a.newRedmine(candidate, credential, a.log)
			if err != nil {
				return err
			}
			if _, err := client.Myself(cmd.Context()); err != nil {
				return err
			}
			if err := auth.Login(cmd.Context(), a.store, a.registry, candidate, credential, a.assumeYes); err != nil {
				return translateLocal(err, candidate.Name)
			}
			a.out.WithContext(candidate.Name, candidate.BaseURL)
			return a.out.Success(profileView{candidate})
		},
	}
	command.Flags().StringVar(&baseURL, "url", "", "canonical Redmine HTTPS base URL")
	command.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read one bounded API token from stdin")
	_ = command.MarkFlagRequired("url")
	return command
}

func (a *App) requireLoginConfirmation(ctx context.Context, name string) error {
	var exists bool
	err := a.registry.WithProfileLock(ctx, name, func() error {
		_, profileErr := a.registry.Get(ctx, name)
		switch {
		case profileErr == nil:
			exists = true
		case errors.Is(profileErr, profile.ErrNotFound):
		default:
			return translateLocal(profileErr, name)
		}
		_, credentialErr := a.store.Load(ctx, name)
		switch {
		case credentialErr == nil:
			exists = true
		case errors.Is(credentialErr, auth.ErrNotFound):
		default:
			return translateLocal(credentialErr, name)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if exists && !a.assumeYes {
		return errx.ConfirmRequired("auth login overwrite")
	}
	return nil
}

func (a *App) newAuthListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List non-secret profile metadata without reading Keychain",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.out.Validate(profileViews(nil)); err != nil {
				return err
			}
			if a.registry == nil {
				return errx.Internal("profile registry is unavailable")
			}
			profiles, err := a.registry.List(cmd.Context())
			if err != nil {
				return translateLocal(err, "")
			}
			return a.out.Success(profileViews(profiles))
		},
	}
}

func (a *App) newAuthStatusCommand() *cobra.Command {
	var check bool
	command := &cobra.Command{
		Use:   "status",
		Short: "Show profile metadata and optionally verify it against Redmine",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !check {
				if err := a.out.Validate(profileView{}); err != nil {
					return err
				}
				selected, err := a.selectedProfile(cmd.Context())
				if err != nil {
					return err
				}
				a.out.WithContext(selected.Name, selected.BaseURL)
				return a.out.Success(profileView{selected})
			}
			if err := a.out.Validate(userView{}); err != nil {
				return err
			}
			client, _, err := a.client(cmd.Context())
			if err != nil {
				return err
			}
			user, err := client.Myself(cmd.Context())
			if err != nil {
				return err
			}
			return a.out.Success(userView{user})
		},
	}
	command.Flags().BoolVar(&check, "check", false, "verify the stored token with a network request")
	return command
}

func (a *App) newAuthLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete one profile token and metadata",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.out.Validate(map[string]any{}); err != nil {
				return err
			}
			selected, err := a.selectedProfile(cmd.Context())
			if err != nil {
				return err
			}
			if a.dryRun {
				return a.out.Success(profileView{selected})
			}
			if !a.assumeYes {
				return errx.ConfirmRequired("auth logout")
			}
			if err := auth.Logout(cmd.Context(), a.store, a.registry, selected.Name); err != nil {
				return translateLocal(err, selected.Name)
			}
			return a.out.Success(map[string]any{"profile": selected.Name, "removed": true})
		},
	}
}

func (a *App) newMeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Read the authenticated Redmine user without exposing api_key",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.out.Validate(userView{}); err != nil {
				return err
			}
			client, _, err := a.client(cmd.Context())
			if err != nil {
				return err
			}
			user, err := client.Myself(cmd.Context())
			if err != nil {
				return err
			}
			return a.out.Success(userView{user})
		},
	}
}

func (a *App) newProjectsCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "projects",
		Short: "Read Redmine projects",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return errx.Usage("%s needs a command", cmd.CommandPath())
		},
	}
	command.AddCommand(a.newProjectsListCommand(), a.newProjectsGetCommand())
	return command
}

func (a *App) newProjectsListCommand() *cobra.Command {
	var limit int
	var cursor string
	var includes []string
	command := &cobra.Command{
		Use:   "list",
		Short: "List one bounded page of visible projects",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.out.Validate(projectViews(nil)); err != nil {
				return err
			}
			if limit < 1 || limit > redmine.MaxLimit {
				return errx.Usage("--limit must be between 1 and %d", redmine.MaxLimit)
			}
			derived, err := projectIncludes(a.fields, includes)
			if err != nil {
				return err
			}
			client, selected, err := a.client(cmd.Context())
			if err != nil {
				return err
			}
			options := redmine.ProjectListOptions{Limit: limit, Include: derived}
			query, err := options.Query()
			if err != nil {
				return err
			}
			fingerprint := redmine.Fingerprint(selected.Name, selected.BaseURL, "projects", query)
			options.Offset, err = redmine.DecodeCursor(cursor, "projects", fingerprint)
			if err != nil {
				return err
			}
			page, err := client.Projects(cmd.Context(), options)
			if err != nil {
				return err
			}
			truncated := page.Offset+len(page.Projects) < page.TotalCount
			nextCursor, err := nextPageCursor("projects", page.Offset, len(page.Projects), truncated, fingerprint)
			if err != nil {
				return err
			}
			return a.out.SuccessPage(projectViews(page.Projects), output.Page{
				TotalCount: page.TotalCount, Offset: page.Offset, Limit: page.Limit,
				Truncated: truncated, NextCursor: nextCursor,
			})
		},
	}
	command.Flags().IntVar(&limit, "limit", redmine.DefaultLimit, "maximum projects in this page (1-100)")
	command.Flags().StringVar(&cursor, "cursor", "", "opaque next_cursor from a previous matching call")
	command.Flags().StringSliceVar(&includes, "include", nil, "associations: trackers,issue_categories,enabled_modules,time_entry_activities,issue_custom_fields")
	return command
}

func (a *App) newProjectsGetCommand() *cobra.Command {
	var includes []string
	command := &cobra.Command{
		Use:   "get ID_OR_IDENTIFIER",
		Short: "Read one project by numeric ID or identifier",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.out.Validate(projectView{}); err != nil {
				return err
			}
			derived, err := projectIncludes(a.fields, includes)
			if err != nil {
				return err
			}
			client, _, err := a.client(cmd.Context())
			if err != nil {
				return err
			}
			projectValue, err := client.Project(cmd.Context(), args[0], derived)
			if err != nil {
				return err
			}
			return a.out.Success(projectView{projectValue})
		},
	}
	command.Flags().StringSliceVar(&includes, "include", nil, "associations: trackers,issue_categories,enabled_modules,time_entry_activities,issue_custom_fields")
	return command
}

func (a *App) newIssuesCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "issues",
		Short: "Read Redmine issues",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return errx.Usage("%s needs a command", cmd.CommandPath())
		},
	}
	command.AddCommand(a.newIssuesListCommand(), a.newIssuesGetCommand())
	return command
}

func (a *App) newIssuesListCommand() *cobra.Command {
	var options redmine.IssueListOptions
	var cursor string
	var includes []string
	command := &cobra.Command{
		Use:   "list",
		Short: "List one bounded page of issues with explicit filters",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.out.Validate(issueViews(nil)); err != nil {
				return err
			}
			derived, err := issueIncludes(a.fields, includes, false)
			if err != nil {
				return err
			}
			options.Include = derived
			client, selected, err := a.client(cmd.Context())
			if err != nil {
				return err
			}
			options.Offset = 0
			query, err := options.Query()
			if err != nil {
				return err
			}
			fingerprint := redmine.Fingerprint(selected.Name, selected.BaseURL, "issues", query)
			options.Offset, err = redmine.DecodeCursor(cursor, "issues", fingerprint)
			if err != nil {
				return err
			}
			page, err := client.Issues(cmd.Context(), options)
			if err != nil {
				return err
			}
			truncated := page.Offset+len(page.Issues) < page.TotalCount
			nextCursor, err := nextPageCursor("issues", page.Offset, len(page.Issues), truncated, fingerprint)
			if err != nil {
				return err
			}
			return a.out.SuccessPage(issueViews(page.Issues), output.Page{
				TotalCount: page.TotalCount, Offset: page.Offset, Limit: page.Limit,
				Truncated: truncated, NextCursor: nextCursor,
			})
		},
	}
	flags := command.Flags()
	flags.IntVar(&options.Limit, "limit", redmine.DefaultLimit, "maximum issues in this page (1-100)")
	flags.StringVar(&cursor, "cursor", "", "opaque next_cursor from a previous matching call")
	flags.StringVar(&options.ProjectID, "project-id", "", "numeric Redmine project ID")
	flags.StringVar(&options.TrackerID, "tracker-id", "", "numeric tracker ID")
	flags.StringVar(&options.StatusID, "status", "open", "open, closed, *, or a numeric status ID")
	flags.StringVar(&options.AssignedToID, "assigned-to", "", "me or a numeric user ID")
	flags.StringVar(&options.CreatedOn, "created-on", "", "Redmine created_on filter expression")
	flags.StringVar(&options.UpdatedOn, "updated-on", "", "Redmine updated_on filter expression")
	flags.StringVar(&options.Sort, "sort", "updated_on:desc", "comma-separated allowlisted sort fields")
	flags.StringSliceVar(&includes, "include", nil, "associations: attachments,relations")
	return command
}

func (a *App) newIssuesGetCommand() *cobra.Command {
	var includes []string
	command := &cobra.Command{
		Use:   "get ID",
		Short: "Read one issue by exact numeric ID",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.out.Validate(issueView{}); err != nil {
				return err
			}
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 || strconv.Itoa(id) != args[0] {
				return errx.Usage("issue ID must be a positive base-10 integer")
			}
			derived, err := issueIncludes(a.fields, includes, true)
			if err != nil {
				return err
			}
			client, _, err := a.client(cmd.Context())
			if err != nil {
				return err
			}
			issue, err := client.Issue(cmd.Context(), id, derived)
			if err != nil {
				return err
			}
			return a.out.Success(issueView{issue})
		},
	}
	command.Flags().StringSliceVar(&includes, "include", nil, "associations: attachments,relations,journals,watchers,children")
	return command
}

func issueIncludes(fields, explicit []string, single bool) ([]string, error) {
	associations := map[string]bool{
		"attachments": true, "relations": true, "journals": true, "watchers": true, "children": true,
	}
	values := append([]string(nil), explicit...)
	for _, field := range fields {
		if associations[field] {
			values = append(values, field)
		}
	}
	return redmine.ValidateIncludes(values, single)
}

func projectIncludes(fields, explicit []string) ([]string, error) {
	associations := map[string]bool{
		"trackers": true, "issue_categories": true, "enabled_modules": true,
		"time_entry_activities": true, "issue_custom_fields": true,
	}
	values := append([]string(nil), explicit...)
	for _, field := range fields {
		if associations[field] {
			values = append(values, field)
		}
	}
	return redmine.ValidateProjectIncludes(values)
}

func nextPageCursor(resource string, offset, count int, truncated bool, fingerprint string) (string, error) {
	if !truncated {
		return "", nil
	}
	next := offset + count
	if count <= 0 || next <= offset {
		return "", errx.Internal("Redmine returned a non-advancing collection page")
	}
	return redmine.EncodeCursor(resource, next, fingerprint)
}

func (a *App) newSkillsCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "skills",
		Short: "Install the embedded Agent Skill for Codex and Claude Code",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return errx.Usage("%s needs a command", cmd.CommandPath())
		},
	}
	command.AddCommand(a.newSkillsActionCommand(true), a.newSkillsActionCommand(false))
	return command
}

func (a *App) newSkillsActionCommand(install bool) *cobra.Command {
	action := "uninstall"
	if install {
		action = "install"
	}
	var providerValue string
	var scopeValue string
	var projectDir string
	var dest string
	command := &cobra.Command{
		Use:   action,
		Short: fmt.Sprintf("%s the embedded Redmine Agent Skill", action),
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.out.Validate([]skills.Result(nil)); err != nil {
				return err
			}
			provider, err := skills.ParseProvider(providerValue)
			if err != nil {
				return err
			}
			scope, err := skills.ParseScope(scopeValue)
			if err != nil {
				return err
			}
			if projectDir == "" && scope == skills.ScopeProject {
				projectDir, err = os.Getwd()
				if err != nil {
					return errx.Internal("locate project directory")
				}
			}
			options := skills.Options{
				Provider: provider, Scope: scope, ProjectDir: projectDir, Dest: dest,
				Confirmed: a.assumeYes, DryRun: a.dryRun,
			}
			var results []skills.Result
			if install {
				results, err = skills.Install(cmd.Context(), options)
			} else {
				results, err = skills.Uninstall(cmd.Context(), options)
			}
			if err != nil {
				return err
			}
			return a.out.Success(results)
		},
	}
	command.Flags().StringVar(&providerValue, "provider", "all", "codex, claude, or all")
	command.Flags().StringVar(&scopeValue, "scope", "user", "user or project")
	command.Flags().StringVar(&projectDir, "project-dir", "", "project root for project scope")
	command.Flags().StringVar(&dest, "dest", "", "explicit provider root override")
	return command
}
