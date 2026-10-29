---
paths:
  - "**/challenges/**"
  - "**/tutorials/**"
  - "**/courses/**"
---

# Challenge Authoring

Practical rules and pitfalls learned from authoring challenges. These complement the format reference in the `challenge-format-docs` rule.

## Review Before Done

When you have created a challenge or substantially changed one, review it with the `labs-review-challenge` skill before reporting the work as done, and fix the blockers it finds.
The iximiuz Labs team runs the same review on community submissions, so a challenge that passes it is ready to be submitted.
Small edits (a typo, a reworded hint) don't need a full review.

## Style Guides

Use simple, clear, and concise "International English". Avoid fancy words and overly complex sentences. Write each challenge as a natural story, tailored for the corresponding problem statement in the index.md file. Using structured prose is preferred over bullet points.

The problem statement in the index.md file should focus on the challenge itself, not the solution.
Hinting at the solution is fine, especially in (collapsed by default) hint boxes.
Hints may name commands and show their flags and arguments (e.g., "the `--add-host <name>:host-gateway` flag of `docker run`"),
but not every hint should spell out the command the learner needs - most hints should point to the idea or the place to look.
The only strict no-go (unless the author asks otherwise) is the exact copy/pastable command that solves the task.

Always try to connect the dots between the challenge and the real-world scenarios it prepares the student for and/or particular skills it helps develop.

Challenge descriptions should be concrete and factual. It should both outline the problem to be solved and the value the exercise provides to the learner.
But do not add an explicit "Why This Matters" section, especially at the end of the problem statement.
Instead, the importance of the problem should be naturally highlighted by the problem statement's framing and phrasing.

Per-machine welcome messages should be concise and factual. They shouldn't restate the problem. Instead, they should describe the machine and its purpose, as well as highlighting its key components (if relevant for the challenge).

## Task Conventions

- **Single-machine challenges: omit the `machine:` attribute on tasks.** The `machine:` field only exists to disambiguate which VM a task runs on in multi-machine topologies. When the playground has exactly one machine, every task implicitly targets it, so spelling out `machine: <the-only-machine>` on each task is redundant noise. Add `machine:` only once a challenge has two or more machines.
- **Don't specify `timeout_seconds` unless you have a concrete reason.** The default task timeout is **60s**. If a task is expected to finish in under a minute and does not need a tighter bound, leave `timeout_seconds` off entirely. There are exactly two reasons to set it explicitly:
  - **Quick turnaround** - the task does a fast operation and you want it to fail fast (e.g., a snappy check or a probe that should never hang), so you set a value *lower* than 60s.
  - **Long task** - the operation genuinely needs more than 60s (e.g., a large image pull, a `kubeadm` bring-up, a multi-step build), so you set a value *higher* than the default.

  If neither applies, omitting `timeout_seconds` keeps the frontmatter clean and signals "ordinary, sub-minute task".

## Setup Artifacts: Never Inline, Always Ship as a Startup File

- **NEVER inline a mini-program, `Dockerfile`, config, or any script-ish artifact longer than ~a dozen lines into an `index.md` task `run:` block.** Embedding a server, checker, or `Dockerfile` as a heredoc inside a YAML block scalar is brittle and a recurring source of bugs: YAML re-indentation silently mangles Python/Makefile indentation, quoting and `$(...)`/backticks/`$VAR` need fragile escaping, and the artifact becomes impossible to review, lint, or test in isolation. Store it as a real file under the challenge's `__static__/` folder and declare it as a `playground.startupFiles` entry instead. Short snippets (a `mkdir`, a few `echo`s, a one-liner check) are fine to keep inline.
- **How this works.** A `startupFiles` entry with `source: __static__/<file>` is fetched and cached by the platform, then baked into the machine's filesystem *before it boots* - before any init task or login session. It only becomes available *after* `labctl content push`, but there's no cache-buster to manage: each start always gets the current content, so editing an artifact is just re-pushing it. Set `owner`/`mode` (or `append: true`) on the entry, and `machines: [...]` if it should only land on some machines.

### Single file

Put e.g. `telemetry_server.py` in `__static__/`, then declare it as a startup file:

```yaml
playground:
  startupFiles:
    - path: /opt/iximiuz-labs/telemetry_server.py
      source: __static__/telemetry_server.py
      owner: root
      mode: "644"
```

