---
name: redmine
description: Read Redmine profiles, the current user, projects, and issues through the redmine-cli machine contract. Use for Redmine issue IDs, project inventories, assigned issue lists, issue details, journals, attachments, and relations; the external API surface is read-only.
---

# Read Redmine with redmine-cli

Use `redmine-cli` as the only Redmine and credential boundary. Every normal
invocation writes one versioned JSON envelope to stdout; parse it, branch on the
process exit code, and follow `hint`. stderr is diagnostic only. Explicit
`--help`, `-o text`, and `-o raw` are caller-selected exceptions.

The installed command surface reads Redmine only. Do not create, edit, comment
on, upload to, or delete Redmine data, and do not bypass the CLI with direct
REST calls.

## Select an explicit profile

Pass `--profile NAME` on every command that contacts Redmine. Never infer an
account from a URL, company, issue subject, environment variable, or prior call.
If the user has not named a profile, run `redmine-cli auth list`, show only
its non-secret metadata, and ask them to choose. Never select even a sole
profile on their behalf.

Never inspect Keychain, run `security`, read credential-bearing environment
variables, print headers, or ask the user to paste a token into chat. On exit 5,
ask the user to run `redmine-cli auth login` themselves.

## Read exact objects directly

A positive numeric issue ID is exact:

```text
redmine-cli issues get 123 --profile work --fields id,subject,status,assigned_to,updated_on
```

Do not convert it into a list query first. Project get accepts a numeric ID or
identifier; issue list `--project-id` is numeric.

## Keep output bounded

Every collection call must pass an explicit `--limit` from 1 to 100; normally
start at 25. Request the smallest useful `--fields` set. Add long or nested
fields such as `description`, `journals`, or `attachments` only when needed.

Follow `meta.next_cursor` with the same profile, filters, sort, include, and
field intent. Treat cursors as opaque. Stop when the answer is complete or no
cursor is present. Issue lists explicitly default to open issues.

Issue-list associations are only `attachments` and `relations`. Exact
issue reads may additionally request `journals`, `watchers`, and
`children`.

## Recovery

Exit 0 means the envelope is usable. For every other exit, read
`error.code` and `hint`; do not repeat the same failing command unchanged.
Read [reference/contract.md](reference/contract.md) for the exit table and
[reference/commands.md](reference/commands.md) for flags and pagination.
Use `redmine-cli contract` if the installed binary reports a different
contract version.
