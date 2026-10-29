---
name: labs-edit-shell-gym
description: Edit-and-test loop for an iximiuz Labs Shell Gym's learning path - pushes the changes, gets them into a running gym, and validates and auto-solves the path in the VM. Use when iterating on a Shell Gym's units and you need the running gym to pick up the changes.
argument-hint: <shell-gym-name>
---

Iterate on the Shell Gym `$0` (rules: `shell-gym-authoring` - print them with `labctl skills show shell-gym-authoring` if they are not in your context).

## 1. Push the changes

Find the local folder (see the `labs-content` skill), then keep it pushed in the background:

```sh
labctl content push shell-gym $0 --dir <folder> --force --watch
```

`labctl` rebuilds `__static__/path.tar.gz` from the `path/` folder before every push (and on every change in `--watch` mode) - driven by the `source: __static__/path.tar.gz` startup file entry in `index.md`, exactly as for challenges - so the archive is never edited by hand.

## 2. Get the changes into a running gym - pick one

A Shell Gym session provisions the VM at start: the path archive is extracted into the startup file's `path` (`/opt/shellgym/path` by default; the same value as `shellgym.path` in `index.md`) and the `shellgym` daemon is started as a service. A push alone does not touch a running VM. Choose, per iteration, between:

### Option A - restart the whole Shell Gym (clean, heavier)

```sh
labctl shell-gym stop $0
labctl shell-gym start $0 --no-open --no-ssh
```

Pros: a pristine VM, the exact provisioning a student gets (archive, service, `/var/lib/shellgym` state all fresh); the only way to verify `index.md` changes (playground, `shellgym.version`/`port`, init tasks). Cons: a full playground start (tens of seconds to minutes); any manual setup on the VM is lost.

Use it after editing `index.md`, after adding modules/units that depend on a clean scene, and always for the final verification before publishing.

### Option B - copy the files and restart the daemon (fast, may be spoiled by leftovers)

With the session's playground ID (`labctl playground list`, or the ID printed by `shell-gym start`):

```sh
labctl cp -r <folder>/path <playground-id>:/tmp/path
labctl ssh <playground-id> -- sudo sh -c 'rm -rf /opt/shellgym/path && mv /tmp/path /opt/shellgym/path && systemctl restart shellgym'
```

(On OpenRC-based playgrounds such as Alpine, use `rc-service shellgym restart`.)

Pros: seconds, keeps the VM and whatever you set up in it. Cons: leftovers from the previous reps - files, processes, listening ports created by earlier `init:` scripts or by your own test runs - stay behind, and the daemon's saved progress in `/var/lib/shellgym/<path-id>` may mark units as already completed. If a unit behaves oddly after a hot swap, reset it in the gym UI (or wipe `/var/lib/shellgym` before the restart) - and confirm with Option A before trusting the result.

Use it for text tweaks and quick check-script fixes while iterating on a single unit.

## 3. Validate and auto-solve in the VM

The daemon binary is at `/opt/shellgym/shellgym` in the VM:

```sh
labctl ssh <playground-id> -- sudo /opt/shellgym/shellgym validate --path /opt/shellgym/path
labctl ssh <playground-id> -- /opt/shellgym/shellgym solve --path /opt/shellgym/path
```

Run `solve` WITHOUT `sudo`, as the path's observed user (`shellUser`, normally `laborant` - the `labctl ssh` login user). The solver types into a pty shell owned by the invoking user, and the daemon ignores exec events from any other uid: under `sudo` every `wait_exec` check times out (each unit fails after the 2-minute per-unit timeout, ~45 min for a 23-unit path). Only `validate` may run under `sudo`. A full path takes minutes, so run `solve` detached in the VM and poll the log with short `labctl ssh` calls instead of holding one long tunnel. Do NOT launch it as a plain `nohup ... &` background job from the non-interactive `labctl ssh` shell: a shell without job control starts background jobs with SIGINT (and SIGQUIT) ignored, the solver's pty shell and every command it runs inherit that disposition, and every `#!keys C-c` in a solve script becomes a no-op (the unit fails with "not completed within 2m0s" while its `sleep`/`cat` stays alive in the foreground, and every later unit then times out on a stuck shell). Reset the disposition on the way in, e.g.:

```sh
labctl ssh <playground-id> -- bash -s <<'EOF'
nohup python3 -c 'import signal,os; signal.signal(signal.SIGINT, signal.SIG_DFL); os.execvp("/opt/shellgym/shellgym", ["shellgym","solve","--path","/opt/shellgym/path"])' > /tmp/solve.log 2>&1 < /dev/null &
EOF
```

Then poll `/tmp/solve.log` (`labctl ssh <playground-id> -- cat /tmp/solve.log`) until the solver process is gone.

`validate` lints and renders the path; `solve` types every unit's `solve:` script into a real shell and reports PASS/FAIL per unit (`--unit <id>` for one unit). Run `solve` on a fresh VM (Option A) - it depends on the scene being clean. Note that a student session runs the daemon with `--live`, which strips `solve:` scripts from the on-disk files; sessions started by the gym's author run without it.

Follow the SSH tunnel discipline from the `labs-playgrounds` skill: one foreground `labctl ssh` at a time, never backgrounded retry loops.

## 4. Review before done

Once the gym is new or substantially changed and the solver passes, review it with the `labs-review-shell-gym` skill before reporting the work as done.
