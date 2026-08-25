# Step 02 - Sensitive File Protection

- Project type: Go
- Repository structure: single application
- Gitignore files: `.gitignore`
- `git ls-files` reported no tracked `.env`, key, certificate, or credential files in the committed release candidate.
- No `.env`, `.env.*`, private key, certificate, PKCS#12, cloud credential JSON, `credentials/`, `secrets/`, or credential-bearing `config.yaml` file exists in the committed snapshot.
- `.gitignore:6-12` covers `.env`, `.env.*`, private key, PEM, P12, and PFX files while allowing a safe `.env.example`.
- No `.env.example` is required because this CLI has no environment-variable credential contract.
- Profile metadata is non-secret and stored separately from Keychain credentials.
- Risks identified: none in sensitive-file protection.
