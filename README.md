# runbook

Personal command center and runbook engine. Define, manage, and execute multi-step operational runbooks from YAML definitions.

## 📚 Documentation

This README is a feature/reference overview. The **[User Guide in `docs/guide/`](docs/guide/)** is where to look for learning runbook, working through real-world examples, and debugging issues:

- **[Getting Started](docs/guide/01-getting-started.md)** — install + 10-minute hello-world tour
- **[Concepts](docs/guide/02-concepts.md)** — mental model with diagrams: pipeline, variables, step types, error policies, parallel groups, logging, notifications, 1Password
- **[Cookbook](docs/guide/03-cookbook.md)** — recipes for every step type and feature, plus end-to-end runbooks (deploy with rollback, multi-region health probe, scheduled backups)
- **[Reference](docs/guide/04-reference.md)** — exhaustive lookup tables for subcommands, flags, the YAML schema, template variables, env vars, history/log JSON schemas, and on-disk locations
- **[Troubleshooting](docs/guide/05-troubleshooting.md)** — symptom-driven fixes with decision trees for "step misbehaving", "scheduled run not firing", and "wrong variable value"
- **[Running as a Service](docs/guide/06-running-as-a-service.md)** — `runbook cron` deep dive plus Windows Task Scheduler equivalent and log rotation
- **[Thinking in runbook](docs/guide/07-thinking-in-runbook.md)** — design patterns, iterative authoring workflow, migration recipes, and anti-patterns

## Features

- Define runbooks as YAML with named steps, variables, and error handling
- Three step types: local shell commands, SSH remote execution, HTTP requests
- Variable system with three-layer resolution (YAML defaults → env → CLI overrides) plus an `op://` transformation that resolves 1Password secrets after layering
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
- **Shared runbook collections** — `runbook pull <git-url>` clones a repo into your books directory so its YAML files are immediately discoverable by name. Pull updates re-fetch on demand. Single-file pulls work too for one-off downloads
- **Templates** — runbooks placed in `templates/` subdirectories (your own or inside pulled collections) are excluded from normal listing and available as scaffolding via `runbook create --from <template>` and `runbook list --templates`
- Automatic runbook discovery from `~/.runbook/books/`, top-level subdirectories (pulled repos), and the current directory

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

> **First time using runbook?** Once installed, the **[10-minute Getting Started tour](docs/guide/01-getting-started.md)** walks you from your first `hello.yaml` through dry-run, real run, history, and capturing output between steps.

## Usage

```
runbook <command> [flags] [arguments]
```

### Commands

| Command | Description |
|---------|-------------|
| `run <name\|path>` | Execute a runbook |
| `list` | List available runbooks (use `--templates` for templates) |
| `create <name>` | Create a new runbook (use `--from <template>` to copy a template) |
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
| `log reindex` | Rebuild log index from files in the logs directory |
| `log reset-index` | Clear the log index |
| `log update <old> <new>` | Update an index entry to point to a new file path |

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

> **For learning:** the **[Concepts](docs/guide/02-concepts.md)** and **[Cookbook](docs/guide/03-cookbook.md)** pages walk through how steps, variables, captures, conditions, and error policies compose, with worked-through recipes. **[Reference](docs/guide/04-reference.md)** has the complete YAML schema and every flag.

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

log:
  enabled: true
  dir: "~/.runbook/logs/"
  filename: "{name}-{timestamp}"
```

### Logging

Runbooks can automatically save run output to log files. Add a `log` section to the YAML:

| Field | Description |
|-------|-------------|
| `enabled` | `true` to auto-save output after each run (default: `false`) |
| `mode` | `new` creates a file per run (default), `append` accumulates in a single file |
| `dir` | Directory for log files (default: `~/.runbook/logs/`) |
| `filename` | Filename template using `{name}` and `{timestamp}` (default: `{name}-{timestamp}`) |

**New mode** (default): creates `{name}-{timestamp}.log` per run.

**Append mode**: writes to a single `{name}.log` file with `--- run: timestamp ---` separators between runs. Useful for scheduled jobs where a single growing file with rotation is preferred over hundreds of individual files.

```yaml
log:
  enabled: true
  mode: append
