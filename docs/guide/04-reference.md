# Reference

Quick-lookup tables. For exhaustive flag documentation, use `runbook <subcommand> --help` or read the [man page](../../runbook.1) (`runbook man | man -l -`).

## Subcommands

| Subcommand | Purpose |
|------------|---------|
| `runbook run <name\|path>` | Execute a runbook |
| `runbook list` | List discovered runbooks |
| `runbook list --templates` | List available templates instead of runbooks |
| `runbook show <name\|path>` | Print runbook details (variables, steps, options, notify, log) |
| `runbook validate <name\|path>` | Parse and validate a runbook without running it |
| `runbook create <name>` | Create a new runbook (use `--from <template>` to copy a template) |
| `runbook history` | Show recent runs in a table |
| `runbook auth <name>` | Pre-resolve and cache `op://` secrets in the keychain |
| `runbook auth --clear <name>` | Remove a runbook's cached secrets from the keychain |
| `runbook cron add <name> <schedule>` | Schedule a runbook via crontab |
| `runbook cron list` | List runbook-managed crontab entries |
| `runbook cron remove <name> [schedule]` | Remove one schedule, or all schedules for a runbook |
| `runbook pull <repo-url\|file-url>` | Clone a git repo or download a single YAML into the books directory |
| `runbook pull list` | List pulled repos |
| `runbook pull remove <name>` | Delete a pulled repo |
| `runbook log reindex` | Rebuild the log index from disk |
| `runbook log reset-index` | Clear the log index |
| `runbook log update <old> <new>` | Update one index entry to point at a new path (post-rotation) |
| `runbook man` | Print the roff-formatted man page to stdout |
| `runbook --version` | Print the binary's version |

## Global flags

These flags work on every subcommand.

| Flag | Default | Purpose |
|------|---------|---------|
| `--dir <path>` | `~/.runbook/books/` | Override the runbook discovery directory |
| `-h`, `--help` | — | Per-subcommand help |
| `--version` | — | Print version |

## Per-subcommand flags

The flags you'll reach for most often.

| Subcommand | Flag | Default | Purpose |
|------------|------|---------|---------|
| `run` | `--var key=value` | — | Set a variable; repeatable |
| `run` | `--dry-run` | `false` | Print the resolved plan without executing |
| `run` | `--yes` | `false` | Auto-accept all `confirm:` prompts |
| `run` | `--no-tui` | `false` | Disable the TUI; stream plain output |
| `history` | `-n`, `--limit <int>` | `20` | Max records to print |
| `history` | `--runbook <name>` | — | Filter records to a single runbook name |
| `auth` | `--clear` | `false` | Remove cached secrets instead of resolving |
| `list` | `--templates` | `false` | List templates instead of runbooks |
| `create` | `--from <name>` | — | Initialize from a named template |

## Runbook YAML schema

Every field is optional unless marked `REQUIRED`.

```yaml
# REQUIRED — the canonical name. This is what `runbook run X` matches against.
name: deploy

# Free text shown in `runbook show`, `runbook list`, the TUI, and notifications.
description: Deploy the application

# Variables the runbook accepts. See "Variable definitions" below.
variables:
  - name: version
    required: true
    prompt: "Version to deploy"
    default: ""
    secret: false

# REQUIRED — at least one step. See "Step definitions" below.
steps:
  - name: Build
    type: shell
    shell:
      command: ./build.sh

# Optional notification config — see "Notify schema" below.
notify:
  on: failure
  slack:
    webhook: 'op://Vault/Slack/webhook'
  desktop: true
  email:
    to: ops@example.com
    from: runbook@example.com
    host: smtp.example.com:587
    username: runbook@example.com
    password: 'op://Vault/SMTP/password'

# Optional log config — see "Log schema" below.
log:
  enabled: true
  mode: new
  dir: ~/.runbook/logs/
  filename: '{name}-{timestamp}'
```

### Variable definitions

