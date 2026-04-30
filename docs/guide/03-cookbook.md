# Cookbook

Working recipes for every step type and feature, plus end-to-end runbooks for common operational tasks. Each recipe follows the same shape:

- **When to reach for this** — the situation it solves
- **YAML** — the runbook (or step), ready to paste
- **What happens** — concrete walk-through
- **Variations** — adaptations for related cases
- **Gotchas** — failure modes specific to this pattern
- **Notes** — undo, scheduling, or output considerations

Every recipe was checked against current runbook (`runbook validate <file>`). Run that command after pasting and editing — it catches missing fields, unknown step types, and `on_error: retry` without `retries:` before any side effect happens.

---

## Table of contents

- **Shell** — [hello with vars](#hello-with-variables), [capture between steps](#capture-output-and-feed-it-forward), [working directory](#shell-step-with-working-directory), [pipe through jq](#parse-json-with-jq), [conditional cleanup](#conditional-cleanup-step)
- **HTTP** — [GET healthcheck](#http-get-healthcheck), [POST JSON](#post-json-payload), [auth header](#http-with-bearer-token), [retry on 5xx](#retry-on-transient-http-failure)
- **SSH** — [agent auth](#ssh-with-agent-auth), [key file](#ssh-with-explicit-key-file), [op:// key](#ssh-with-1password-stored-key), [via ~/.ssh/config alias](#ssh-via-ssh-config-alias)
- **Variables and templates** — [required + prompt](#prompt-for-a-required-variable), [env override](#env-override-for-ci), [op:// secret](#1password-secret-as-a-variable), [optional with fallback](#optional-variable-with-fallback)
- **Flow control** — [confirm before destructive](#confirm-before-destructive-step), [parallel probes](#parallel-health-probes), [retry with backoff](#retry-with-exponential-backoff), [conditional step](#conditional-step)
- **Logging and notifications** — [append-mode log](#append-mode-log-for-scheduled-runs), [Slack on failure](#slack-on-failure-only), [desktop confirmation](#desktop-notification-on-success), [email digest](#email-digest-with-per-step-status)
- **Scheduled runs** — [hourly health check](#hourly-health-check-via-cron), [nightly backup with rotation hint](#nightly-backup)
- **Sharing and templates** — [pull a shared collection](#pull-a-shared-runbook-collection), [publish your own](#publish-your-own-runbook-collection), [scaffold from template](#scaffold-a-new-runbook-from-a-template), [author a template](#author-a-template-for-a-shared-collection), [keep collections fresh](#keep-pulled-collections-fresh)
- **Backups** — [browse and restore](#browse-and-restore-backups), [scheduled snapshot via goback](#scheduled-snapshot-via-goback), [prune old backups](#prune-old-backups)
- **End-to-end runbooks** — [deploy with rollback gate](#deploy-with-rollback-gate), [multi-region health probe](#multi-region-health-probe), [pull updates and notify](#pull-updates-and-notify)

---

## Shell recipes

### Hello with variables

**When to reach for this:** the smallest useful runbook — proves install works, shows variable substitution, gives you something to point `runbook run` at.

**YAML:**

```yaml
name: hello
description: Greet someone
variables:
  - name: who
    default: world
steps:
  - name: Greet
    type: shell
    shell:
      command: 'echo "Hello, {{.who}}!"'
```

**What happens:** `runbook run hello` prints `Hello, world!`. `runbook run --var who=Mark hello` prints `Hello, Mark!`. The `{{.who}}` template references the resolved variable map.

**Variations:**

- **Required input:** drop the `default:` and add `required: true` plus `prompt: "Who to greet?"`. The runbook will prompt before executing if `--var who=...` isn't supplied.
- **Env override:** `RUNBOOK_VAR_WHO=team runbook run hello`. The env name is uppercased.

**Notes:** if the value contains special characters, prefer `$RUNBOOK_who` over `{{.who}}` — the env var goes through the shell's argv parser, not the template renderer, which sidesteps quoting issues.

---

### Capture output and feed it forward

**When to reach for this:** any runbook where step N+1 needs information from step N — the kernel version, the current commit hash, an API response.

**YAML:**

```yaml
name: tag-image
steps:
  - name: Read commit
    type: shell
    shell:
      command: git rev-parse --short HEAD
    capture: commit

  - name: Tag and push
    type: shell
    shell:
      command: 'docker tag app:latest app:{{.commit}} && docker push app:{{.commit}}'
```

**What happens:** step 1 runs `git rev-parse --short HEAD`, captures the trimmed stdout (e.g. `a3f2c91`) into a variable named `commit`. Step 2's command template expands to `docker tag app:latest app:a3f2c91 && docker push app:a3f2c91`. The shell sees one fully-resolved command line.

**Variations:**

- **Capture into a name that overrides an input variable.** Useful for "look up X from Y, then use X going forward."
- **Capture an HTTP response.** Same syntax, except the captured string is the response body, not stdout. See [GET healthcheck](#http-get-healthcheck).

**Gotchas:**

- Captures are `TrimSpace`d. Trailing newlines are removed. If you wanted the trailing newline (rare), capture into a tempfile in the shell step and don't use `capture:`.
- Captures are strings. To pull a field from JSON, pipe through `jq` and capture the result. See [Parse JSON with jq](#parse-json-with-jq).
- A capture into name `X` overwrites whatever input variable `X` was. Pick distinct names if both should remain visible.

---

### Shell step with working directory

**When to reach for this:** running a command in a specific repo or build dir without `cd && command` boilerplate.

**YAML:**

```yaml
- name: Run tests
  type: shell
  shell:
    command: go test ./...
    dir: '{{.repo_root}}'
```

**What happens:** the child process is spawned with cwd set to the expanded `dir:` value. The `command:` runs there. Both `command` and `dir` are template-expanded — you can use captured variables in either.

**Notes:** `dir` is roughly equivalent to prefixing the command with `cd <dir> &&` but cleaner — no quoting headaches if the path contains spaces, and the `cd` failure (e.g. directory doesn't exist) is reported as a clear error.

---

### Parse JSON with jq

**When to reach for this:** you need a single field from a JSON response — an ID, a status, a URL — and want to use it in later steps.

**YAML:**

```yaml
steps:
  - name: Look up release
    type: http
    http:
      method: GET
      url: 'https://api.github.com/repos/{{.owner}}/{{.repo}}/releases/latest'
    capture: release

  - name: Extract tag
    type: shell
    shell:
      command: 'echo {{.release | toJSON}} | jq -r .tag_name'
    capture: tag

  - name: Show
    type: shell
    shell:
      command: 'echo "Latest: {{.tag}}"'
```

Wait — runbook templates don't have a `toJSON` function. The cleaner pattern: pipe via stdin to bypass quoting:

```yaml
- name: Extract tag
  type: shell
  shell:
    command: 'jq -r .tag_name <<< $RUNBOOK_release'
  capture: tag
```

**What happens:** step 1 captures the entire JSON body. Step 2 references it via `$RUNBOOK_release` (the env-export form, no template quoting needed) and pipes it into `jq -r .tag_name`. The result (e.g. `v1.2.3`) is captured into `tag`.

**Variations:**

- **Pull multiple fields:** `jq -r '.id, .name, .url'` and split downstream.
- **Strict mode:** add `set -euo pipefail` at the top if you want the step to fail loudly when jq returns nothing.

**Gotchas:**

- `<<<` is bash-specific (and zsh's, as a herestring). `runbook` invokes via `sh -c`, which on most distros is bash or dash. Dash doesn't support herestrings — fall back to `echo "$RUNBOOK_release" | jq ...` if you're targeting cross-shell environments.
- Don't use `{{.release}}` for a multi-line JSON value inside a quoted string — newlines confuse `sh -c`. The `$RUNBOOK_release` form receives the value as a single env-var string, which the shell handles correctly.

---

### Conditional cleanup step

**When to reach for this:** a step that should only run when a previous step set a particular condition (deploy succeeded, rebuild was needed, etc.).

**YAML:**

```yaml
steps:
  - name: Build artifact
    type: shell
    shell: { command: ./build.sh }
    capture: build_output

  - name: Cleanup tmp
    type: shell
    shell: { command: 'rm -rf /tmp/build-*' }
    condition: '{{if .build_output}}true{{end}}'
    on_error: continue
```

**What happens:** the cleanup step checks if `.build_output` rendered to anything non-empty. If so, the condition template renders to `"true"` and the step runs. If the build step was skipped (e.g., earlier abort), the variable is empty, the condition renders to empty string, and cleanup is skipped.

**Variations:**

- **Run only on a specific environment:** `condition: '{{if eq .env "prod"}}true{{end}}'`.
- **Run only when an explicit flag is set:** add a `--var force_cleanup=true` flag and condition on `'{{if eq .force_cleanup "true"}}true{{end}}'`.

**Notes:** the condition's template must render to the literal string `"true"` (case-sensitive, trimmed). Anything else — `"yes"`, `"True"`, `"1"`, errors, empty — skips the step.

---

## HTTP recipes

### HTTP GET healthcheck

**When to reach for this:** checking that a service is up before (or after) doing something else.

**YAML:**

```yaml
- name: Healthcheck
  type: http
  http:
    method: GET
    url: 'https://{{.host}}/healthz'
  capture: health
  timeout: 10s
```

**What happens:** runbook fires a GET, captures the response body into `health`, and fails the step if the status is 4xx or 5xx. The streamed log starts with `HTTP 200 OK` followed by the body, so you can eyeball success at a glance.

**Variations:**

- **Add headers:** `headers: { Accept: application/json }`.
- **Treat 5xx as transient:** see [Retry on transient HTTP failure](#retry-on-transient-http-failure).
- **Treat 404 as "OK, missing":** `on_error: continue` to record the failure but proceed; subsequent steps can `condition:` on the captured body.

**Gotchas:**

- Status codes 400+ are *errors*, not just states. Capture happens before the error is raised, so `{{.health}}` in later steps is whatever 4xx body the server returned. Be deliberate with `on_error: continue` here.
- No automatic Content-Type. If the server returns JSON, set `Accept: application/json` so any negotiation works as expected.

---

### POST JSON payload

**When to reach for this:** triggering a webhook, kicking off a CI build, posting to an internal API.

**YAML:**

```yaml
- name: Trigger build
  type: http
  http:
    method: POST
    url: 'https://ci.example.com/api/builds'
    headers:
      Content-Type: application/json
      Authorization: 'Bearer {{.api_token}}'
    body: |
      {"branch": "{{.branch}}", "commit": "{{.commit}}", "force": false}
  capture: build_id
  timeout: 30s
```

**What happens:** the `body:` block is a multi-line YAML string with template expansions inline. Headers are template-expanded too — handy for `Authorization: Bearer {{.api_token}}` patterns. The captured body is the API's response — typically `{"id": "..."}`. Pipe through jq if you need just the id (see [Parse JSON with jq](#parse-json-with-jq)).

**Variations:**

- **Form-encoded body:** swap Content-Type for `application/x-www-form-urlencoded` and write `body: 'key1=val1&key2=val2'`.
- **No body:** omit `body:` entirely. POST with no body is valid for some APIs (e.g., "trigger a refresh").

**Notes:** runbook preserves the body as-typed. Don't double-quote JSON — the template engine doesn't strip quotes.

---

### HTTP with bearer token

**When to reach for this:** any authenticated API call where the token comes from 1Password.

**YAML:**

```yaml
variables:
  - name: api_token
    default: 'op://Vault/Service/api-token'
    secret: true

steps:
  - name: List jobs
    type: http
    http:
      method: GET
      url: 'https://api.example.com/jobs'
      headers:
        Authorization: 'Bearer {{.api_token}}'
    capture: jobs
```

**What happens:** at variable resolution time, runbook sees `op://...` as the value of `api_token`, looks up the keychain for `api_token`, falls back to `op read` on first run, caches the result. From step 1 onwards, `{{.api_token}}` is the actual token. The TUI masks it (`secret: true`).

**Notes:** pre-warm with `runbook auth list-jobs` to front-load the Touch ID prompt before the run starts. Not pre-warming is fine for interactive runs but breaks for cron.

---

### Retry on transient HTTP failure

**When to reach for this:** flaky upstream services, post-deploy waits where the server hasn't fully come back yet.

**YAML:**

```yaml
- name: Wait for app
  type: http
  http:
    method: GET
    url: 'https://{{.host}}:8080/healthz'
  on_error: retry
  retries: 10
  timeout: 5s
```

**What happens:** the step runs up to 11 times (1 initial + 10 retries). Each attempt has a 5-second timeout. The first 200 OK ends the loop and proceeds. If all 11 attempts fail, the step is `failed` and `on_error: retry` falls through to abort by default — subsequent steps skip.

**Variations:**

- **Linear backoff:** the executor itself doesn't sleep between retries. Add a separate `sleep` step that's conditional on the previous step's failure (see [Retry with exponential backoff](#retry-with-exponential-backoff)).
- **Don't abort on final failure:** there's no per-step `on_error_after_retries`. The closest equivalent is moving the step's effect into a chain where the *next* step is `on_error: continue` — runbook still fails the *step* but the runbook keeps running.

**Gotchas:**

- A `404` is currently a "failure" because of the 4xx-is-error rule. Retrying a 404 is rarely useful. Pair `retry` with a step you expect to succeed eventually (post-deploy 502 → 200, etc.), not with checks of "does this resource exist."
- Retries don't sleep. A misconfigured URL that returns instantly will burn through `retries+1` attempts in a fraction of a second, then abort.

---

## SSH recipes

### SSH with agent auth

**When to reach for this:** you already have keys loaded in `ssh-agent` (or 1Password's SSH agent) and just want runbook to use them.

**YAML:**

```yaml
- name: Restart nginx
  type: ssh
  ssh:
    host: prod-web-01
    user: deploy
    agent_auth: true
    command: 'sudo systemctl restart nginx'
  timeout: 30s
```

**What happens:** runbook reads `SSH_AUTH_SOCK` from the env, dials the agent, and lets it sign the handshake. If you have a `~/.ssh/config` block for `prod-web-01` with `IdentityAgent`, that overrides `SSH_AUTH_SOCK`.

**Variations:**

- **Multiple hosts:** repeat the step with different hosts. They run sequentially; add `parallel: true` if they're independent.

**Gotchas:**

- Cron-launched runs don't inherit your interactive `SSH_AUTH_SOCK`. Either bake the socket path into the cron environment via `~/.ssh/config`'s `IdentityAgent`, or use a cached key (see [SSH with 1Password-stored key](#ssh-with-1password-stored-key)).

---

### SSH with explicit key file

**When to reach for this:** dedicated deploy key on disk, not in the agent.

**YAML:**

```yaml
- name: Deploy
  type: ssh
  ssh:
    host: prod-db-01
    user: backup
    key_file: ~/.ssh/backup-deploy
    command: '/usr/local/bin/run-backup'
  timeout: 1h
```

**What happens:** runbook reads `~/.ssh/backup-deploy` (with `~` expanded), parses it (OpenSSH or PKCS#8), and uses it as the auth method. Doesn't touch the SSH agent.

**Notes:** if the key is encrypted with a passphrase, runbook will fail to parse it. Use an unencrypted key for unattended runs, or load it into the agent and use `agent_auth: true` instead. For passphrase-protected keys that should remain encrypted on disk, the recommended pattern is to store the unencrypted key in 1Password and use the `op://` form below.

---

### SSH with 1Password-stored key

**When to reach for this:** keep keys in 1Password, never on disk in plaintext, but still want unattended cron-launched runs to work.

**YAML:**

```yaml
- name: Deploy
  type: ssh
  ssh:
    host: prod-web-01
    user: deploy
    key_file: 'op://Vault/Production Deploy Key/private key'
    command: 'sudo systemctl restart app'
```

**What happens:** the SSH executor checks the keychain for the `op://`-keyed cache. If present, decrypts and uses it. If not, **the step fails with "ssh key not cached — run `runbook auth` first"**. This is intentional — Touch ID prompts in cron jobs would just lock the run.

The cache is shared by `op://` reference: multiple steps using the same key get one entry. Cache key is `ssh_key_<op-ref>`.

**Setup:**

```sh
# Pre-cache before scheduling
runbook auth deploy
# Touch ID once, key cached. Future runs (interactive or cron) skip the prompt.
```

**Gotchas:**

- 1Password's "Copy Private Key" produces PKCS#8 format. Runbook handles both that and OpenSSH format, so paste either into the 1Password item.
- If the cached key gets stale (rotated in 1Password but not re-cached), `runbook auth --clear deploy` then `runbook auth deploy` refreshes it.

---

### SSH via ~/.ssh/config alias

**When to reach for this:** you already have a `Host prod-web` entry in `~/.ssh/config` with `Hostname`, `User`, `Port`, and `IdentityFile`. Don't repeat yourself in the runbook.

**YAML:**

```yaml
- name: Deploy
  type: ssh
  ssh:
    host: prod-web                 # ~/.ssh/config alias
    command: 'sudo systemctl restart app'
```

**What happens:** runbook reads `~/.ssh/config`, finds the `Host prod-web` block, and pulls `Hostname`, `User`, `Port`, `IdentityFile`, and `IdentityAgent` from it. Anything explicitly set in the runbook (`user:`, `port:`, etc.) takes precedence over the config-file value.

**Supported `~/.ssh/config` directives:**

| Directive | Used? |
|-----------|-------|
| `Hostname` | ✓ |
| `User` | ✓ |
| `Port` | ✓ |
| `IdentityFile` | ✓ (single, the first one wins for that block) |
| `IdentityAgent` | ✓ |
| `ProxyJump`, `ProxyCommand` | ✗ |
| `ForwardAgent` | ✗ |
| `StrictHostKeyChecking` | ✗ — runbook always uses `~/.ssh/known_hosts` if present, else falls back to `InsecureIgnoreHostKey` |
| `Match` blocks | ✗ — only `Host` blocks are parsed |
| Wildcards in `Host` | ✓ (`*` and `?`) |

**Notes:** the parser is intentionally simple. If your `~/.ssh/config` uses `Include`, `Match`, or `ProxyJump`, those won't be honored — set them explicitly in the runbook step or shell out to `ssh` via a `shell:` step that uses your full-fat OpenSSH client.

---

## Variables and templates

### Prompt for a required variable

**When to reach for this:** an argument that has no sensible default and that you'd rather have the runbook ask for than fail on.

**YAML:**

```yaml
variables:
  - name: version
    required: true
    prompt: 'Version to deploy'
```

**What happens:**

| Invocation | Behavior |
|-----------|----------|
| `runbook run deploy` | Prompts: `? Version to deploy: ` and reads from stdin |
| `runbook run --var version=1.2.3 deploy` | Skips prompt; uses `1.2.3` |
| `RUNBOOK_VAR_VERSION=1.2.3 runbook run deploy` | Skips prompt; uses `1.2.3` |
| `runbook run --yes deploy` | **Still prompts.** `--yes` skips confirmation steps but cannot manufacture variable values. |

**Notes:** prompts happen before the TUI starts, so they always render correctly regardless of `--no-tui`. In CI / cron, always supply variables via `--var` or env — runbook will hang waiting for stdin otherwise.

---

### Env override for CI

**When to reach for this:** a single runbook used both interactively and in CI, where CI sets a different value via env.

**YAML:**

```yaml
variables:
  - name: deploy_target
    default: staging
```

**What happens:**

```sh
runbook run deploy                              # → staging (default)
RUNBOOK_VAR_DEPLOY_TARGET=prod runbook run deploy  # → prod (env)
runbook run --var deploy_target=prod deploy     # → prod (CLI; same effect)
```

The env name is uppercased and prefixed: `deploy_target` → `RUNBOOK_VAR_DEPLOY_TARGET`. Hyphens are not allowed in env names — use underscores in your variable names if you plan to override via env (`deploy-target` is invalid).

**Notes:** CLI overrides beat env overrides. CI typically uses env (cleaner in pipeline definitions); humans typically use `--var`.

---

### 1Password secret as a variable

**When to reach for this:** any value that's a real secret and should never live in plaintext YAML.

**YAML:**

```yaml
variables:
  - name: db_password
    default: 'op://Engineering/PostgreSQL prod/password'
    secret: true
```

**What happens:** at variable-resolution time, runbook resolves the `op://` reference through the keychain (if cached) or the 1Password CLI (if not), and stores the result in `vars["db_password"]`. The `secret: true` flag tells the TUI to mask the value in the variable summary panel — useful when sharing a screen demo.

**Pre-warm:**

```sh
runbook auth migrate-database     # caches all op:// vars and SSH keys
```

**Notes:** the `secret: true` flag is purely cosmetic (TUI masking). It does *not* affect resolution behavior — `op://` references are always cached in the keychain regardless. Setting `secret: true` on a non-`op://` variable masks its value in the TUI without resolving anything.

---

### Optional variable with fallback

**When to reach for this:** a variable that's normally unset but, when present, changes behavior.

**YAML:**

```yaml
variables:
  - name: log_level

steps:
  - name: Run app
    type: shell
    shell:
      command: '{{if .log_level}}LOG={{.log_level}} {{end}}./run-app'
```

**What happens:** if `log_level` isn't set (no default, no env, no CLI), the template renders to `./run-app`. If set to e.g. `debug`, renders to `LOG=debug ./run-app`.

**Variations:**

- **Default with `or`:** `command: 'LOG={{or .log_level "info"}} ./run-app'`. Cleaner when you always want *some* value.
- **Inline conditional:** `command: '{{if eq .env "prod"}}--strict {{end}}./run'`.

**Notes:** templates use `Option("missingkey=zero")` — referencing a variable that's never been set is the empty string, not an error. That keeps templates safe for optional variables but also silently swallows typos. `runbook validate` plus `--dry-run` are the early-warning system.

---

## Flow control

### Confirm before destructive step

**When to reach for this:** a deploy, a database migration, anything that's hard to undo.

**YAML:**

```yaml
steps:
  - name: Health check
    type: http
    http: { method: GET, url: 'https://{{.host}}/healthz' }

  - name: Confirm deploy
    confirm: 'Deploy {{.version}} to {{.host}}?'

  - name: Stop service
    type: ssh
    ssh:
      host: '{{.host}}'
      command: 'sudo systemctl stop app'

  - name: Start service
    type: ssh
    ssh:
      host: '{{.host}}'
      command: 'sudo systemctl start app'
```

**What happens:** the runbook pauses at the confirm step, prompts `? Deploy 1.2.3 to web-01? [y/N]:`. On `y`, proceeds. On `n` (or any non-yes input), the confirm step is marked skipped and execution continues to the next step. **Important:** the confirm step's "skip" does NOT cancel the runbook — for that, abort the run via `Ctrl-C`.

To make a "no" actually halt the runbook, attach the confirm to the destructive step itself:

```yaml
- name: Stop service
  type: ssh
  ssh: { host: '{{.host}}', command: 'sudo systemctl stop app' }
  confirm: 'Really stop app on {{.host}}?'
```

If you decline, the SSH step is skipped — and the start step is also skipped (because it depends on the previous step having stopped the service, which it didn't). Effectively you've cancelled the destructive part and everything downstream of it... unless a later step has `condition:` logic that bypasses the dependence. Sequential YAML order is your friend; parallel groups complicate this.

**Variations:**

- **`--yes` for unattended:** auto-accepts every prompt. Always pair with cron / scripted runs.
- **Confirm on a regex or timestamp condition:** combine `confirm:` with `condition:` to gate the prompt itself behind a check.

**Notes:** the prompt happens via the configured observer — the TUI shows it as a modal, the CLI mode prints to stderr and reads from stdin. In `--no-tui` mode, declining requires typing `n` or any non-`y/yes` answer.

---

### Parallel health probes

**When to reach for this:** check N services concurrently and aggregate the results, instead of waiting on each one sequentially.

**YAML:**

```yaml
name: multi-service-healthcheck
variables:
  - name: base_url
    default: 'http://localhost'
steps:
  - name: Check API
    type: http
    http: { method: GET, url: '{{.base_url}}:8080/healthz' }
    parallel: true
    capture: api_health
    on_error: continue

  - name: Check Web
    type: http
    http: { method: GET, url: '{{.base_url}}:3000/health' }
    parallel: true
    capture: web_health
    on_error: continue

  - name: Check Worker
    type: http
    http: { method: GET, url: '{{.base_url}}:9090/health' }
    parallel: true
    capture: worker_health
    on_error: continue

  - name: Report
    type: shell
    shell:
      command: 'echo "API={{.api_health}} Web={{.web_health}} Worker={{.worker_health}}"'
```

**What happens:** the three checks fire concurrently. The aggregate step waits for all three to finish, then prints whatever responses each one captured. Because each probe has `on_error: continue`, a single failure doesn't abort the runbook — the report step always runs and shows the actual state.

**Variations:**

- **Fail-loud aggregation:** drop `on_error: continue` from each probe. Then any 4xx/5xx fails the runbook, but the parallel group still runs to completion (so all three responses are captured before the runbook aborts).
- **More than three:** up to N consecutive `parallel: true` steps form one group. There's no concurrency cap — they all fire together.

**Gotchas:**

- Captures from a parallel group land in the variable map in arbitrary order. Don't have one parallel step's command reference another parallel step's capture — that's a race. Sequential is the right shape there.
- Long timeouts in a parallel group apply per-step. Three steps with `timeout: 30s` in one group can take up to 30s total (whichever is slowest), not 90s.

---

### Retry with exponential backoff

**When to reach for this:** a step that's expected to flap during a transient window (post-deploy DB warmup, S3 eventual consistency, slow service starts).

**YAML:**

```yaml
steps:
  - name: Probe service (try 1)
    type: http
    http: { method: GET, url: 'https://{{.host}}/health' }
    on_error: continue
    capture: probe_1

  - name: Sleep 2s
    type: shell
    shell: { command: 'sleep 2' }
    condition: '{{if not .probe_1}}true{{end}}'

  - name: Probe service (try 2)
    type: http
    http: { method: GET, url: 'https://{{.host}}/health' }
    on_error: continue
    condition: '{{if not .probe_1}}true{{end}}'
    capture: probe_2

  - name: Sleep 5s
    type: shell
    shell: { command: 'sleep 5' }
    condition: '{{if not .probe_2}}true{{end}}'

  - name: Probe service (try 3)
    type: http
    http: { method: GET, url: 'https://{{.host}}/health' }
    condition: '{{if not .probe_2}}true{{end}}'
```

**What happens:** the first probe fires. If it captures a body (success), subsequent steps' conditions are false and they skip. If it fails, the sleep runs and the next probe fires. Same pattern for a 5-second wait. The final probe doesn't have `on_error: continue` — failure there aborts the runbook.

**Why not just `on_error: retry` with `retries: 3`?** Built-in retry has no inter-attempt delay. For services that need 5+ seconds to come back, back-to-back retries burn through the attempt budget instantly. Manual backoff via separate sleep steps gives you control.

**Variations:**

- **Cleaner with a wrapping shell script:** if you have one that handles backoff (`while !curl ...; do sleep 5; done`), call it from a `shell:` step instead.
- **Inline `sh -c` with a loop:** `command: 'for i in 1 2 4 8; do curl -fsS https://... && exit; sleep $i; done; exit 1'`. Compact, single step, no captures.

**Notes:** with this pattern, the *runbook* is what's "retrying" via condition logic, not the step's `on_error: retry`. That makes the retry visible in history (multiple step records, each clearly labeled) at the cost of more YAML.

---

### Conditional step

**When to reach for this:** a step that should only run sometimes — based on an environment variable, a previous capture, a flag.

**YAML:**

```yaml
- name: Send slack notification (prod only)
  type: shell
  shell:
    command: 'curl -X POST {{.webhook}} -d ''{"text":"Deploy complete"}'''
  condition: '{{if eq .env "prod"}}true{{end}}'
```

**What happens:** the template renders to `"true"` only when `env == "prod"`. Otherwise it renders to the empty string and the step is skipped (status `skipped` in history).

**Variations:**

- **Skip unless flag:** `condition: '{{if .force}}true{{end}}'` — only runs when `--var force=anything` is set.
- **Capture-driven:** `condition: '{{if eq .build_status "success"}}true{{end}}'` — runs only if a previous step's capture matches.
- **Negation:** `condition: '{{if not .build_status}}true{{end}}'` — runs only if the previous capture is empty.

**Gotchas:**

- The match is against the literal string `"true"` (case-sensitive, post-trim). `"True"` doesn't work. `"yes"` doesn't work. `"1"` doesn't work. Always render to exactly `"true"` for the affirmative case.
- Empty templates render to empty (falsy). A typo in the condition (e.g. `{{if eq .ev "prod"}}true{{end}}` — wrong field name) silently skips the step every time.

---

## Logging and notifications

### Append-mode log for scheduled runs

**When to reach for this:** any cron-scheduled runbook where you want a single growing log file you can `tail -f` and rotate externally.

**YAML:**

```yaml
log:
  enabled: true
  mode: append
  dir: ~/.runbook/logs/
```

**What happens:** every run appends to `~/.runbook/logs/<name>.log`, prefixed with a separator like:

```
--- run: 2026-04-28T03:00:00 ---
▸ Step 1: Pull updates
  │ ...
```

The separator includes a parseable ISO timestamp, so log viewers and the Mac app's History view can locate the bytes for a specific run.

**Variations:**

- **`mode: new`** (default): one file per run, named `<name>-<timestamp>.log`. Easier to grep individually but produces hundreds of files for frequently-scheduled runbooks.
- **Custom dir:** `dir: /var/log/runbook/` if you want logs centralized elsewhere. Make sure the user running the cron job can write there.

**Gotchas:**

- Append-mode files grow without bound. Pair with external rotation (logrotate on Linux, newsyslog on macOS) or with a sortie rule that compresses-and-archives weekly. After rotation, run `runbook log reindex` to update the index file the Mac app uses.

**Notes:** cron-launched runs already write a separate logfile (via crontab's `>> ~/.runbook/history/<name>.log` redirect). With both `log: enabled` and cron, you get two files with the same content. Harmless duplication.

---

### Slack on failure only

**When to reach for this:** quiet runbook that you want to ignore unless something breaks.

**YAML:**

```yaml
notify:
  on: failure
  slack:
    webhook: 'op://Vault/Slack/runbook-alerts-webhook'
    channel: '#alerts'
```

**What happens:** after a successful run, the notify block is skipped. After a failure, runbook posts a message to the configured webhook with the runbook name, status, duration, and a step-by-step status list (with errors).

**Variations:**

- **`on: always`** (default): notify every run regardless of outcome. Useful for high-stakes runbooks where "it didn't fire" is also a problem.
- **`on: success`**: e.g., a backup that should chirp into a `#backups` channel only on a clean completion.

**Notes:** the Slack message uses the webhook's default channel unless `channel:` is set. Some webhooks ignore `channel:` overrides; check your webhook config if the messages aren't landing where you expect.

---

### Desktop notification on success

**When to reach for this:** a long-running interactive runbook (rebuild, sync, batch process) where you want a banner when it's done.

**YAML:**

```yaml
notify:
  on: success
  desktop: true
```

**What happens:** at the end of a successful run, runbook fires a native OS notification:

| Platform | Backend |
|----------|---------|
| macOS | `osascript` — banner with title "runbook" and the runbook name as subtitle |
| Linux | `notify-send` (requires `libnotify-bin`) |
| Windows | PowerShell BurntToast if installed; falls back to console message |

**Notes:** desktop notifications from cron-launched runs don't work reliably — there's no graphical session. Use Slack or email for unattended runs and reserve desktop for interactive ones.

---

### Email digest with per-step status

**When to reach for this:** stakeholder summaries — weekly health checks where someone wants to see the full per-step status in their inbox.

**YAML:**

```yaml
notify:
  on: always
  email:
    to: oncall@example.com
    from: runbook@example.com
    host: smtp.example.com:587
    username: runbook@example.com
    password: 'op://Vault/SMTP runbook/password'
```

**What happens:** the email body is auto-generated:

```
Runbook: weekly-health-check
Status: succeeded
Duration: 1m32.5s
Steps: 5

  ✓ Probe API (200ms)
  ✓ Probe Web (180ms)
  ✓ Probe Worker (210ms)
  ✓ DB connection (450ms)
  ✓ Report (90ms)
```

The subject is `✓ Runbook "weekly-health-check" succeeded` (or `✗ ...failed` on failure).

**Notes:** the SMTP path uses Go's `net/smtp.SendMail` with PLAIN auth. TLS happens via the port (587 with STARTTLS upgrade, 465 explicit TLS). If your provider needs OAuth or app-specific passwords, generate one in the provider's settings and store it in 1Password.

---

## Scheduled runs

### Hourly health check via cron

**When to reach for this:** a runbook you want fired every hour, regardless of whether you're at the keyboard.

**Setup:**

```sh
runbook cron add hourly-health "0 * * * *"
```

That installs a crontab line:

```
0 * * * * /Users/you/.local/bin/runbook run --no-tui --yes hourly-health >> /Users/you/.runbook/history/hourly-health.log 2>&1 # runbook: hourly-health
```

The trailing `# runbook: <name>` marker is how `runbook cron list` and `runbook cron remove` find runbook-managed entries — they don't touch any other crontab entries.

**Verify:**

```sh
runbook cron list
crontab -l | grep runbook   # raw view
```

**Watch the log:**

```sh
tail -f ~/.runbook/history/hourly-health.log
```

**Remove when done:**

```sh
runbook cron remove hourly-health
```

**Variations:**

- **Multiple schedules per runbook:** call `runbook cron add` more than once with different schedule strings. They all coexist; remove one specifically with `runbook cron remove <name> "<schedule>"`.
- **Different schedule:** see [Reference › Cron schedule syntax](04-reference.md#cron-schedule-syntax) for the field semantics and common patterns.

**Notes:** the runbook must already be discoverable (`runbook list` shows it) before `cron add` will take it. The cron line runs the binary at the path it had when you ran `cron add` — if you move the binary later, the cron entry stops working. The Makefile's `make deploy` puts it at `~/.local/bin/runbook`, which is stable.

---

### Nightly backup

**When to reach for this:** unattended backup with notifications and a permanent log.

**YAML (`~/.runbook/books/nightly-backup.yaml`):**

```yaml
name: nightly-backup
description: Snapshot the database to S3

variables:
  - name: db_url
    default: 'op://Engineering/PostgreSQL/url'
    secret: true
  - name: s3_bucket
    default: 'company-backups'

steps:
  - name: Dump database
    type: shell
    shell:
      command: 'pg_dump "$RUNBOOK_db_url" | gzip > /tmp/backup.sql.gz'
    timeout: 1h
    on_error: abort

  - name: Upload to S3
    type: shell
    shell:
      command: 'aws s3 cp /tmp/backup.sql.gz s3://{{.s3_bucket}}/$(date +%Y-%m-%d).sql.gz'
    timeout: 1h

  - name: Cleanup
    type: shell
    shell:
      command: 'rm -f /tmp/backup.sql.gz'
    on_error: continue

notify:
  on: failure
  slack:
    webhook: 'op://Vault/Slack/backup-alerts-webhook'

log:
  enabled: true
  mode: append
```

**Setup:**

```sh
runbook auth nightly-backup            # cache the op:// secrets
runbook cron add nightly-backup "0 3 * * *"   # 3 AM daily
```

**What happens at 3 AM:** cron fires `runbook run --no-tui --yes nightly-backup`. The runbook pulls the cached database URL from the keychain (no Touch ID needed). It dumps, uploads, and cleans up. On failure, Slack gets a message; either way, the run appends to `~/.runbook/logs/nightly-backup.log`. The history record lands in `~/.runbook/history/`.

**Notes:** the dump command uses `$RUNBOOK_db_url` (env-export form) instead of `{{.db_url}}` to avoid quoting the URL inside an interpolated shell command. The env form passes the value as a single argv entry, which `pg_dump` handles cleanly even if the URL contains special characters.

---

## Sharing and templates

Runbook treats your books directory as the unit of distribution. A git repo full of YAML runbooks (plus optional `templates/` subdirectories) can be pulled by anyone with `runbook pull` and used immediately by name. This is how a team standardizes on a shared set of operational runbooks without copying YAML around.

### Pull a shared runbook collection

**When to reach for this:** your team (or a public source) maintains a git repo of runbooks and you want them all locally, named and runnable.

**Setup:**

```sh
runbook pull github.com/yourteam/runbooks
```

**What happens:** runbook does a shallow `git clone --depth 1` of the repo into `~/.runbook/books/runbooks/` (the directory name is the repo's last path component, with `.git` stripped). Discovery walks one level into the books dir, so every YAML inside the cloned repo is now reachable by `name:` from `runbook run`. Templates inside the repo's `templates/` subdirectories surface via `runbook list --templates`.

```sh
runbook list                    # see everything (including pulled)
runbook pull list               # see which collections are pulled
runbook run deploy-app          # run a runbook from the pulled collection
runbook list --templates        # see scaffolds the collection ships
```

**Variations:**

- **Single-file pull:** `runbook pull https://example.com/deploy.yaml` downloads one YAML directly into `~/.runbook/books/`. Useful when a colleague shares a one-off file or you want to grab a published recipe without subscribing to the whole collection.
- **Authenticated repos:** if the repo requires SSH or token auth, the underlying `git clone` uses your normal git credentials (`~/.ssh/...`, git credential helpers, etc.). runbook doesn't intercept auth.

**Gotchas:**

- **Re-pulling fast-forwards.** Calling `runbook pull` on an already-pulled URL runs `git pull --ff-only` in the existing checkout. If the remote has been force-pushed or your local has uncommitted changes, the pull fails — fix it manually with `cd ~/.runbook/books/<name> && git status`.
- **Name collisions** between your top-level books and a pulled collection are decided by discovery order: top-level wins. If you pull a collection that ships a runbook called `deploy` and you already have one, your local one is what `runbook run deploy` runs. Rename one of them.

**Notes:** `runbook pull remove <name>` removes the cloned directory entirely — irreversible from runbook's side, but a fresh `runbook pull <url>` re-clones from the remote.

---

### Publish your own runbook collection

**When to reach for this:** you've built a useful set of runbooks for yourself or your team and want them version-controlled and shareable across machines.

**Setup:**

```sh
cd ~/.runbook/books
git init
echo 'history/' > .gitignore       # ignore any history that wandered in
git add *.yaml templates/
git commit -m "Initial runbook collection"
git remote add origin git@github.com:you/runbooks.git
git push -u origin main
```

**What happens:** your books directory is now a regular git repo with a remote. From any other machine, `runbook pull github.com/you/runbooks` clones it into that machine's `~/.runbook/books/runbooks/` and the runbooks are immediately usable. Updates flow via `git push` from your authoring machine and `runbook pull` (re-pull) on the consuming machine.

**Variations:**

- **Don't put the whole `~/.runbook/books/` under git** if you also have local-only runbooks you don't want shared. Instead, make a *subdirectory* the repo: `~/.runbook/books/myteam-runbooks/` becomes the cloned repo for everyone (including you). Discovery walks one level into the books dir, so YAML in the subdirectory is found regardless of whether you cloned it or authored it in place.
- **Include a `templates/` directory.** Put scaffolding runbooks there. Anyone who pulls the collection gets `runbook create --from <name>` access to those templates without polluting their normal runbook list.
- **README in the collection.** Even though runbook doesn't render it, a `README.md` in the repo root describes what each runbook does for humans browsing on GitHub.

**Gotchas:**

- **Don't commit secrets.** Use `op://` references for any sensitive value — keys, webhooks, tokens. The YAML stays safe in plaintext because the secret resolves at run-time from each user's own 1Password vault.
- **Filename vs `name:` field.** `runbook` matches by `name:`, not filename. If you rename a YAML file, callers who use `runbook run <name>` aren't affected — but anyone who scripted `runbook run path/to/file.yaml` is. Pick a convention and stick to it.
- **Cron entries reference absolute paths to the binary.** Sharing the YAML doesn't share scheduling — each consumer who wants a scheduled run must call `runbook cron add` on their own machine.

**Notes:** pulled repos are full git checkouts. Consumers can `cd ~/.runbook/books/<name> && git log` to see history, `git diff HEAD~1` to see what changed in the last update, etc.

---

### Scaffold a new runbook from a template

**When to reach for this:** you're starting a new runbook of a familiar shape (SSH deploy, HTTP healthcheck, scheduled backup) and don't want to write the boilerplate.

**Setup:**

```sh
runbook list --templates              # see what's available
runbook create my-deploy --from ssh-remote
```

**What happens:** runbook finds the template named `ssh-remote` (anywhere under `~/.runbook/books/**/templates/`), copies its YAML, replaces the `name:` field with `my-deploy`, writes the result to `~/.runbook/books/my-deploy.yaml`, and opens it in `$EDITOR` (if set). You fill in the variables, tweak the steps, save, and the new runbook is immediately runnable.

**Variations:**

- **Without `--from`:** `runbook create my-task` writes a minimal one-step shell scaffold. Use this when no template fits and you want a known-clean starting point rather than a blank file.
- **From a specific collection's template:** templates are matched by their `name:` field, not their location. If two collections both ship a template called `ssh-remote`, the first one discovery encounters wins — rename one if it matters.

**Gotchas:**

- **`runbook create` fails if `<name>.yaml` already exists.** That's a safety check — it won't overwrite. Rename or delete the old file first.
- **`$EDITOR` opens the new file but the runbook is already on disk.** If you save no changes (or your editor crashes), the file is still there with the template's defaults. Edit it later with `$EDITOR ~/.runbook/books/my-deploy.yaml`.

**Notes:** the `name:` field swap is the only transformation runbook makes. Everything else — steps, variables, notify, log — is copied verbatim from the template.

---

### Author a template for a shared collection

**When to reach for this:** you maintain a runbook collection and you want consumers to be able to scaffold new runbooks of common shapes without writing them from scratch.

**Layout:**

```
~/.runbook/books/myteam-runbooks/
├── deploy-app.yaml          # a real runbook, runs by name
├── nightly-backup.yaml      # a real runbook, runs by name
└── templates/
    ├── ssh-remote.yaml      # template, ONLY surfaces via list --templates
    └── http-healthcheck.yaml
```

**Template content (`templates/ssh-remote.yaml`):**

```yaml
name: ssh-remote-template     # placeholder; runbook create overwrites this
description: SSH to a remote host and run a command

variables:
  - name: host
    required: true
    prompt: "Target host"
  - name: command
    required: true
    prompt: "Command to run"

steps:
  - name: Run remote command
    type: ssh
    ssh:
      host: '{{.host}}'
      user: deploy
      agent_auth: true
      command: '{{.command}}'
    timeout: 5m

log:
  enabled: true
  mode: append
```

**What happens:** because the file lives under `templates/`, discovery skips it during normal listing — `runbook list` doesn't show it, `runbook run ssh-remote` won't find it. But `runbook list --templates` does show it, and `runbook create my-new-thing --from ssh-remote-template` scaffolds a real runbook from it.

**Variations:**

- **Use `prompt:` and `required:` liberally** in templates — they're the user-friendly way to ask for the values that need to be filled in. Consumers running `runbook run` on the new runbook will be prompted for any required field they don't pass via `--var`.
- **Include `log:` and `notify:` blocks** in templates if your team's convention is to log every run. Consumers can delete those blocks if they don't want them.
- **Document the template's intent** in the `description:` field. It surfaces in `runbook list --templates` output.

**Gotchas:**

- **The `name:` field gets replaced** by `runbook create`, so don't bake meaning into it. The template's name only matters for `--from` lookup; pick something descriptive (`ssh-remote-template`, not `template-1`).
- **Templates are not validated as runbooks** during `runbook validate` runs — they're skipped. `runbook validate` operates on user-runnable files only. Test your templates by scaffolding from them and validating the result.

**Notes:** a `templates/` subdirectory can live anywhere under `~/.runbook/books/` — at the top level (your own personal templates) or inside a pulled collection (shared with the team). Template discovery walks the whole tree, so all sources combine into one catalog visible via `runbook list --templates`.

---

### Keep pulled collections fresh

**When to reach for this:** you're using a shared collection that gets updates and you want to stay current without thinking about it.

**Manual approach:**

```sh
runbook pull github.com/yourteam/runbooks   # re-pull at will
```

**Automated approach:** schedule a runbook that re-pulls every collection you care about.

**YAML (`~/.runbook/books/sync-collections.yaml`):**

```yaml
name: sync-collections
description: Re-pull all my shared runbook collections

steps:
  - name: Sync team collection
    type: shell
    shell:
      command: 'runbook pull github.com/yourteam/runbooks'
    on_error: continue

  - name: Sync personal collection
    type: shell
    shell:
      command: 'runbook pull github.com/you/personal-runbooks'
    on_error: continue
```

**Schedule:**

```sh
runbook cron add sync-collections "0 * * * *"     # hourly
```

**What happens:** every hour, cron fires the sync runbook, which re-pulls each collection. `on_error: continue` means a temporary git failure on one collection doesn't stop the others. The history record shows which steps succeeded.

**Variations:**

- **More aggressive:** `*/15 * * * *` for collections that change frequently.
- **With notification on failure:** add a `notify:` block with `on: failure` to ping Slack only when a sync breaks (network outage, repo moved, etc.).

**Gotchas:**

- **Local edits in pulled repos block the fast-forward.** If you've edited a file inside `~/.runbook/books/yourteam-runbooks/` and then sync runs, `git pull --ff-only` fails. Either commit/stash your changes upstream, or move your local-only runbooks out of the pulled directory.
- **No SSH agent in cron.** If your collection's git remote uses SSH and requires an unlocked key, the scheduled sync will fail. Use HTTPS remotes for collections you sync via cron, or make sure the relevant SSH key is cached (e.g., via `runbook auth` if it's an `op://` key).

**Notes:** `runbook pull` is idempotent — re-pulling an unchanged repo is a no-op (`Already up to date`). The history records will look identical; the cost is one network round-trip per collection per tick.

---

## Backups

Two kinds of backup files coexist in `~/.runbook/backups/`:

- **Per-YAML backups** — `<name>-<ISO timestamp>.yaml`. Written automatically by the Mac app before every save and delete. Fine-grained edit history.
- **Full-state snapshots** — `runbook-<ISO timestamp>.tar.gz`. Created by `runbook backup snapshot`. Contains `books/`, `history/`, `pinned.json`, `highlights.yaml`. Designed for migration to a new machine and as a goback target.

The `runbook backup` subcommand tree manages both kinds. Restore is intentionally limited to per-YAML backups (snapshot tarballs need `tar -xzf` because they overwrite a directory tree, which is too destructive for a one-liner).

### Browse and restore backups

**When to reach for this:** you saved a YAML, regret one of the changes, want to see what was there before.

**How:**

```sh
# List everything, newest first
runbook backup list

# Filter to one runbook
runbook backup list deploy

# See the contents of the most recent backup for `deploy`
runbook backup show deploy

# See the diff between the most recent backup and the current file
runbook backup diff deploy

# Restore the most recent backup (auto-saves the current state first,
# so this restore is itself reversible via another restore call)
runbook backup restore deploy
```

**Variations:**

- **`--at <prefix>`:** match a specific timestamp. Prefix-matched, newest hit wins. Examples: `--at 2026-04-29` (any time on that day), `--at 2026-04-29T08` (any time in the 8 AM hour), `--at 2026-04-29T083014` (exact timestamp).
- **Pipe `show` to a pager:** `runbook backup show deploy | less` — useful for long files.
- **Targeted diff:** `runbook backup diff deploy --at 2026-04-28` — compare current state to a specific older backup.

**Gotchas:**

- **The runbook must still exist on disk** for restore and diff to work — the CLI looks up the current file via the same name resolution as `runbook run`. If you've deleted the file entirely, copy the backup manually: `cp ~/.runbook/backups/deploy-<ts>.yaml ~/.runbook/books/deploy.yaml`.
- **Snapshots can't be restored automatically.** `runbook backup restore` rejects them and points at `tar -xzf <file> -C ~/.runbook` so the user can do it deliberately.

**Notes:** the per-YAML backups are written by the Mac app, so there's nothing the CLI does to *create* them. This subcommand tree is purely for managing what's on disk.

---

### Scheduled snapshot via goback

**When to reach for this:** you want a recurring full-state backup of your runbook home — books, history, pinned, highlights — sent to your normal backup pipeline alongside other apps' state.

**How:**

```sh
# Manually, to test
runbook backup snapshot
# → Creates ~/.runbook/backups/runbook-<ISO timestamp>.tar.gz
```

The output path is exactly what goback's `local` job format expects. Add this entry to `~/.config/goback/config.yaml`:

```yaml
- name: runbook
  type: local
  schedule: "0 8 * * 0"    # Sunday 8:00 AM
  folder: runbook
  filename: "runbook_{2006-01-02}.tar.gz"
  pre_command: "~/.local/bin/runbook backup snapshot"
  local_pattern: "~/.runbook/backups/runbook-*.tar.gz"
  post_command: "rm -f ~/.runbook/backups/runbook-*.tar.gz"
  retention: 4
```

**What goback does each Sunday:**

1. Runs `runbook backup snapshot` — writes the tarball to the staging path.
2. Picks up files matching `local_pattern` and copies them into `~/backups/runbook/runbook_<date>.tar.gz`.
3. Runs `post_command` to clean up the staging file (so the dir doesn't grow without bound).
4. Enforces `retention: 4` on the destination — the four most recent archives are kept; older ones are pruned.

**Variations:**

- **More frequent:** `schedule: "0 8 * * *"` for daily.
- **Different retention:** any integer; goback applies `retention` to the destination folder.

**Gotchas:**

- **The staging file must be cleaned up** by `post_command`, otherwise `~/.runbook/backups/` accumulates tarballs you've already archived elsewhere. The provided `rm -f` glob is matched specifically against the snapshot prefix so it never touches the per-YAML backups.
- **Cron's PATH doesn't include `~/.local/bin/` by default** — but `runbook` itself augments PATH with `~/.local/bin/`, `/opt/homebrew/bin/`, `/usr/local/bin/`, and `~/go/bin/` for the steps it spawns. Goback's `pre_command` runs through goback's invocation context, so use the absolute path `~/.local/bin/runbook` (as in the config above) to avoid relying on cron's PATH.

**Notes:** the snapshot deliberately excludes `~/.runbook/logs/` (ephemeral) and `~/.runbook/backups/` (recursive — would back up the backups). This keeps the tarball small and round-trippable.

---

### Prune old backups

**When to reach for this:** the backups directory has grown over time and you want to thin it.

**How:**

```sh
# Default: keep the newest 10 per (kind, name) group
runbook backup prune

# Show what would happen without deleting
runbook backup prune --dry-run

# Keep only 5
runbook backup prune --keep 5

# Delete anything older than 30 days, regardless of count
runbook backup prune --keep 0 --older-than 30d

# Both rules at once
runbook backup prune --keep 10 --older-than 90d
```

**What happens:** the entries are grouped by `(kind, name)` — so `deploy` YAMLs are pruned independently of `nightly-backup` YAMLs, which are pruned independently of snapshot tarballs. Within each group, the newest `--keep N` entries are kept, and anything older than `--older-than DURATION` is removed in addition. `--dry-run` prints the targeted paths without actually deleting them.

**Variations:**

- **Disable the count rule:** `--keep 0` keeps all, ignoring count (useful when paired with `--older-than`).
- **Days vs. hours:** `--older-than 7d` is shorthand for `168h`. `30d`, `2h30m`, `1h` all parse.

**Notes:** snapshots are pruned by the same rules but in a separate group from per-YAML backups. If you have 20 snapshots and 100 per-YAML backups, `--keep 10` keeps 10 of each, not 10 total.

---

## End-to-end runbooks

### Deploy with rollback gate

**When to reach for this:** a real production deploy where you want a pre-deploy probe, a confirmation, the deploy itself, and a post-deploy verification with a manual rollback gate.

**YAML:**

```yaml
name: deploy-app
description: Deploy app version to host with rollback gate

variables:
  - name: version
    required: true
    prompt: 'Version to deploy'
  - name: host
    default: prod-web-01
  - name: api_token
    default: 'op://Vault/Production/api-token'
    secret: true

steps:
  - name: Pre-deploy health
    type: http
    http:
      method: GET
      url: 'https://{{.host}}:8080/healthz'
      headers:
        Authorization: 'Bearer {{.api_token}}'
    capture: pre_health
    on_error: abort

  - name: Confirm deploy
    confirm: 'Deploy {{.version}} to {{.host}}? (pre-deploy was healthy)'

  - name: Deploy
    type: ssh
    ssh:
      host: '{{.host}}'
      user: deploy
      agent_auth: true
      command: 'sudo /usr/local/bin/deploy.sh {{.version}}'
    timeout: 5m
    on_error: abort

  - name: Wait for service
    type: http
    http:
      method: GET
      url: 'https://{{.host}}:8080/healthz'
      headers:
        Authorization: 'Bearer {{.api_token}}'
    on_error: retry
    retries: 12
    timeout: 5s
    capture: post_health

  - name: Confirm rollback
    confirm: 'Post-deploy looks bad. Rollback to previous version?'
    condition: '{{if not .post_health}}true{{end}}'

  - name: Rollback
    type: ssh
    ssh:
      host: '{{.host}}'
      user: deploy
      agent_auth: true
      command: 'sudo /usr/local/bin/rollback.sh'
    condition: '{{if not .post_health}}true{{end}}'

notify:
  on: always
  slack:
    webhook: 'op://Vault/Slack/deploys-webhook'
  desktop: true

log:
  enabled: true
  mode: new
```

**Walk-through:**

1. Pre-deploy probe captures the current health. Fails the runbook if the server isn't reachable.
2. Confirm prompt; user accepts.
3. SSH-deploy fires. 5-minute timeout. Abort on failure.
4. Wait-for-service polls the health endpoint up to 13 times (1 + 12 retries) with a 5s timeout each. First 200 OK ends the loop.
5. If the wait succeeded (`post_health` captured), the rollback steps' conditions are false — they skip, and the run completes as success.
6. If the wait failed (`post_health` empty), the rollback confirm fires. User accepts → rollback SSH executes.
7. Either way, Slack and desktop notify. Per-run log file is written to `~/.runbook/logs/deploy-app-<timestamp>.log`.

**Notes:** the runbook never auto-rolls-back without explicit human confirmation. That's a deliberate safety choice — you might prefer to leave the broken version up while you investigate, rather than thrash. To make rollback automatic, drop the `Confirm rollback` step.

---

### Multi-region health probe

**When to reach for this:** weekly check that each region of a service is up and responding correctly.

**YAML:**

```yaml
name: multi-region-health
description: Probe app in all regions and email a report

variables:
  - name: api_token
    default: 'op://Vault/Health-checks/token'
    secret: true

steps:
  - name: Probe us-east
    type: http
    http:
      method: GET
      url: 'https://us-east.example.com/healthz'
      headers: { Authorization: 'Bearer {{.api_token}}' }
    parallel: true
    capture: us_east
    on_error: continue

  - name: Probe us-west
    type: http
    http:
      method: GET
      url: 'https://us-west.example.com/healthz'
      headers: { Authorization: 'Bearer {{.api_token}}' }
    parallel: true
    capture: us_west
    on_error: continue

  - name: Probe eu
    type: http
    http:
      method: GET
      url: 'https://eu.example.com/healthz'
      headers: { Authorization: 'Bearer {{.api_token}}' }
    parallel: true
    capture: eu
    on_error: continue

  - name: Probe ap
    type: http
    http:
      method: GET
      url: 'https://ap.example.com/healthz'
      headers: { Authorization: 'Bearer {{.api_token}}' }
    parallel: true
    capture: ap
    on_error: continue

  - name: Summary
    type: shell
    shell:
      command: |
        echo "Regional health snapshot:"
        echo "  us-east: ${RUNBOOK_us_east:-FAILED}"
        echo "  us-west: ${RUNBOOK_us_west:-FAILED}"
        echo "  eu:      ${RUNBOOK_eu:-FAILED}"
        echo "  ap:      ${RUNBOOK_ap:-FAILED}"

notify:
  on: always
  email:
    to: ops@example.com
    from: runbook@example.com
    host: smtp.example.com:587
    username: runbook@example.com
    password: 'op://Vault/SMTP/password'
```

**Walk-through:** four parallel HTTP steps go out simultaneously, each capturing into a region-named variable. The summary step runs after all four return and prints a table; the email notification (always-on) sends the formatted body to ops.

**Variations:**

- **Pager on failure:** add a `slack:` block to the notify with `on: failure` and a pager-style channel.
- **Latency tracking:** capture the timing into a variable by wrapping the HTTP step in a shell + curl (curl can output timing info that runbook's HTTP step doesn't expose).

**Schedule:**

```sh
runbook cron add multi-region-health "0 9 * * 1"   # Mondays at 9 AM
```

---

### Pull updates and notify

**When to reach for this:** keeping a remote runbook collection in sync with upstream.

**YAML:**

```yaml
name: sync-runbooks
description: Pull the latest runbook collection from git and notify

steps:
  - name: Pull from upstream
    type: shell
    shell:
      command: 'runbook pull github.com/team/runbooks'
    capture: pull_output

  - name: Notify if new
    type: shell
    shell:
      command: 'echo "Pulled: {{.pull_output}}"'
    condition: '{{if .pull_output}}true{{end}}'

notify:
  on: success
  slack:
    webhook: 'op://Vault/Slack/runbook-sync-webhook'
```

**What happens:** step 1 invokes `runbook pull` recursively (calling the binary itself from inside a runbook step is fine — there's no parent/child runbook coupling). The output is captured. Step 2 conditions on having any output (a no-op `git pull` with "Already up-to-date" still produces output, so this fires every time — refine the condition with `grep -v "Already up to date"` if you want true silence on no-op pulls).

**Schedule:**

```sh
runbook cron add sync-runbooks "*/15 * * * *"   # every 15 minutes
```

**Notes:** `runbook pull` requires `git` on PATH. The cron-augmented PATH (see [Concepts › shell](02-concepts.md#shell)) usually picks it up from `/usr/local/bin` or `/opt/homebrew/bin`; if not, add `git` to PATH explicitly via the user's shell config.

---

## Where to go next

- [Reference](04-reference.md) — quick-lookup tables for every YAML field.
- [Troubleshooting](05-troubleshooting.md) — when a recipe doesn't behave the way you'd expect.
- [Running as a Service](06-running-as-a-service.md) — deeper coverage of `runbook cron` and log management.
- [Thinking in runbook](07-thinking-in-runbook.md) — design patterns and anti-patterns.
