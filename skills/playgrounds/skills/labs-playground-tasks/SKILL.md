---
name: labs-playground-tasks
description: Lists, inspects, and waits for the examiner tasks (init, helper, and verification tasks) of a running iximiuz Labs playground with labctl. Use to check whether a playground finished initializing, why a task fails or hangs, or whether a challenge or tutorial task passed.
argument-hint: "<play-id> [task-name]"
---

Inspect the examiner tasks of playground `$0` (focus on the task `$1` if it is given).

Tasks are shell scripts that the **examiner**, an agent inside every playground VM, runs to prepare the environment (init tasks) and to verify the user's progress (regular tasks).

## List tasks and their last-run output

```sh
labctl playground tasks <play-id>                # name, machine, status
labctl playground tasks <play-id> -o yaml        # adds exit codes and the stdout/stderr of the last run (incl. hintcheck/failcheck)
labctl playground tasks <play-id> --kind init    # or: helper, regular
```

This command reads from the API. It needs no SSH tunnel and covers **all** machines at once, including the `noSSH` ones, so prefer it over SSH for both inspection and polling.
It shows the last run only and never triggers a new one.
The task scripts and their output are visible only to the author of the content or playground.

## Wait for tasks

```sh
labctl playground tasks <play-id> --kind init --wait --fail-fast --timeout 5m -o yaml   # init tasks only
labctl playground tasks <play-id> --wait --fail-fast --timeout 10m -o yaml              # all non-helper tasks
```

The `--wait` flag polls until the tasks settle, and `--fail-fast` stops at the first failed task.
The command exits with code 2 if a task failed or the timeout expired, so always pass `--timeout`.
Use it instead of hand-written polling loops.
Verification tasks complete only once the learner's goal is achieved, so waiting for all tasks of an unsolved challenge ends with the timeout.

To block until one specific task settles, use the in-VM `examinerctl` CLI (a single foreground call - see the SSH tunnel discipline in the `labs-playgrounds` skill):

```sh
labctl ssh <play-id> -- examinerctl task wait <task-name> --timeout 120s
```

The `examinerctl task wait` command only waits.
There is no way to trigger, start, or re-run a task by hand, because the examiner evaluates the tasks on its own.

## Task statuses

Tasks are **edge-activated**, and their status **never regresses**.
The examiner keeps evaluating a task until it first observes success or failure.
Then the task settles into a terminal state - `completed` (`40`) or `failed` (`35`) - and never runs again, even if the world state changes later.
A single `completed` observation is authoritative: a passed task cannot fail later.
To run the tasks again, start a fresh playground (or a fresh challenge attempt).

## Look inside a machine

The `examinerctl` CLI also lists and shows the tasks of the machine you SSH into (one machine per call, and each call takes a tunnel slot):

```sh
labctl ssh <play-id> [--machine <machine>] -- examinerctl task ls
labctl ssh <play-id> [--machine <machine>] -- examinerctl task get <task-name>
```

For machine-level problems, such as boot failures or crashed services, read the machine logs:

```sh
labctl playground machine console <play-id> <machine>                        # serial console, one section per boot
timeout 30s labctl playground machine journal <play-id> <machine> -n 500     # systemd journal (-u <unit> to narrow down)
```

The `journal` command follows the journal until interrupted, so always cap it with `timeout`.
