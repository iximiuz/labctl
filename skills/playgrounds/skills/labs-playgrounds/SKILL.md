---
name: labs-playgrounds
description: Finds, starts, lists, and destroys iximiuz Labs playgrounds (disposable remote Linux VMs), runs commands on them, and copies files to and from them with labctl. Use when you need a sandbox VM to test or debug something, or a play ID for other labctl commands.
argument-hint: "[start <playground> | run <play-id> <command> | destroy <play-id>]"
---

Work with iximiuz Labs playgrounds using `labctl`. If arguments were given, do what they ask: $ARGUMENTS

Most commands take a **play ID**. `labctl playground start` prints it, and `labctl playground list` lists it.

## Find a playground

```sh
labctl playground catalog                      # official playgrounds: ubuntu-24-04, debian-stable, docker, k3s, k8s-omni, ...
labctl playground catalog --filter my-custom   # or: recent, popular, community
```

## Start a playground

```sh
labctl playground start <playground> --quiet --skip-wait-init
```

The `--quiet` flag makes the command print only the play ID, so capture it.
The `--skip-wait-init` flag returns right away instead of blocking on the init tasks, which may be broken while you are debugging.
To wait for the init tasks explicitly (the command exits with code 2 if one fails):

```sh
labctl playground tasks <play-id> --kind init --wait --fail-fast --timeout 5m
```

See the `labs-playground-tasks` skill for inspecting tasks.

## Find running playgrounds

```sh
labctl playground list                             # running and recent sessions
labctl playground list --all                       # also stopped and terminated ones
labctl playground list --filter challenge=<name>   # or tutorial=<name>
labctl playground status <play-id>                 # state, machines, task counters
```

## Run commands

```sh
labctl ssh <play-id> -- <command>
labctl ssh <play-id> --machine <machine> -- <command>   # multi-machine playgrounds
labctl ssh <play-id> -- bash -s < script.sh             # longer scripts via stdin
```

- The command after `--` runs on the VM, and its output is returned.
- Start long-running work detached (`nohup bash -c '<cmd>' > /tmp/out.log 2>&1 < /dev/null &`) and poll the log with short follow-up calls instead of holding one long session.
- Without a command, `labctl ssh <play-id>` opens an interactive session. It needs a real terminal, so ask the user to run it when they want hands-on access.

## Copy files

```sh
labctl cp <local-path> <play-id>:<remote-path>
labctl cp <play-id>:<remote-path> <local-path>
labctl cp -r <local-dir> <play-id>:<remote-dir>
```

## SSH tunnel discipline

`labctl ssh` and `labctl cp` borrow a tunnel slot from a small per-account pool.
Misusing it breaks **all** SSH for the session with a cryptic error:

```
labctl: Couldn't start SSH session: couldn't start tunnel: client.StartTunnel(): retry after 2562047h47m16s.
```

The `2562047h...` value is an `int64` overflow that means "no free slots". It is not a real wait time.
It is almost always caused by orphaned `labctl ssh` processes left behind by backgrounded retry loops: when the parent shell is killed, the `labctl ssh` child keeps running and holds its slot.

- Run `labctl ssh` in the foreground, one call at a time. Never wrap it in a backgrounded `for ... sleep` retry loop, and never run several calls concurrently.
- Poll the playground state with the API commands (`labctl playground status`, `labctl playground tasks`). They need no tunnel.
- To recover, run `pkill -9 -f 'labctl ssh'` and retry a single foreground call. If the error persists, restart the playground (or `labctl challenge stop` + `start`) to release the server-side slots. API commands keep working the whole time - only the tunnel is affected.

## Destroy a playground

```sh
labctl playground destroy <play-id>
```

**Never use `labctl playground stop` to get rid of a playground.** It only suspends the session for a later resume, so the playground keeps lingering around.
Destroy every playground you started for testing once you are done.
For a challenge attempt, run `labctl challenge stop <name>` instead (it tears down the attempt's playground too).
