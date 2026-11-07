# Security Policy

## Reporting a vulnerability

Please report vulnerabilities **privately** using GitHub's private vulnerability reporting: open the repository's **Security** tab, then **Report a vulnerability** (Security Advisories). Do not open a public issue or pull request for security problems.

Include the version (shown in the release name), your Windows version, clear reproduction steps and the impact you observed. You can expect an acknowledgement within a few days; this is a volunteer project, so fixes are best-effort, and we will coordinate disclosure with you.

## Scope

In scope:

- **Wrongful deletion** (high severity): anything that can delete a protected path, a path the user did not confirm, a path outside the confirmed plan, or follow a junction/symlink into protected areas; bypasses of the confirmation token (single-use, 5 minutes, bound to the plan); a rule or AI response that can trigger or widen a deletion.
- Leaking data that should stay local: API keys outside the Windows Credential Manager, file contents, or non-anonymized data in the AI payload that differs from the preview shown to the user.
- Injection through AI output or rule files (for example script execution via the rendered report).
- Supply-chain issues in the release workflow or published artifacts.

Out of scope: issues that need an already-compromised machine or administrator account, missing code signing (known, tracked in the roadmap), and the documented limitations in the README (for example hardlinks counted twice).

## Supported versions

Only the latest release receives fixes.