```

Logs are indexed in `~/.runbook/logs/index.json` for fast lookup. After log rotation, run `runbook log reindex` to rebuild the index.

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

Variables go through three layers, lowest priority to highest:

1. YAML `default:` value
2. `RUNBOOK_VAR_<NAME>` environment variable (uppercased)
3. `--var key=value` CLI flag

Whatever value comes out of that layering, if it starts with `op://`, is then resolved through the platform keychain (cache hit) or the 1Password CLI (cache miss, cached on success). So `--var token=op://...` and a YAML default of `op://...` both end up calling 1Password — `op://` is a transformation applied *after* layering, not its own priority tier.

If a variable has no value after layering and op:// resolution, runbook prompts (when `prompt:` is set) or errors (when `required: true`).

### 1Password Secrets

Variables and SSH keys with `op://` references (e.g., `op://Vault/Item/field`) are resolved through the 1Password CLI and cached in the system keychain for future runs.

```yaml
variables:
  - name: api_token
    default: "op://Vault/Service/token"
    secret: true

steps:
  - name: Deploy
    type: ssh
    ssh:
      host: "prod-01"
      key_file: "op://Vault/SSH Key/private key"
      command: "deploy.sh"
```

```bash
# Pre-cache all secrets and SSH keys (one-time Touch ID)
runbook auth deploy

# Clear cached secrets and SSH keys
runbook auth --clear deploy
```

Cached SSH keys bypass 1Password's SSH agent entirely — no Touch ID prompts on repeat runs, cron jobs, or Mac app execution. Supports both OpenSSH and PKCS#8 key formats.

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

# List available templates
runbook list --templates

# Create a new runbook from a template
runbook create my-deploy --from ssh-remote

# Create a blank runbook
runbook create my-task

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

> Looking for the full cron walk-through, Windows Task Scheduler equivalent, log rotation patterns, or troubleshooting for "scheduled run isn't firing"? See the **[Running as a Service](docs/guide/06-running-as-a-service.md)** guide page.

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

## Sharing runbooks

runbook is built to make a personal collection of runbooks portable across your machines and shareable with a team. Two mechanisms work together:

### Pulled collections (`runbook pull`)

Any git repo containing YAML runbook files can be pulled into your books directory and used by name:

```sh
# Pull a collection — clones to ~/.runbook/books/<repo-name>/
runbook pull github.com/yourteam/runbooks

# Re-pulling fast-forwards to the latest
runbook pull github.com/yourteam/runbooks

# All runbooks inside the pulled repo are now usable by name
runbook list
runbook run deploy-app

# See what's pulled
runbook pull list

# Remove a collection
runbook pull remove runbooks
```

Discovery walks one level into the books directory, so each pulled repo lives in its own subdirectory but its runbooks surface at the top level by `name:` field. Single-file pulls work too — `runbook pull https://example.com/deploy.yaml` downloads one YAML into `~/.runbook/books/`.

### Templates

A runbook placed in any `templates/` subdirectory (in your own books dir or inside a pulled collection) is **excluded from normal listing** and available as a scaffold for new runbooks:

```sh
# See what templates are available
runbook list --templates

# Scaffold a new runbook from a template
runbook create my-deploy --from ssh-remote
```

`runbook create --from <name>` copies the template, replaces its `name:` field with your new name, writes it to `~/.runbook/books/<new-name>.yaml`, and opens it in `$EDITOR`. Tweak the steps, fill in the variables, and you're off.

### Publishing your own collection

Treat your books directory like any other git repo:

```sh
cd ~/.runbook/books
git init
git add *.yaml templates/
git commit -m "Initial runbook collection"
git remote add origin git@github.com:you/runbooks.git
git push -u origin main
```

Now teammates can `runbook pull github.com/you/runbooks` to get the same set. Updates flow via `git push` from your side and `runbook pull` on the consumer side.

For a full walk-through (including how pulled repos compose with discovery, name collisions, and templates inside collections), see the **[Sharing and templates section in the Cookbook](docs/guide/03-cookbook.md#sharing-and-templates)**.

## Build

```
make build
```

## Test

```
make test
```

## Need help?

Hit a wall? The **[Troubleshooting guide](docs/guide/05-troubleshooting.md)** has decision trees for the three biggest categories (step misbehaving, scheduled run not firing, wrong variable value) plus 15+ symptom→fix entries covering 1Password, SSH auth, cron PATH, append-mode log growth, and more. If your problem isn't there, file an issue at https://github.com/msjurset/runbook/issues with the artifacts listed in the [bug-report checklist](docs/guide/05-troubleshooting.md#filing-a-useful-bug-report).

## License

MIT
