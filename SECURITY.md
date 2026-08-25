# Security

Report suspected credential disclosure or request-boundary vulnerabilities
privately to the repository owner. Do not include live Redmine tokens, complete
request headers, Keychain dumps, or production response bodies in a report.

The supported macOS build stores one API token per named profile in Keychain.
Profile metadata contains only the profile name and normalized Redmine base
URL. The CLI refuses redirects and sends credentials only in the
`X-Redmine-API-Key` header to the selected profile's validated HTTPS origin.

The source-built distribution deliberately uses an allow-any-application
decrypt ACL so rebuilt unsigned binaries can run non-interactively. Keychain
therefore provides encrypted at-rest storage, not isolation from other
processes running as the same macOS user while the Keychain is unlocked. Use a
dedicated OS account for stronger local process isolation, and rotate the
Redmine token after any suspected compromise of that account.
