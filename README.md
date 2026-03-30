# runbook

Personal command center and runbook engine. Define, manage, and execute multi-step operational runbooks from YAML definitions.

## Features

- Define runbooks as YAML with named steps, variables, and error handling
- Three step types: local shell commands, SSH remote execution, HTTP requests
- Variable system with four-layer resolution: YAML defaults, environment variables, CLI overrides, and 1Password secrets
- 1Password integration — `op://` variable references are resolved via the 1Password CLI and cached in the system keychain (macOS, Linux, Windows)
- Go template expansion in all string fields (commands, URLs, headers, bodies)
- Output capture between steps — pipe the result of one step into later steps
- Per-step error policies: abort, continue, or retry with configurable attempts
- Per-step timeouts and conditional execution
- Parallel step execution — consecutive steps marked `parallel: true` run concurrently
- Interactive confirmation prompts before sensitive steps
- TUI mode with live step tracking, output viewport, and interactive controls
- Dry-run mode to preview execution without running anything
- Run history with per-step timing and status
- Cron scheduling — manage crontab entries for unattended runbook execution with log capture
- Pull and share runbooks from git repos or URLs
- Automatic runbook discovery from `~/.runbook/books/`, subdirectories (pulled repos), and the current directory

## Install

### Homebrew

```sh
brew install msjurset/tap/runbook
```

### From source

```sh
make deploy
```

This builds the binary, installs it to `~/.local/bin/`, installs the man page, and sets up zsh completions.

## Usage

```
runbook <command> [flags] [arguments]
```

### Commands

| Command | Description |
|---------|-------------|
| `run <name\|path>` | Execute a runbook |
| `list` | List available runbooks |
| `show <name\|path>` | Show runbook details |
| `validate <name\|path>` | Validate a runbook without executing |
| `history` | Show runbook execution history |
| `auth <name\|path>` | Pre-resolve and cache 1Password secrets |
| `cron add <name> <schedule>` | Schedule a runbook via crontab |
| `cron list` | List all scheduled runbooks |
| `cron remove <name>` | Remove a scheduled runbook |
| `pull <repo-url\|file-url>` | Pull runbooks from a git repo or URL |
| `pull list` | List pulled repositories |
| `pull remove <name>` | Remove a pulled repository |

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `~/.runbook/books/` | Override the runbook directory |

### Run Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--var` | — | Set variable (key=value), repeatable |
| `--dry-run` | `false` | Validate and show steps without executing |
| `--yes` | `false` | Auto-confirm all prompts |
| `--no-tui` | `false` | Disable TUI mode |

### History Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --limit` | `20` | Max records to show |
| `--runbook` | — | Filter by runbook name |

### Auth Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--clear` | `false` | Remove cached secrets instead of resolving |

### Runbook YAML Format

```yaml
name: deploy-app
description: Deploy the application to production

variables:
  - name: version
    required: true
    prompt: "Enter version to deploy"
  - name: host
    default: "prod-01.internal"
  - name: api_token
    default: "op://Vault/Deploy/token"
    secret: true

steps:
  - name: Run tests
    type: shell
    shell:
      command: "go test ./..."
    timeout: 5m
    on_error: abort

  - name: Health check
    type: http
    http:
      method: GET
      url: "https://{{.host}}:8080/healthz"
      headers:
        Authorization: "Bearer {{.api_token}}"
    capture: health_status
    on_error: continue

  - name: Confirm deployment
    confirm: "Deploy {{.version}} to {{.host}}?"

  - name: Deploy
    type: ssh
    ssh:
      host: "{{.host}}"
      user: deploy
      agent_auth: true
      command: "sudo systemctl restart app"
    timeout: 30s

notify:
  on: failure
  slack:
    webhook: "op://Vault/Slack/webhook"
  desktop: true
```

### Notifications

Runbooks can send notifications after completion. Add a `notify` section to the YAML:

| Field | Description |
|-------|-------------|
| `on` | When to notify: `always` (default), `failure`, `success` |
| `slack.webhook` | Slack incoming webhook URL (supports `op://` references) |
| `slack.channel` | Override default webhook channel |
| `desktop` | `true` to show a native OS notification (macOS, Linux, Windows) |
| `email.to` | Recipient email address |
| `email.from` | Sender email address |
| `email.host` | SMTP server as `host:port` |
| `email.username` | SMTP username (optional) |
| `email.password` | SMTP password (supports `op://` references) |

### Variable Resolution

Variables are resolved in priority order (highest wins):

1. `op://` references resolved via 1Password CLI + system keychain cache
2. `--var key=value` CLI flag
3. `RUNBOOK_VAR_<NAME>` environment variable
4. YAML `default` value

### 1Password Secrets

Variables with `op://` references (e.g., `op://Vault/Item/field`) are automatically resolved through the 1Password CLI and cached in the system keychain for future runs.

```bash
# Pre-cache all secrets (avoids 1Password prompts during execution)
runbook auth deploy

# Clear cached secrets
runbook auth --clear deploy
```

Supported keychains: macOS Keychain, GNOME Secret Service (Linux), Windows Credential Manager.

### Examples

```bash
# Run a runbook from the current directory
runbook run deploy.yaml

# Run by name from ~/.runbook/books/ with a variable override
runbook run --var version=1.2.3 deploy

# Auto-confirm all prompts
runbook run --yes --var host=web01 restart-services

# Preview what would run without executing
runbook run --dry-run deploy.yaml

# Plain CLI mode (no TUI)
runbook run --no-tui deploy.yaml

# List all available runbooks
runbook list

# Inspect a runbook's structure
runbook show deploy

# Validate without running
runbook validate deploy.yaml

# View run history
runbook history

# View last 5 runs of a specific runbook
runbook history -n 5 --runbook deploy

# Pre-cache 1Password secrets
runbook auth deploy

# Schedule a runbook to run every Sunday at 3am
runbook cron add update-pihole "0 3 * * 0"

# List scheduled runbooks
runbook cron list

# Remove a schedule
runbook cron remove update-pihole
```

### Cron Schedule Syntax

```
┌───────── minute (0-59)
│ ┌─────── hour (0-23)
│ │ ┌───── day of month (1-31)
│ │ │ ┌─── month (1-12)
│ │ │ │ ┌─ day of week (0-6, Sun=0)
* * * * *
```

| Symbol | Meaning | Example |
|--------|---------|---------|
| `*` | every value | `* * * * *` — every minute |
| `,` | list of values | `0 9,17 * * *` — at 9 AM and 5 PM |
| `-` | range | `0 9 * * 1-5` — weekdays at 9 AM |
| `/` | step interval | `*/15 * * * *` — every 15 minutes |

When both day-of-month and day-of-week are specified, cron fires when **either** matches (OR, not AND).

**Examples:**

| Schedule | Description |
|----------|-------------|
| `0 9 * * *` | Every day at 9:00 AM |
| `0 3 * * 0` | Every Sunday at 3:00 AM |
| `*/15 * * * *` | Every 15 minutes |
| `0 8 * * 1-5` | Weekdays at 8:00 AM |
| `0 0 1 * *` | 1st of every month at midnight |
| `0 9 1/15 * 6` | Every 15 days from the 1st, and on Saturdays, at 9:00 AM |

```sh

# Pull runbooks from a git repo
runbook pull github.com/user/runbooks

# Download a single runbook file
runbook pull https://example.com/deploy.yaml

# List pulled repos
runbook pull list

# Remove a pulled repo
runbook pull remove runbooks

# Set variables via environment
RUNBOOK_VAR_HOST=staging-01 runbook run deploy
```

## Build

```
make build
```

## Test

```
make test
```

## License

MIT
