# runbook User Guide

A comprehensive, recipe-driven guide for power users of the runbook CLI. For a feature overview or install instructions, start with the [project README](../../README.md). For exhaustive command, flag, and YAML-field reference, see the [man page](../../runbook.1) (or `runbook man | man -l -`).

## Who this guide is for

You're comfortable on a terminal and can edit YAML. You don't need to be a Go programmer. The guide explains the moving parts as they come up — from the first hello-world runbook through SSH, HTTP, 1Password secrets, scheduled runs, and notifications.

## Contents

1. [Getting Started](01-getting-started.md) — install runbook, write your first runbook, and watch a step run. ~10 minutes.
2. [Concepts](02-concepts.md) — how steps, variables, the executor, error policies, capture, conditions, parallel groups, log files, and notifications fit together.
3. [Cookbook](03-cookbook.md) — recipes for every step type, plus end-to-end runbooks for deploys, health checks, backups, and scheduled jobs.
4. [Reference](04-reference.md) — quick-lookup tables for subcommands, flags, the YAML schema, template variables, and on-disk locations.
5. [Troubleshooting](05-troubleshooting.md) — symptom-driven fixes for the most common failure modes.
6. [Running as a Service](06-running-as-a-service.md) — using `runbook cron` to schedule unattended runs, plus a tour of where logs land and how to keep them tidy.
7. [Thinking in runbook](07-thinking-in-runbook.md) — design patterns, an iterative authoring workflow, migration recipes from shell scripts and Ansible playbooks, and anti-patterns to skip past.

## Platform support

runbook is a single Go binary. The shell, SSH, and HTTP step types are portable across **macOS, Linux, and Windows**. The 1Password CLI integration uses the platform-native keychain on each OS (macOS Keychain, GNOME Secret Service on Linux, Windows Credential Manager). `runbook cron` requires the system `crontab` and is therefore Unix-only — on Windows, schedule runs through Task Scheduler instead. Desktop notifications use a different backend per OS (`osascript` on macOS, `notify-send` on Linux, PowerShell on Windows). Anything platform-specific is called out inline throughout the guide.
