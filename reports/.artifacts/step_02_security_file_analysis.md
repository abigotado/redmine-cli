# Step 02 - Sensitive File Protection

## Scope and detection

- Detected project type: Go (`PROJECT_DETECTION_RESULTS: go@.` from step 01).
- Repository structure: single application. No `apps/`, `packages/`, `libs/`, or `vendor/` directory is present.
- Gitignore files found: root `.gitignore` only.
- This check inspected path, tracking, ignore, and Git mode metadata only. It did not read any credential-file contents.

## Sensitive files and protection status

| File class | Present in tracked files | Present untracked and not ignored | Present untracked and ignored | Protection status |
| --- | ---: | ---: | ---: | --- |
| `.env`, `.env.local`, `.env.*` | 0 | 0 | 0 | Protected prospectively by `.env` and `.env.*`; `.env.example` is explicitly allowed. |
| `*.key`, `*.pem`, `*.p12`, `*.pfx` | 0 | 0 | 0 | Protected prospectively by matching root ignore rules. |
| `*.cert` | 0 | 0 | 0 | No file exists; no ignore rule is present. |
| `secrets/`, `credentials/` | 0 | 0 | 0 | No directory or file exists; no ignore rule is present. |
| `*-key.json`, `*-credentials.json`, `service-account*.json` | 0 | 0 | 0 | No file exists; no dedicated ignore rule is present. |
| `config.yaml`, `config.yml` | 0 | 0 | 0 | No credential configuration file exists; no dedicated ignore rule is present. |

Mandatory environment verification was clean: `git ls-files .env .env.local .env.development .env.production .env.staging .env.test '.env.*'` returned no paths, while `.gitignore` contains both `.env` and `.env.*` rules. There are **zero tracked credential files** among all checked classes, so there are no credential file-mode findings.

No `.env.example` or `.env.sample` is tracked. This is not a finding for this project because the CLI contract does not use environment variables for credentials; Redmine API tokens are confined to Keychain.

## `.gitignore` coverage

Coverage present:

- `.env` and `.env.*`, with `!.env.example` for a safe template if one is ever introduced.
- `*.key`, `*.pem`, `*.p12`, and `*.pfx`.
- `/bin/`, `/dist/`, `/coverage/`, `*.test`, `*.out`, and `.DS_Store` for Go/build output and common local artifacts.

Defense-in-depth coverage gaps:

- No generic rules for `*.cert`, `secrets/`, `credentials/`, `*-key.json`, `*-credentials.json`, or `service-account*.json`.
- No generic rules for `*.exe`, `*.log`, or `vendor/`.

These are hardening gaps, not current security risks: metadata inspection found none of the corresponding sensitive files, and the repository does not contain a vendored dependency tree.

## Security risks identified

None. No sensitive file is tracked, and no sensitive file exists locally without ignore coverage.

## Missing technical security configurations

None required for the current repository contents. The optional ignore-rule hardening above would reduce the chance of accidental future additions but is not evidence of current secret exposure.
