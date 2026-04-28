# Getting Started

A ten-minute tour. By the end you'll have runbook installed, a hello-world YAML in `~/.runbook/books/`, a clean dry-run, a real run, history, and a second step that captures output and feeds it to a third step.

If anything feels magical along the way, skip ahead to [Concepts](02-concepts.md) afterwards — that page explains the moving parts.

## 1. Install

### Homebrew (macOS, Linux)

```sh
brew install msjurset/tap/runbook
runbook --version
```

That places `runbook` on your `PATH`, installs the man page, and registers zsh completions if you have `compinit` configured.

### From source

If you have the repo checked out (`~/workspace/go/runbook` or wherever):

```sh
make deploy
runbook --version
```

`make deploy` builds the binary, installs it to `~/.local/bin/`, drops the man page into your local manpath, and writes the zsh completion script. Make sure `~/.local/bin/` is on your `PATH`.

### What gets created

The first time you run anything, runbook creates these directories on demand:

| Path | Purpose |
|------|---------|
| `~/.runbook/books/` | Where it looks up runbooks by name |
| `~/.runbook/history/` | One JSON file per execution + cron stdout logs |
| `~/.runbook/logs/` | Run output logs (only when a runbook has `log:` configured) |

You don't have to create these manually — the binary does it the first time it needs them.

## 2. Write your first runbook

Create `~/.runbook/books/hello.yaml`:

```yaml
name: hello
description: Say hi from runbook
steps:
  - name: Greet
    type: shell
    shell:
      command: echo "Hello from runbook!"
```

Or use the scaffolding subcommand:

```sh
runbook create hello
```

That writes a starter file with a single shell step and opens it in `$EDITOR` if set. Replace the placeholder command with `echo "Hello from runbook!"`.

## 3. Inspect before running

```sh
runbook list
```

You should see `hello` in the table. To inspect what's inside without running anything:

```sh
runbook show hello
```

That prints the runbook's name, description, file path, variables (none yet), and an outline of every step with its type and configured options.

To validate the YAML structurally without executing:

```sh
runbook validate hello
```

You'll get `✓ hello is valid (1 steps)` or a structured error pointing at the line that's broken.

## 4. Dry-run

`runbook run --dry-run` parses the runbook, resolves variables, and prints what *would* happen — without running any commands, making any HTTP calls, or opening any SSH connections.

```sh
runbook run --dry-run hello
```

Output:

```
Runbook: hello
  Say hi from runbook

Variables:

Steps:
  1. [shell] Greet
```

The variables block is empty because `hello` has no `variables:`. We'll add one in step 8.

## 5. Run for real

```sh
runbook run hello
```

When `stdout` is a TTY, runbook launches a TUI showing each step's status, output, and timing as it runs. Press `q` to quit when the run finishes (it auto-completes with a summary).

To skip the TUI and just stream plain output (handy in pipelines, scripts, or logs):

```sh
runbook run --no-tui hello
```

You'll see:

```
Running: hello — Say hi from runbook (1 steps)
▸ Step 1: Greet
  │ Hello from runbook!
  ✓ done
```

## 6. Inspect history

Every completed run leaves a JSON record in `~/.runbook/history/`. To see the most recent runs in a table:

```sh
runbook history
```

Output:

```
TIME                 RUNBOOK  STATUS  STEPS  DURATION
2026-04-28 14:30:21  hello    ✓       1      0.1s
```

Filter by name with `--runbook hello`, limit with `-n 5`. The records themselves are JSON files you can grep or feed to `jq`:

```sh
ls ~/.runbook/history/
jq . ~/.runbook/history/2026-04-28_14-30-21_hello.json
```

The schema is in [Reference › History records](04-reference.md#history-record-schema).

## 7. Add a variable

Edit `~/.runbook/books/hello.yaml` to take a name as input:

```yaml
name: hello
description: Say hi from runbook
variables:
  - name: who
    default: world
steps:
  - name: Greet
    type: shell
    shell:
      command: 'echo "Hello, {{.who}}!"'
```

Now:

```sh
runbook run hello                         # → "Hello, world!"
runbook run --var who=Mark hello          # → "Hello, Mark!"
RUNBOOK_VAR_WHO=team runbook run hello    # → "Hello, team!"
```

Variables resolve in priority order: **YAML default → `RUNBOOK_VAR_<NAME>` env → `--var` CLI flag** — last one wins. Full details in [Concepts › Variable resolution](02-concepts.md#variable-resolution).

## 8. Capture output between steps

Replace the file with this two-step version:

```yaml
name: hello
description: Show the kernel and greet the user
steps:
  - name: Read kernel version
    type: shell
    shell:
      command: uname -r
    capture: kernel

  - name: Greet
    type: shell
    shell:
      command: 'echo "Hello from kernel {{.kernel}}"'
```

Run it:

```sh
runbook run hello
```

You'll see step 1 print the kernel version, step 2 print `Hello from kernel 25.4.0` (or whatever your kernel is). The `capture: kernel` field stores step 1's stdout (trimmed) into a variable named `kernel`, which step 2's template references via `{{.kernel}}`.

Capture is the primary way information flows between steps. Anything stdout-producing — `shell`, `ssh`, or HTTP response bodies — can be captured. More patterns in the [Cookbook](03-cookbook.md).

## 9. Where to next

You've now seen 80% of what runbook does day-to-day. The remaining 20% is choosing the right step type for the job, wiring up secrets, scheduling, and notifications:

- [Concepts](02-concepts.md) — the mental model. Read this once; everything else clicks faster afterwards.
- [Cookbook](03-cookbook.md) — concrete runbooks for common tasks: SSH deploys, HTTP health checks, parallel probes, retries, conditional steps, 1Password secrets.
- [Reference](04-reference.md) — quick-lookup tables for subcommands, flags, YAML fields, and on-disk locations.
- [Running as a Service](06-running-as-a-service.md) — schedule a runbook to run every Sunday at 3 AM via `runbook cron`.
- [Troubleshooting](05-troubleshooting.md) — when a step does something you didn't expect, this is the first stop.