```yaml
variables:
  - name: version           # REQUIRED — variable identifier; lowercase + underscores recommended
    default: ''             # YAML default value; lowest priority in resolution
    required: false         # If true, error when unresolved (unless prompt: is also set)
    prompt: ''              # Interactive prompt text; if set, asks user when value is missing
    secret: false           # If true, mask in TUI variable summary
```

Resolution priority, lowest to highest: `default` → env (`RUNBOOK_VAR_<NAME>`) → CLI (`--var name=value`). The op:// transformation runs *after* layering — whatever value the layering pass produces, if it starts with `op://`, is then resolved through the keychain or 1Password CLI.

### Step definitions

```yaml
steps:
  - name: Run tests              # REQUIRED — used in TUI, history, log markers
    type: shell                  # one of: shell, ssh, http (omit for confirm-only)

    # Type-specific block; exactly one of shell/ssh/http applies.
    shell:
      command: 'go test ./...'   # REQUIRED for shell — template-expanded
      dir: ''                    # Working directory; template-expanded
    ssh:
      host: prod-web-01          # REQUIRED for ssh — template-expanded; supports ~/.ssh/config alias
      user: deploy               # Defaults to $USER if empty
      port: 22                   # Defaults to 22
      key_file: ''               # Path or op:// reference; defaults to ~/.ssh/id_*
      command: 'systemctl restart app'  # REQUIRED for ssh — template-expanded
      agent_auth: false          # If true, use SSH_AUTH_SOCK
    http:
      method: GET                # Defaults to GET
      url: 'https://...'         # REQUIRED for http — template-expanded
      headers:                   # Map of header → value; values template-expanded
        Authorization: 'Bearer {{.api_token}}'
      body: ''                   # Template-expanded request body

    # Cross-cutting features (apply to any step type)
    capture: ''                  # Variable name to store stdout/body into; trimmed
    condition: ''                # Skip step unless template renders to "true"
    confirm: ''                  # Prompt before running; template-expanded
    on_error: abort              # one of: abort (default), continue, retry
    retries: 0                   # Max retry count for on_error: retry; total attempts = retries+1
    timeout: 0s                  # Per-step duration: 30s, 5m, 1h30m, etc.
    parallel: false              # Group with adjacent parallel: true steps for concurrent execution
```

### Notify schema

```yaml
notify:
  on: always              # always (default), failure, success
  desktop: false          # Native OS notification (osascript / notify-send / BurntToast)
  macos: false            # DEPRECATED alias for desktop:
  slack:
    webhook: ''           # REQUIRED if slack: present — URL or op:// ref
    channel: ''           # Optional channel override
  email:
    to: ''                # REQUIRED — recipient
    from: ''              # REQUIRED — sender
    host: ''              # REQUIRED — host:port (e.g. smtp.gmail.com:587)
    username: ''          # Optional SMTP PLAIN auth
    password: ''          # Optional; supports op:// references
```

### Log schema

```yaml
log:
  enabled: false                       # If false, no log file is written
  mode: new                            # new (default; one file per run) or append
  dir: ~/.runbook/logs/                # Output directory
  filename: '{name}-{timestamp}'       # File-stem template; only used in new mode
                                       # Available placeholders: {name}, {timestamp}
                                       # ".log" extension is appended automatically
```

In `append` mode, the filename is always `<name>.log` (the `filename:` template is ignored). Each run prepends `\n--- run: <ISO ts> ---\n` before its output.

## Template variables

Available in any template-expanded field (`command`, `url`, `body`, header values, `host`, `user`, `dir`, `confirm`, `condition`):

| Reference | Source |
|-----------|--------|
| `{{.foo}}` | Variable named `foo` (declared in `variables:` or captured from a previous step) |

