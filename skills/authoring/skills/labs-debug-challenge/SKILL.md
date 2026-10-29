---
name: labs-debug-challenge
description: Starts, stops, and debugs an iximiuz Labs challenge with labctl - launches a fresh attempt, inspects its examiner tasks, troubleshoots the VMs, fixes the source files, pushes, and restarts until the tasks behave. Use when a challenge's tasks fail or hang, or when you need a running attempt to test a challenge.
argument-hint: <challenge-name>
---

Debug the challenge `$0` (strip the `https://labs.iximiuz.com/challenges/` prefix if a URL was given).
Follow these steps in order.

## 1. Start a fresh attempt

```sh
labctl challenge start $0 --no-open --no-ssh --skip-wait-init
```

The `--no-open` and `--no-ssh` flags keep the command non-interactive.
The `--skip-wait-init` flag makes it return right away with the **play ID** (`Challenge playground is ready! (play ID: ...)`) instead of blocking on init tasks that may be broken.
If the ID is lost, find it with `labctl playground list --filter challenge=$0`.

To check that the challenge also works for free-tier learners, add `--as-free-tier-user`.

## 2. Inspect the examiner tasks

Wait for the init tasks and read their output via the API (no SSH tunnel):

```sh
labctl playground tasks <play-id> --kind init --wait --fail-fast --timeout 5m -o yaml
labctl playground tasks <play-id> -o yaml
```

The YAML output includes each task's status, exit code, and the stdout/stderr of its last run on all machines, including the `noSSH` ones.
Tasks are edge-activated and never regress: once a task shows `completed` (`40`), it stays there for the life of the playground.
See the `labs-playground-tasks` skill for the details.

## 3. Troubleshoot on the VMs

If the task output is not enough, run diagnostics on the playground (see the `labs-playgrounds` skill):

```sh
labctl ssh <play-id> [--machine <machine>] -- <diagnostic-command>
```

Check file contents, run the task scripts by hand, inspect service status and logs, test network connectivity, and so on.
Run `labctl ssh` in the foreground, one call at a time - never in a backgrounded retry loop.

A machine with `noSSH: true` doesn't accept `labctl ssh`.
Its task status and output are still in `labctl playground tasks <play-id> -o yaml`.
Only when you need live diagnostics on that machine, temporarily comment out the directive (`#noSSH: true`), push, and restart the challenge.
Restore it when you are done.

## 4. Fix the challenge source

Find the local folder (see the `labs-content` skill) and fix the source files, typically `index.md`: init task scripts, verification conditions, expected outputs or paths, startup files, and so on.

## 5. Push and restart

A running attempt keeps serving the old version of the content, so push the changes and start a fresh attempt:

```sh
labctl content push challenge $0 --dir <folder> --force [--file index.md ...]
labctl challenge stop $0
labctl challenge start $0 --no-open --no-ssh --skip-wait-init
```

Go back to step 2 and check that the tasks pass now.
Repeat steps 2-5 until they do.

## 6. Clean up

```sh
labctl challenge stop $0
```

The stop command also tears down the attempt's playground.
If it fails while the playground is still running, destroy the playground with `labctl playground destroy <play-id>`.
**Never use `labctl playground stop`** - it only suspends the playground instead of terminating it.
