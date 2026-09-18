# Security

Report suspected credential disclosure or request-boundary vulnerabilities
privately to the repository owner. Do not include live Redmine tokens, complete
request headers, Keychain dumps, or production response bodies in a report.

The supported macOS build stores one API token per named profile in Keychain.
Profile metadata contains only the profile name and normalized Redmine base
URL. The CLI refuses redirects and sends credentials only in the
`X-Redmine-API-Key` header to the selected profile's validated HTTPS origin.

Network mutations require `--yes` and are sent exactly once: a transport or
server failure after a write can mean that Redmine applied it, so callers must
inspect Redmine before retrying. File uploads use opaque in-memory tokens that
are never rendered. Attachment downloads first validate a same-origin Redmine
URL and buffer the bounded response before writing raw bytes to stdout.

The source-built distribution deliberately uses an allow-any-application
decrypt ACL so rebuilt unsigned binaries can run non-interactively. Keychain
therefore provides encrypted at-rest storage, not isolation from other
processes running as the same macOS user while the Keychain is unlocked. Use a
dedicated OS account for stronger local process isolation, and rotate the
Redmine token after any suspected compromise of that account.