Runbook does not register any custom template functions. The standard Go [`text/template`](https://pkg.go.dev/text/template) operators all work:

| Operator | Example |
|----------|---------|
| `or` | `{{or .user "deploy"}}` |
| `if` / `else` | `{{if eq .env "prod"}}--strict {{end}}` |
| `eq`, `ne`, `lt`, `le`, `gt`, `ge` | `{{if gt .retries 5}}…{{end}}` |
| `and`, `or`, `not` | `{{if and .a .b}}…{{end}}` |
| `with` | `{{with .vendor}}{{.}}-{{end}}{{.name}}` |
| Whitespace trim | `{{- .foo -}}` |

Templates use `Option("missingkey=zero")` — referencing an unset variable produces the empty string, not an error. Typos fail silently; pair with `runbook validate` and `--dry-run` to catch them early.

## Environment variables

Runbook reads these from the environment:

| Var | Effect |
|-----|--------|
| `RUNBOOK_VAR_<NAME>` | Sets the value of variable `<name>` (uppercased). Beats YAML defaults; loses to `--var` |
| `EDITOR` | Used by `runbook create` to open the new file after writing |
| `SSH_AUTH_SOCK` | Used by `ssh` steps when `agent_auth: true` |
| `USER` | Used as the default SSH user when neither `ssh.user:` nor `~/.ssh/config` provides one |
| `HOME` | Used for `~/...` expansion and default SSH key locations |
| `PATH` | Augmented (not overridden) for `shell:` steps with `~/.local/bin`, `/opt/homebrew/{bin,sbin}`, `/usr/local/bin`, `~/go/bin` |

Inside a `shell:` step, the child process additionally sees:

| Var | Source |
|-----|--------|
| `RUNBOOK_<name>` | Every resolved variable, with its YAML name preserved (case-sensitive). e.g. variable `host` → `$RUNBOOK_host`. |

## Cron schedule syntax

Standard 5-field cron format:

```
┌───────── minute (0-59)
│ ┌─────── hour (0-23)
│ │ ┌───── day of month (1-31)
│ │ │ ┌─── month (1-12)
│ │ │ │ ┌─ day of week (0-6, Sun=0)
* * * * *
```

| Symbol | Meaning |
|--------|---------|
| `*` | Every value |
| `,` | List (`1,3,5`) |
| `-` | Range (`1-5`) |
| `/` | Step interval (`*/15`, `1/15`) |

When both day-of-month and day-of-week are set, the schedule fires when **either** matches (OR, not AND).

Common patterns:

| Schedule | Description |
|----------|-------------|
| `0 9 * * *` | Every day at 9:00 AM |
| `0 3 * * 0` | Every Sunday at 3:00 AM |
| `*/15 * * * *` | Every 15 minutes |
| `0 8 * * 1-5` | Weekdays at 8:00 AM |
| `0 0 1 * *` | First of every month at midnight |
| `0 9 1/15 * 6` | Every 15 days from the 1st, plus every Saturday, at 9:00 AM |

The full installed crontab line for a runbook is:

```
<schedule> <bin>/runbook run --no-tui --yes <name> >> <history-dir>/<name>.log 2>&1 # runbook: <name>
```

The trailing `# runbook: <name>` marker is how `runbook cron list` and `runbook cron remove` identify managed entries — they leave non-runbook crontab lines alone.

## Step status reference

Status values that appear in the TUI, in `runbook history` output, and in JSON history records:

| Status | Meaning |
|--------|---------|
| `success` | Step completed without error |
| `failed` | Step errored; `on_error` controls whether the runbook continues |
| `skipped` | `condition:` was false, `confirm:` was declined, or an earlier abort propagated |
| `retrying` | Transient state during retry attempts (between failed attempt N and attempt N+1) |
| `running` | Transient state during execution |
| `pending` | Transient state before the step is reached |

Only `success`, `failed`, and `skipped` appear in persisted history records; the others are TUI-internal.

## On-disk locations

All paths are user-relative.

| Path | Purpose |
|------|---------|
| `~/.runbook/books/` | Default discovery directory for runbooks |
| `~/.runbook/books/<name>.yaml` | A runbook (top-level discovery) |
| `~/.runbook/books/<repo>/` | Pulled git repos (recursive discovery one level deep) |
| `~/.runbook/books/**/templates/` | Template runbooks; excluded from normal listing |
| `~/.runbook/history/` | One JSON file per run; also where cron-launched runs write stdout |
| `~/.runbook/history/<date>_<time>_<name>.json` | A history record |
| `~/.runbook/history/<name>.log` | Cron-launched runs' captured stdout (append-style) |
| `~/.runbook/logs/` | YAML-driven `log:` output |
| `~/.runbook/logs/index.json` | Log index used by the Mac app's History view |
| `~/.runbook/logs/archive/` | Convention for rotated logs (referenced by the index after rotation) |
| `~/.ssh/config` | Read for `Host`, `Hostname`, `User`, `Port`, `IdentityFile`, `IdentityAgent` |
| `~/.ssh/known_hosts` | Used for SSH host key verification |
| `~/.ssh/id_ed25519`, `id_rsa`, `id_ecdsa` | Default SSH key fallbacks |

## History record schema

Each `~/.runbook/history/*.json` file is a single Record:

```jsonc
{
  "runbook_name": "deploy",                                // string — the canonical name from YAML
  "file_path": "/Users/me/.runbook/books/deploy.yaml",     // string — absolute path on disk
  "started_at": "2026-04-28T14:30:21.123456Z",             // RFC 3339 with subsecond precision (UTC)
  "duration": "12.4s",                                      // Go-style duration string, rounded to 100ms
  "success": true,                                          // bool — overall run outcome
  "step_count": 5,                                          // int — number of steps in this run
  "steps": [                                                // array — one entry per step
    {
      "name": "Run tests",                                  // string — step's name from YAML
      "type": "shell",                                      // one of: shell, ssh, http; "" for confirm-only
      "status": "success",                                  // success | failed | skipped
      "duration": "2.1s",                                   // Go duration, rounded
      "error": "..."                                        // string — only present when status=failed
    }
  ]
}
```

Common `jq` queries:

```sh
# Last 5 failed runs across all runbooks
jq -r 'select(.success | not) | "\(.started_at) \(.runbook_name)"' ~/.runbook/history/*.json | tail -5

# Runs of "deploy" longer than 1 minute
jq -r 'select(.runbook_name == "deploy" and (.duration | endswith("s") and (.[:-1] | tonumber) > 60))' ~/.runbook/history/*.json

# All step errors in the last week (with file timestamps as the time axis)
find ~/.runbook/history/ -name '*.json' -mtime -7 -print0 | \
  xargs -0 -I{} jq -r '.steps[] | select(.error) | "\(.name): \(.error)"' {}
```

## Log file format markers

When `log: enabled: true` is set, the per-runbook log file has a structured shape:

| Marker | Form | Where |
|--------|------|-------|
| Run separator (append mode) | `\n--- run: 2026-04-28T14:30:21 ---\n` | Once at the top of each run's section in append-mode files |
| Run banner | `Running: <name> — <description> (<n> steps)` | Once at the top of each run, regardless of mode |
| Step header | `▸ Step <N>: <name>` | Once at the top of each step |
| Step body line | `  │ <line>` | One per output line |
| Step result | `  ✓ done` / `  ✗ failed` / `  ⊘ skipped` | One per step |

The Mac app's History view parses these markers to extract per-step output for old runs. If you write a custom log parser, those are the patterns to match.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success — every step ran or was deliberately skipped, runbook reports success |
| `1` | Failure — at least one step failed and propagated, or validation failed, or the runbook wasn't found |

`runbook` follows the standard Unix convention: any non-zero exit indicates failure. Pipelines and shell scripts can use `&&` / `||` to chain runs.

## Where to go next

- [Getting Started](01-getting-started.md) — install and first-run walkthrough
- [Concepts](02-concepts.md) — mental model behind every field on this page
- [Cookbook](03-cookbook.md) — applied recipes for everything in the schema
- [Troubleshooting](05-troubleshooting.md) — symptom-driven fixes
- [`runbook.1`](../../runbook.1) — the man page
