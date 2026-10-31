# Security Policy

## Supported Versions

YtVault is in **maintenance phase**. Security fixes are released for the latest
published version only. Users should pin a version tag rather than tracking
`:latest`.

| Version | Supported | Notes |
|---|---|---|
| Latest published tag (currently `v1.1.0`) | ✅ | Receives security fixes |
| `dev` / `:beta` | ❌ | Development branch; no guarantees |
| Older tags | ❌ | Upgrade to the latest tag |

Check the available tags at <https://github.com/AC13139/ytvault/tags>.
Official releases are Git tags; the repository does not publish GitHub Releases.

## Reporting a Vulnerability

**Please do not open a public issue for security problems.**

Report privately via GitHub's private vulnerability reporting:

> <https://github.com/AC13139/ytvault/security/advisories/new>

Private vulnerability reporting is enabled for this repository, so the form
above is always available.

Please include:

- Type of issue (e.g. path traversal, command injection, SSRF, XSS, auth bypass)
- Affected file or endpoint, and the version or commit
- Steps to reproduce, ideally with a minimal request or command
- Impact: what an attacker gains, and what access they need to start

### What to expect

| Stage | Target |
|---|---|
| Acknowledgement | 3 business days |
| Initial triage (severity + scope) | 7 business days |
| Fix released for Critical/High | 30 days, sooner if actively exploited |
| Fix released for Medium/Low | Next scheduled release |

### Disclosure

We follow coordinated disclosure. We ask that you:

1. Give us the agreed window to release a fix before public disclosure.
2. Avoid accessing data that is not yours, and avoid any impact on other users.
3. Credit you in the advisory, unless you prefer to stay anonymous.

We will not pursue legal action against good-faith research that follows this
policy.

## Security Model — Read This Before Exposing YtVault

**Authentication is off by default.** Every endpoint — including destructive
ones like creating sources, triggering downloads, and uploading cookies — is
reachable by anyone who can reach the port.

### Turn on authentication

Set both variables and every route except `/healthz` requires HTTP Basic
credentials:

```yaml
environment:
  YTVAULT_AUTH_USER: admin
  YTVAULT_AUTH_PASSWORD: choose-a-long-random-password
```

The browser handles Basic auth natively: it prompts once and then attaches
the credentials to every request, so the web UI needs no changes. The CLI
reads the same two variables (`YTVAULT_AUTH_USER`, `YTVAULT_AUTH_PASSWORD`).

Both variables must be set — a user with no password would lock everyone
out, so a half-configured instance stays unauthenticated and logs a warning
at startup.

Cross-origin requests that mutate state are rejected, so a page on another
site cannot drive the API using the credentials the browser has cached.

### If you leave it off

- ❌ **Do not expose port 8080 to the public internet.**
- ❌ **Do not run YtVault on a shared host or a network with untrusted clients.**
- ✅ **Do run it behind a reverse proxy** (Caddy, nginx, Traefik) that enforces
  authentication and TLS.
- ✅ **Do bind it to localhost or a private VLAN** when a reverse proxy is not
  available:
  ```bash
  YTVAULT_ADDR=127.0.0.1:8080
  ```
- ✅ **Do put it behind a VPN** (Tailscale, WireGuard) if remote access is needed.

A minimal Caddy front end:

```caddy
ytvault.example.com {
    # Basic auth — replace with a real identity provider for anything serious.
    basicauth {
        admin $2a$14$YOUR_HASH_HERE
    }
    reverse_proxy 127.0.0.1:8080
}
```

### Cookies are credentials

`cookies.txt` contains a **logged-in YouTube session**. Anyone who obtains it can
act as you on YouTube. It is written to the YtVault data volume, is never
committed to git (it is covered by `.gitignore` and `.dockerignore`), and is
uploaded through the web UI. Treat it as a password:

- Never commit it, paste it into an issue, or store it in a config manager that
  is not access-controlled.
- Give the container user read access only, and keep it out of backups you do
  not control.
- Rotate it (re-export from your browser) if it is ever exposed.

## Out of Scope

The following are **not** treated as vulnerabilities:

- **Lack of authentication on the HTTP API when `YTVAULT_AUTH_USER` and
  `YTVAULT_AUTH_PASSWORD` are unset.** See the security model above. Set them
  to enable HTTP Basic auth, or deploy behind a trusted network or an
  authenticating reverse proxy.
- **The upstream `yt-dlp`, FFmpeg, Node.js, and Python toolchain.** These are
  third-party components; report their bugs upstream. Dependabot tracks the
  base images in the Dockerfile; `yt-dlp`, FFmpeg, and Node.js are installed
  unpinned at build time and pick up new versions on each image rebuild.
- **YouTube's own anti-bot / captcha behaviour.** It is a property of the
  upstream service, not a YtVault defect.
- **Denial of service by an operator who already controls the host** (e.g.
  queueing unlimited downloads on purpose).
- Findings that require an attacker to already have filesystem or shell access
  to the host running the container.

## In-Scope

Reports are welcome for anything that lets an **unauthenticated network client**
or a **less-privileged user** escalate beyond what they should reach, including:

- Command injection through the `yt-dlp` wrapper or any task parameter.
- Path traversal in media, poster, or cookie handling (escape from the data
  volume).
- SSRF where a request parameter selects the destination host.
- XSS in the server-rendered UI or its inline JavaScript.
- Authentication bypass when `YTVAULT_AUTH_USER` and `YTVAULT_AUTH_PASSWORD`
  are set.
- Secrets, tokens, or cookies leaked into logs, images, or the git history.

## Project Security Controls

The repository uses GitHub's free-tier security services:

| Control | Purpose |
|---|---|
| CodeQL (`security-extended`) | Static analysis of Go code on every push and PR |
| `govulncheck` | Scans the Go standard library and module graph for known CVEs |
| Dependency Review | Blocks vulnerable additions in PRs (no licence allow/deny list configured) |
| Dependabot | Weekly updates for GitHub Actions, Docker base images, and Go modules |
| Secret scanning + push protection | Enabled in repository settings — blocks and reports committed secrets |
| Private vulnerability reporting | Enabled in repository settings — private, non-public disclosure channel |
| OpenSSF Scorecard | Tracks supply-chain security posture over time |
| Pinned action SHAs | Prevents mutable-tag hijacking of third-party actions |
| Outbound request guard | Resolved-IP validation on user-supplied URLs — no cloud metadata, no private network |
| Branch protection on `main` | Enabled as a repository ruleset — requires PRs and passing status checks |

Security-relevant changes should also be reviewed with the `code-reviewer` and
`security-reviewer` agents before merge, per the project's `CLAUDE.md`.