### Multiple related files → a folder that `labctl` archives for you

When the setup is a small project (e.g. `Dockerfile` + `main.go` + `go.mod`, or a server plus its checker), keep the sources in a sibling subfolder of the challenge (conventionally `app/`) and declare `__static__/<folder>.tar.gz` (or `.tgz`, `.tar`) as a single startup file with `extract: true`. There is no build step: `labctl content push` notices a startup file whose `source` is `__static__/app.tar.gz` next to an `app/` folder and creates the archive from the folder's files itself - on every push, and on every change in `--watch` mode. The platform unpacks the archive into `path` before the machine boots, so there's no init task to unpack it at all:

```yaml
playground:
  startupFiles:
    - path: /home/laborant/app
      source: __static__/app.tar.gz
      extract: true
      owner: laborant
```

- **No `Makefile`, no `make build`, no `tar` step.** The folder is the source of truth; the generated `__static__/<folder>.tar.gz` is a build artifact that doesn't need to be committed. (A hand-made archive still works - `labctl` only builds one when a sibling `<folder>/` exists.)
- **`.labctlignore` the folder** (`app/`) so its files aren't also pushed as loose content files - `labctl` keeps watching it anyway. A `.labctlignore` *inside* the folder excludes files from the archive too (e.g. `README.md`). Without the ignore, `labctl content push` may 400 with `Content kind challenge does not support files with kind undefined` on human-facing files.
- **Editing an artifact later** means editing the file (or the folder) and re-pushing - the next challenge start always fetches the current version, no cache-buster needed.

## User Input Tasks

- **Use `x(.input)` to read user-submitted values, not a `destination:` file.** The `destination:` field writes the input to a file on disk; `x(.input)` injects it directly into the task script at the point of use. Prefer `x(.input)` - it is cleaner and avoids disk writes. The `destination:` field is deprecated and exists only for backwards compatibility.
- **In `run:`, assign `x(.input)` to a shell variable first**, then work with the variable. Example:

  ```yaml
    verify_something:
      run: |
        PROVIDED=$(echo "x(.input)" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
        if [ -z "${PROVIDED}" ]; then
          echo "No answer provided."
          exit 1
        fi
        ...
  ```

- **In `.solution.sh`, submit answers with `examinerctl task input`**, then wait for the task to pass:

  ```sh
  echo -n "yes" | examinerctl task input verify_something
  examinerctl task wait verify_something --timeout 30s
  ```

## Sharing Data Between Tasks: Prefer `stdout` Over a File

