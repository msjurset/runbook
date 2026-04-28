# Running runbook as a Service

Most runbooks are interactive at first — you write them, you run them, you watch the TUI. The next step is unattended scheduled execution: a backup at 3 AM, a health probe every 15 minutes, a weekly maintenance run.

runbook's `cron` subcommand is the supported scheduler on Unix. It manages crontab entries with a labeling convention so it doesn't disturb your other (non-runbook) cron jobs. On Windows, the equivalent is Task Scheduler — runbook doesn't have a direct integration, but the same `runbook run --no-tui --yes <name>` invocation works there too.

- [The cron model](#the-cron-model) — what `cron add` actually installs
- [macOS / Linux setup](#macos--linux-setup) — a complete walk-through
- [Windows: Task Scheduler](#windows-task-scheduler) — the equivalent setup
- [Where logs land](#where-logs-land) — and how to watch them
- [Updating the binary](#updating-the-binary) — what happens to in-flight schedules
- [Removing schedules](#removing-schedules) — clean uninstall
- [Troubleshooting](#troubleshooting) — common scheduling-specific issues

---

## The cron model

`runbook cron add deploy "0 3 * * 0"` installs exactly one line into your user crontab, looking like this:

```
0 3 * * 0 /Users/you/.local/bin/runbook run --no-tui --yes deploy >> /Users/you/.runbook/history/deploy.log 2>&1 # runbook: deploy
```

Three things to notice:

1. **The binary path is absolute, resolved at the moment you ran `cron add`.** Symlinks are followed (so the path is the real binary, not the symlink). If you later move the binary, the cron entry becomes stale — you'll need to remove and re-add it. `make deploy` always installs to `~/.local/bin/runbook`, so the path is stable across rebuilds.
2. **The flags are fixed:** `run --no-tui --yes <name>`. No TUI (no terminal, nothing to render to), and `--yes` auto-accepts confirmation steps. If your runbook has a `confirm:` step, `--yes` makes it pass-through. (Required prompts that have no value still fail — `--yes` doesn't manufacture variable values.)
3. **The trailing `# runbook: <name>` comment** is how runbook identifies its own entries when you later run `cron list` or `cron remove`. It does **not** parse or modify any other crontab lines — manually-installed cron jobs without that marker are invisible to runbook.

`stdout` and `stderr` are appended to `~/.runbook/history/<name>.log`. This is the **history** directory, not the **logs** directory — historical naming. The file accumulates across runs, separated by the runbook's own per-run banner (`Running: <name> — ...`). If the runbook also has `log: enabled: true` in its YAML, a separate file lands in `~/.runbook/logs/`; both files have the same content.

---

## macOS / Linux setup

### Prerequisites

- The `crontab` binary must be on PATH. Standard on every Unix.
- `cron` must be running. `launchd` runs it on macOS; `systemd-cron` or `cronie` or `vixie-cron` runs it on Linux distros (depending on which one).
- On macOS, **the cron daemon needs Full Disk Access** to write to `~/.runbook/`. System Settings → Privacy & Security → Full Disk Access → click `+` → navigate to `/usr/sbin/cron`.

### Schedule a runbook

```sh
# Daily at 3 AM
runbook cron add nightly-backup "0 3 * * *"

# Every 15 minutes
runbook cron add hourly-probe "*/15 * * * *"

# Weekdays at 8 AM
runbook cron add morning-summary "0 8 * * 1-5"
```

After each `cron add`, runbook prints what it installed:

```
✓ Scheduled "nightly-backup": 0 3 * * *
  Logs: /Users/you/.runbook/history/nightly-backup.log
```

### Inspect installed schedules

```sh
runbook cron list
```

Output:

```
RUNBOOK         SCHEDULE     COMMAND
nightly-backup  0 3 * * *    /Users/you/.local/bin/runbook run --no-tui --yes nightly-backup >> /Users/you/.runbook/history/nightly-backup.log 2>&1
hourly-probe    */15 * * * *  /Users/you/.local/bin/runbook run --no-tui --yes hourly-probe >> /Users/you/.runbook/history/hourly-probe.log 2>&1
```

To see the raw crontab (including any non-runbook entries):

```sh
crontab -l
```

### Multiple schedules per runbook

You can call `cron add` more than once with different schedules — they coexist. To run a runbook every weekday morning AND every Sunday afternoon:

```sh
runbook cron add weekly-summary "0 8 * * 1-5"
runbook cron add weekly-summary "0 14 * * 0"
```

Both lines end up in your crontab with the same `# runbook: weekly-summary` marker but different schedule fields. To remove just one:

```sh
runbook cron remove weekly-summary "0 14 * * 0"
```

To remove all schedules for that runbook:

```sh
runbook cron remove weekly-summary
```

### Variables in scheduled runs

`runbook cron add` doesn't carry `--var` flags into the crontab line. If your runbook has required variables, you have two choices:

**Option 1 — bake the variables into the YAML.** Add `default:` values for each variable. This is the right choice for things like target hosts and bucket names that don't change between runs.

**Option 2 — use environment variables.** Add a `cron` line by hand (or edit the one runbook installed) to set `RUNBOOK_VAR_X=value` before invoking the binary:

```
0 3 * * * RUNBOOK_VAR_VERSION=stable /Users/you/.local/bin/runbook run --no-tui --yes deploy >> ... # runbook: deploy
```

Note: editing the line by hand is fine, but be careful — runbook's `cron list` parses by the marker, and a malformed line might still appear in the listing while no longer working.

---

## Windows: Task Scheduler

Windows doesn't have `crontab`. `runbook cron` won't work on Windows. The equivalent is Task Scheduler.

### Install via PowerShell (recommended)

```powershell
$action = New-ScheduledTaskAction `
    -Execute "C:\Users\$env:USERNAME\bin\runbook.exe" `
    -Argument "run --no-tui --yes nightly-backup" `
    -WorkingDirectory "C:\Users\$env:USERNAME"

$trigger = New-ScheduledTaskTrigger -Daily -At 3am

$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -ExecutionTimeLimit ([TimeSpan]::FromHours(2))

$principal = New-ScheduledTaskPrincipal `
    -UserId "$env:USERDOMAIN\$env:USERNAME" `
    -LogonType Interactive

Register-ScheduledTask -TaskName "runbook-nightly-backup" `
    -Action $action -Trigger $trigger -Settings $settings -Principal $principal `
    -Description "Nightly database backup runbook"
```

To capture stdout and stderr to a file (Task Scheduler doesn't redirect by default), use a small wrapper script:

```powershell
# C:\Users\<you>\bin\run-nightly-backup.ps1
$logDir = "$env:USERPROFILE\.runbook\history"
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
& "$env:USERPROFILE\bin\runbook.exe" run --no-tui --yes nightly-backup *>&1 | `
    Tee-Object -FilePath "$logDir\nightly-backup.log" -Append
```

Update the scheduled task's action to invoke `powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File <path-to-script>` instead of `runbook.exe` directly.

### Inspect

```powershell
Get-ScheduledTask -TaskName "runbook-*"
Get-ScheduledTask -TaskName "runbook-nightly-backup" | Get-ScheduledTaskInfo
```

The latter shows last run time, last result code, and next scheduled run.

### Remove

```powershell
Unregister-ScheduledTask -TaskName "runbook-nightly-backup" -Confirm:$false
```

---

## Where logs land

Two places, by launch source:

| Source | Path |
|--------|------|
| Cron-launched (Unix) | `~/.runbook/history/<name>.log` (always) |
| YAML `log: enabled: true` | `~/.runbook/logs/<name>...log` (in addition to the cron log, if both apply) |
| Task Scheduler (Windows) wrapper script | Whatever path you `Tee-Object` to in the wrapper |

The cron-launched log file is **append-only across runs.** Each run's output is preceded by runbook's own start-of-run banner (`Running: <name> — ...`) — so even though the file grows without separators inserted by cron itself, you can still tell where one run ends and the next begins.

### Tail a scheduled runbook's log

```sh
tail -f ~/.runbook/history/nightly-backup.log
```

If you also have `log: enabled: true mode: append` in the YAML:

```sh
tail -f ~/.runbook/logs/nightly-backup.log
```

The two files have the same content. The YAML-driven one inserts explicit `--- run: <ts> ---` separators between runs (the cron-driven one only has runbook's own banner).

### Rotation

Append-mode log files grow unbounded. runbook does not rotate them. Three options:

1. **External rotation.** `logrotate` on Linux, `newsyslog` on macOS. Set up a config that rotates `~/.runbook/{logs,history}/*.log` weekly (or by size).
2. **Self-rotating runbook.** Define a runbook that's itself scheduled to rotate logs:

   ```yaml
   name: rotate-runbook-logs
   description: Compress and archive old runbook logs
   steps:
     - name: Move and compress
       type: shell
       shell:
         command: |
           mkdir -p ~/.runbook/logs/archive
           cd ~/.runbook/logs
           for f in *.log; do
             [ -e "$f" ] || continue
             gzip -c "$f" > "archive/$f-$(date +%Y%m%d-%H%M%S).gz"
             : > "$f"   # truncate, don't delete (preserves inode)
           done
   ```

   Schedule it weekly: `runbook cron add rotate-runbook-logs "0 4 * * 0"`. **After it runs, run `runbook log reindex`** so the Mac app's index file points at the archived `.gz` files instead of the (now-truncated) live logs.

3. **Use a `sortie` rule** if you have sortie installed. `~/.config/sortie/.sortie.yaml` (a per-directory config in `~/.runbook/logs/`) can compress old logs into `archive/` automatically.

### After rotation

The Mac app's History view uses `~/.runbook/logs/index.json` to find log files. Rotation moves files; the index goes stale. Two options:

```sh
runbook log reindex                # Rebuild the entire index from disk
runbook log update <old> <new>     # Update a single entry to point at the new location
```

`log reindex` is the safer choice — it scans both `~/.runbook/logs/` and `~/.runbook/logs/archive/` and rebuilds entries for every `.log` and `.gz` file matching the runbook-name pattern.

---

## Updating the binary

When you run `make deploy` (or `brew upgrade runbook`), the binary at `~/.local/bin/runbook` (or `/opt/homebrew/bin/runbook`) is replaced. The crontab line still references the same path, so the next scheduled run picks up the new binary automatically.

There's no in-flight runbook to worry about — runbook is one-shot per invocation, not a long-running daemon. A schedule that fires while you're deploying might race with the file replacement; in practice, the OS handles atomic file replacement and the running process keeps using the old binary until it exits.

If you've moved the binary elsewhere (e.g., `mv ~/.local/bin/runbook /opt/runbook`), the crontab line points at the old path and the next run will fail with `command not found`. To fix:

```sh
runbook cron list                              # see what's scheduled
runbook cron remove <name>                     # remove the stale entry
runbook cron add <name> "<original schedule>"  # reinstall with the new binary path
```

The `cron add` reinstall picks up wherever the binary is now (via `os.Executable()` followed by symlink resolution).

---

## Removing schedules

```sh
# Remove all schedules for a runbook
runbook cron remove <name>

# Remove one specific schedule (when there are multiple)
runbook cron remove <name> "<schedule>"
```

After removal, `runbook cron list` no longer shows the entry, and `crontab -l` no longer contains the corresponding line. Logs in `~/.runbook/history/<name>.log` are **left alone** — uninstalling the schedule doesn't delete history. Remove logs manually if you want a clean slate.

To remove every runbook-managed entry at once (without disturbing other crontab entries):

```sh
runbook cron list | tail -n +2 | awk '{print $1}' | xargs -n1 runbook cron remove
```

(The tail strips the header row; awk extracts the runbook name.)

---

## Troubleshooting

### Cron entry is installed but the runbook isn't firing

Walk the [decision tree](05-troubleshooting.md#a-scheduled-run-isnt-firing). The most common cause on macOS is missing Full Disk Access for `cron`.

To verify cron is even alive:

```sh
# macOS / Linux — see when cron last ran any user job
ls -la ~/.runbook/history/
# If timestamps update at the schedule's tick, cron is firing
```

A simpler smoke test — install a one-minute schedule and watch the log:

```sh
runbook cron add hello "* * * * *"     # every minute
sleep 70
runbook cron remove hello
tail ~/.runbook/history/hello.log
```

If `hello.log` shows entries every minute, cron is working. If not, the issue is at the cron-daemon level, not runbook.

### Cron-launched run fails with "no ssh authentication methods"

Cron's environment doesn't include your interactive `SSH_AUTH_SOCK`. Either:

- **Set `IdentityAgent` in `~/.ssh/config`** for the host(s) the runbook connects to. runbook reads the config and uses the agent socket from there.
- **Pre-cache an SSH key** stored in 1Password via `runbook auth <name>`. The cached key is read from the system keychain on every run and doesn't need an agent.
- **Use `key_file:` with a path to an unencrypted key file.** Simple but less secure than the keychain-cached `op://` approach.

### Cron-launched run fails with "command not found"

Cron's PATH is heavily stripped. runbook augments it with `~/.local/bin`, `/opt/homebrew/bin`, `/opt/homebrew/sbin`, `/usr/local/bin`, and `~/go/bin` — but only those. Anything elsewhere on your interactive PATH won't be visible.

See [Troubleshooting › Cron-launched run can't find a binary](05-troubleshooting.md#cron-launched-run-cant-find-a-binary) for the fix patterns (absolute paths, wrapper scripts, or symlinks).

### `runbook cron list` shows entries that aren't in the actual crontab

Possible if you edited the crontab manually with `crontab -e` and removed lines without the `# runbook: <name>` marker. Runbook's `cron list` re-reads the crontab on each call, so this shouldn't happen — but if you're seeing stale data, run `crontab -l` directly to see ground truth.

### I want to schedule a runbook but cron is disabled on this system

Some sandboxed environments (containers, locked-down corporate machines) disable cron. Your options:

- **Use a host-level scheduler** (the host's cron, a CI scheduler, GitHub Actions cron triggers).
- **Run a tiny supervisor.** `while sleep 3600; do runbook run --no-tui --yes hourly-probe; done` in a `screen` or `tmux` session is crude but works.
- **Use `systemd` user timers** on Linux. Define `~/.config/systemd/user/runbook-X.service` and `~/.config/systemd/user/runbook-X.timer` and enable with `systemctl --user enable --now runbook-X.timer`. This bypasses `cron` entirely.

`runbook cron` is intentionally a thin wrapper over `crontab`. If `crontab` doesn't work, neither does `runbook cron` — work around by going one level deeper to whatever scheduler the platform offers.

---

## Where to go next

- [Cookbook → Hourly health check via cron](03-cookbook.md#hourly-health-check-via-cron) — concrete schedule recipes.
- [Troubleshooting](05-troubleshooting.md) — symptom-driven fixes, including a dedicated decision tree for scheduled runs.
- [Concepts → Logging](02-concepts.md#logging) — the architecture of where logs land per launch source.