- **Pass values between tasks via `x(.needs.<task>.stdout)`, not by writing to a shared file on disk.** When one task produces a value another task consumes (a generated secret, a computed digest, a discovered IP/port, a token), have the producer task simply print it to stdout and have the consumer reference it through `env:` + `x(.needs.<producer>.stdout)`. This is the default; reach for a file only when the artifact is genuinely large (e.g. a multi-KB cert bundle, a tarball, a generated dataset) or must live at a specific path for a long-running process to read.
- **Why stdout is better for small values.** A file is visible to anyone with shell access to that machine - for a *secret* shared between a server and its verifier, an on-disk copy is a leak waiting to happen, especially on the student-facing machine. The `stdout` channel is internal to the examiner: the student never sees init-task stdout, so the value stays out of their reach. It also keeps the data flow explicit in the `needs:`/`env:` graph instead of relying on an implicit file convention, and avoids file-permission/ownership pitfalls across machines and users.
- **Prefer binding `x(...)` in the `env:` block over using it inline in `run:`.** It works in either place. The benefit of binding it once in `env:` (e.g. `- SECRET=x(.needs.init_gen_secret.stdout)`) is that the value is then available as `$SECRET` across all of the task's scripts - `run:`, `hintcheck:`, and `failcheck:` - instead of being re-expanded in each one.
- **Producer task: emit exactly the value, no trailing noise.** Print just the value (e.g. `openssl rand -hex 32`, or `sha256sum file | awk '{print $1}'`). Because the captured stdout may pick up a trailing newline, normalize on the consuming side - `tr -d '[:space:]'` for shell comparisons, or `.strip()` in Python - so both sides agree byte-for-byte (a mismatched HMAC key from a stray `\n` is a maddening bug).
- **Cross-machine flow:** any task (init or verify) may `needs:` a task on *another* machine and read its `stdout` - cross-machine `needs:` is fully supported, including `init → init`. The only gotcha: cross-machine dependencies are never implicit (only same-machine init tasks are auto-added to a regular task's `needs:`), so an init task from another machine must be listed in `needs:` explicitly. Example: `init_gen_secret` runs on the `noSSH` server machine, a long-running server reads the secret via `systemd-run --setenv=...`, and the verifier on the dev machine pulls the *same* secret via `x(.needs.init_gen_secret.stdout)` - no copy of the secret ever touches disk on either box.

### Secret Generation

- **Prefer `openssl rand -hex <n>` for generating secrets/tokens.** It is concise, emits only hex (no whitespace, no `+`/`/`/`=` that need quoting when passed through `--setenv`, env vars, or shell), and is available on the playground VMs. Example: `openssl rand -hex 32`. Fall back to `head -c <n> /dev/urandom | od -An -tx1 | tr -d ' \n'` only if `openssl` is genuinely unavailable on the target image. Avoid `base64` for secrets that flow through env/`--setenv`/comparisons - its `=` padding and `+`/`/` chars are quoting hazards.

## Playground & VM Environment

- **`flexbox` playground** is the base for custom multi-VM network topologies. Define `networks:` and `machines:` with explicit `network.interfaces` and addresses.
- **`noSSH: true`** hides a machine from the student's UI but does NOT prevent inter-machine SSH between playground VMs.
- **Debugging `noSSH` machines:** To inspect examiner *task status and last-run output* on a `noSSH: true` machine, just use `labctl playground tasks <playID> -o yaml` - it is served via the API and covers all machines, including hidden ones, so you do **not** need to comment out `noSSH: true`. Only temporarily comment it out (`#noSSH: true`, then push + restart) when you need to actually `labctl ssh` into the machine to run live diagnostic commands. Remember to restore `noSSH: true` when done.
- **curl is pre-installed** on Ubuntu playground VMs - no need for a separate install task.

## Container Image Extraction

- **`crane export` writes a tar stream to stdout.** Always extract to a temp directory with `tar xf - -C "$tmpdir"` to avoid path collisions with existing files on the VM (e.g., `/var/www/html/` may already exist).

## Content Naming

- **Remote names SOMETIMES get hex suffixes.** E.g., local `linux-protect-ports/` becomes `linux-protect-ports-b76c415e` on the server. The `labs-content` skill explains how to find the local folder reliably.
- **`labctl content create` before first push.** The challenge must exist on the server before `labctl content push` will work.

## Solution Files

- **Human-facing:** `solution-01.md`, `solution-02.md`, etc. Use YAML frontmatter with `title:` and `<!--more-->` to separate summary from detail.
- **Scripted:** `.solution-01.sh`, `.solution-02.sh`, etc. (hidden dot-prefix). Minimal scripts that apply the solution non-interactively, so the challenge can be re-tested end-to-end (see "Apply a single-VM solution in one shot" below).
- **Remove the default `solution.md`** when using numbered solution files (but challenges with just one solution should keep it).
- **Never start a solution with the "The key to this challenge is" phrase.** It's redundant and annoying. Write each solution as a natural story, tailored for the corresponding problem statement in the index.md file.

## Task Completion Messages

The `#completed` slot of a task component (`::simple-task`, `::user-input-task`) is shown only after the task
has been solved, and the learner often never reads it. Its only purpose is to confirm that the action was
performed correctly and, optionally, to congratulate or motivate the learner. Keep it to one short sentence:
"Correct! The service reports this version." or "The record was replaced."

NEVER put important information there: no explanations of why the answer is what it is, no facts about
the tool or protocol the learner needs later, no pointers to the next task. Anything worth teaching goes
where the learner reads it while working - the task description above the component, a hint box, or a
remark box after the task.

- Bad: `#completed` "Correct! A `201 Created` response also carries the URL of the new record in its `Location` header."
- Good: task text mentions the `Location` header, `#completed` says "The artifact is registered."

## Hints

- **Use the `hint-box` component** to show collapsed by default clues on how to solve the challenge.
- Never put the exact copy/pastable command that solves the task in a hint box (unless the author asks for it).
  Mentioning commands, flags, and arguments is fine when it helps, e.g. "`--network host` shares the host's network namespace" or
  "the `--add-host <name>:host-gateway` flag". Just do not turn every hint into the needed command - keep most hints at the level of
  the idea or the place to look.

### Hint summary format

The `:summary:` of a `hint-box` allows only three formats:

- `Hint <Seq No>` - numbered hints for a small linear sequence of clues (`Hint 1`, `Hint 2`, ...).
- `Hint <Seq No>: <Short title>` - numbered hints whose subject is worth naming (`Hint 1: Where to start looking`).
- `Hint: <Short title>` - a descriptive label when the hint is standalone or its subject matters more than the number.

Quote the value in the two titled formats because of the colon.

```md
::hint-box
---
:summary: Hint 1
---

Use tools that list block devices and look for one with no partitions.
::

::hint-box
---
:summary: "Hint 2: Finding the blank drive"
---

Use tools that list block devices and look for one with no partitions.
::

::hint-box
---
:summary: "Hint: Finding the blank drive"
---

Use tools that list block devices and look for one with no partitions.
::
```

No other formats are allowed. Do not use other separators after the number (`Hint 1 - Where to start looking`),
and do not drop the `Hint` prefix (`Frontend debugging hint`, `ENTRYPOINT vs CMD`). Keep the short title to a few words.

### Dynamic (file-based) hints for verification tasks

Static hint boxes appear regardless of what went wrong. Dynamic hints are generated by the
`run:` script based on what it actually observed, so the message is relevant to the specific
failure. Use them for any task with more than one plausible failure mode.

**Pattern:** the `run:` script writes a context-specific message to a temp file before
`exit 1`; the `hintcheck:` script reads and prints that file, then cleans it up.

```yaml
tasks:
  verify_config_file:
    machine: server-01
    run: |
      rm -f /tmp/verify_config_file_hint.txt

      if [ ! -f /etc/myapp/config.toml ]; then
        echo "The config file is expected at /etc/myapp/config.toml." | tee /tmp/verify_config_file_hint.txt
        exit 1
      fi

      if ! grep -q '^\[server\]' /etc/myapp/config.toml; then
        echo "The file exists but is missing a [server] section. Check the expected format with 'myapp --print-default-config'." | tee /tmp/verify_config_file_hint.txt
        exit 1
      fi

      PORT=$(grep -E '^port\s*=' /etc/myapp/config.toml | awk -F= '{print $2}' | tr -d ' ')
      if [ "${PORT}" != "8080" ]; then
        echo "port= is set to '${PORT}' but the expected value is 8080." | tee /tmp/verify_config_file_hint.txt
        exit 1
      fi

      echo "Config file is correct."

    hintcheck: |
      if [ -f /tmp/verify_config_file_hint.txt ]; then
        cat /tmp/verify_config_file_hint.txt
        rm -f /tmp/verify_config_file_hint.txt
      fi
```

Rules:
- `hintcheck:` **must always exit 0**, regardless of what it outputs. It is informational only.
- `failcheck:` exits non-zero to fail a task that `run:` passed. Use it when the main check
  is expensive and a secondary invariant must also hold. It follows the same file pattern.
- Clean up the hint file at the top of `run:` (`rm -f`) so a stale file from a previous run
  does not mislead the user after a fix.
- Hint messages should point to what is wrong or where to look - not how to fix it. Never print the exact
  copy/pastable command that solves the challenge.

## Testing Workflow

- **NEVER validate challenge artifacts locally. Always use a remote playground via `labctl`.**
  - This includes (non-exhaustive): running scripts embedded in `index.md` (Python, shell, Go, etc.) on the host;
    invoking helpers like `systemd-socket-activate`, `docker`, `crane`, `curl`, `nft`, `iptables`, `ip netns`, `qemu-*`
    against locally-extracted binaries or files; pasting heredoc bodies into local files to "syntax-check" them
    with `python3 -m py_compile`, `bash -n`, `node --check`, etc.; spinning up local containers or VMs to
    reproduce the challenge environment.
  - **Why:** the host machine is not the playground. It has a different distro, kernel, package set, user, network,
    and pre-installed tools. A local pass tells you nothing about whether the challenge works in the actual VM,
    and a local fail can be a host-only artifact that wastes time. Local execution can also leak state into the
    author's environment.
  - **How to apply:** the only acceptable validation path is a challenge attempt (the `labs-debug-challenge` skill) or a
    playground (the `labs-playgrounds` skill) with the content pushed (the `labs-content` skill), inspected with the
    `labs-playground-tasks` skill and `labctl ssh`. Syntax-level sanity (YAML parses, frontmatter keys present) on the markdown file
    itself is fine; executing any code that the challenge ships is not.
- **NEVER use `labctl playground stop` - terminate playgrounds with `labctl playground destroy <playID>`.** `playground stop` does NOT terminate the playground: it suspends it, preserving its state for a later resume, so the session keeps lingering around. Every ephemeral playground started for authoring/testing must be destroyed when done. For a challenge attempt, the right command is `labctl challenge stop <name>` (it tears down the attempt's playground too); only if it fails, force the teardown with `labctl playground destroy <playID>`.
- **NEVER run `labctl auth login` or `labctl auth logout`** (see the `iximiuz-labs` rule). To test a challenge as a free-tier user, start it with `labctl challenge start <name> --as-free-tier-user` instead of switching accounts.
- **Tasks are edge-activated (NOT level-triggered) and their status NEVER regresses.** The examiner keeps evaluating a task until it *first* observes a success or failure state; at that moment the task settles into its terminal state - `completed` (`40`) or `failed` (`35`) - and **never runs again**, even if the world state later changes and the original passing/failing condition no longer holds. The numerical status only ever increases; it cannot flip back to `running`/`blocked` or to the opposite terminal state. So a single observation of `completed` is authoritative, and a task that has already passed cannot later fail.
- **`examinerctl task wait <task> --timeout <dur>` only WAITS; it does not trigger, start, or re-run anything** (there is no `task start`/`task run` - you cannot manually fire a task). The examiner evaluates the task on its own; `task wait` just blocks until the task reaches a terminal state (or the timeout). A `.solution-NN.sh` ends each step with `examinerctl task wait <verify-task>` so applying the solution blocks until that task has settled.
- **Wait with `labctl playground tasks <id> --wait --fail-fast --timeout <dur>`, not long sleeps.** It polls the task status via the **API (no SSH tunnel)** until the tasks settle and exits with code 2 if a task fails or the timeout expires (`--kind init` waits for the init tasks only). Add `-o json` or `-o yaml` for machine-readable status codes (`status: 40` == completed). A VM reboot in these playgrounds comes back in **a few seconds**; cap any post-reboot wait at ~20s - if the box isn't back by then, something is wrong (don't schedule multi-minute waits).
- **SSH tunnel discipline.** `labctl ssh` borrows a slot from a small per-account tunnel pool. NEVER background a `labctl ssh` retry loop and never fire several `labctl ssh` calls concurrently: when the parent shell is killed the `labctl ssh` child is orphaned and keeps holding its slot, and once the pool is exhausted **every** later ssh fails with the cryptic `client.StartTunnel(): retry after 2562047h47m16s` (an overflow, not a real wait). Run ssh foreground, one at a time; poll status via `labctl playground tasks` (API), and reserve ssh for a single `examinerctl task wait <task>` or a diagnostic. Recovery: `pkill -9 -f 'labctl ssh'`, then `labctl challenge stop`+`start` if it persists.
- **To INSPECT a task's last-run output, prefer the tunnel-free `labctl playground tasks <id> -o yaml` over `labctl ssh -- examinerctl task get <task>`.** The YAML form is served via the API (no SSH tunnel), covers **all** machines at once (including `noSSH` ones - so you no longer need to comment out `noSSH: true` just to read a task's output), and includes each task's status plus its last-run stdout/stderr. Reserve `examinerctl` over SSH for the one thing the API can't do: **blocking** until a task reaches its terminal state (`examinerctl task wait`).
- **Apply a single-VM solution in one shot:** `{ echo 'set -exuo pipefail'; cat .solution-NN.sh; } | labctl ssh <id> -- bash -s`. Do NOT use `$(cat file)` inside a quoted `<<'CHUNK'` heredoc - it is sent literally and runs on the VM, where the file does not exist.
- **Test each solution from a fresh start.** Stop the challenge between solution tests to ensure no leftover state from previous attempts.
