---
name: labs-review-shell-gym
description: Reviews an iximiuz Labs Shell Gym before publication - security, training value, the reliability of the units' checks, writing and authoring rules, and a validate-and-solve run in a live gym - and reports the blockers and suggestions in the format of the iximiuz Labs team's review. Use when asked to review, check, or vet a Shell Gym before submitting or publishing it, and after creating a Shell Gym or substantially changing one, before reporting the work as done.
argument-hint: <shell-gym-name-or-url>
---

Review the Shell Gym `$0` (strip the `https://labs.iximiuz.com/shell-gyms/` prefix if a URL was given) and report everything that would block its publication.
Refer to the gym as `<name>` below.

The iximiuz Labs team runs the same review on community submissions before listing them in the catalog, so every blocker found here should be fixed before submitting.
The deliverable is the report (step 6).
When you review a gym that you are authoring for the user (e.g., right after creating it), fix the blockers you can, re-run the affected checks, and report what is left.
Otherwise, don't change the gym's files unless the user asks for it.

## 1. Read the source

Find the local folder, or pull the gym if there's none (see the `labs-content` skill).
Read `index.md`, `path/path.yaml`, every `module.md` and `unit.md` (with their `init:`, `check:`, `hint:`, and `solve:` scripts), and the startup files under `__static__/` other than the `path.tar.gz` archive that `labctl` builds.
Also read the `shell-gym-authoring` and `shell-gym-format-docs` rules and the general style guides (`labctl skills show <rule-name>` prints any of them if they are not in your context), and the shellgym tool's own authoring guide - especially its "Reliable checks" and "Authoring guidelines" sections: https://github.com/iximiuz/shellgym/blob/main/skills/authoring.md (or `shellgym skills authoring`).

## 2. Security

Unit `init:` scripts and the playground's init tasks run as root, so look closely at every script: keyloggers, cryptominers, reverse shells, credential or data exfiltration, downloading and executing remote code, suspicious base64 or otherwise obfuscated code, and XSS in the markdown.
Report the findings explicitly, including "nothing found".

Each of the following is a **blocker**:

- **Malicious or suspicious code** anywhere in the gym.
- **Mutable external code fetched at init** - `curl ... | sh` from a third-party URL, a `latest` download, or a container image pulled by a moving tag. The gym can break or change under the student later. Pin versions (or digests) and prefer official sources.

## 3. Training value

A Shell Gym trains command-line skills through many short, repeated reps.
A path that works technically but builds no skill is **not** ready for publication.
Read the front page, the modules, and the units from the student's seat, and judge them as a strict editor would.
Each of the following is a **blocker**:

- **The path has no focus.** It should train one coherent skill area. A tour of unrelated commands builds none of them.
- **Too little repetition.** A single rep forms no skill. Every command the path teaches should come up in many reps across the path.
- **Repetition without variation.** Five identical reps in a row, or units with no `vars:` or scenarios, so that a second session is a copy of the first. The command should keep coming up on its own in small realistic scenarios, each hitting the same skill from a different angle.
- **The units are not quick, one-action reps.** One skill per unit, and most units should complete in under a minute. A multi-step exercise that takes minutes belongs in a challenge.
- **The text gives the solution away.** The problem statement must never contain the exact command. Hints may point at it, but not paste it.

Don't downgrade these to optional polish because the solver passes or the texts are polished.
When unsure whether a non-technical issue blocks, treat it as a blocker and explain why.

## 4. Checks, writing, and authoring rules

Review every unit's checks against both failure modes (see "Reliable checks" in the shellgym guide). Each confirmed case is a **blocker**:

- **False positives** - a check completes on something that isn't the rep: a command typed earlier or elsewhere, state left over from a previous session or unit, a wrapper process whose argv mentions the command, the check's own text (`pgrep -f` and `wait_proc*` patterns without the bracket trick or `^...$` anchors).
- **False negatives** - a check never completes although the rep was done: an over-exact regex, a fast command that was missed, a builtin that never `exec`s, a check killed by its own `--timeout`.
- **Negative checks without a baseline** - a task verifying that something *disappears* must first confirm that it existed, or it completes before the scene exists.

Then flag the authoring issues, even when the tooling tolerates them:

- the module and unit folders aren't `NNN.name` with lowercase letters, digits, and dashes only;
- a unit has no `solve:` script, or its `needs:` chain is broken;
- the startup file's `path` and `shellgym.path` in `index.md` differ, or `__static__/path.tar.gz` is hand-made instead of built by `labctl`;
- scenes in the wrong place for their lifetime (`/tmp/gym` for standalone units, the student's home for state that later units build on);
- files created by root in `init:` but not `chown`ed to `$GYM_USER`, or student-owned processes started without `systemd-run --uid=$GYM_USER`;
- distro-specific commands in units without `labels:`, or random tokens generated with `openssl` (it isn't installed everywhere).

Finally, go through the `shell-gym-authoring` rules and the general style guides and flag every violation, naming the rule: the "Shell Gym" naming, the short factual front page, terse imperative unit texts, sentences that start with a code token, the word "rep" in student-facing text, and completion messages longer than one short sentence.

## 5. Test it in a live gym

A gym session always runs the server's version of the content.
If the local folder has changes that aren't pushed yet, ask the user whether to push them before testing.

1. **Start a fresh session** as a free-tier student would see it:

   ```sh
   labctl shell-gym start <name> --as-free-tier-user --safety-disclaimer-consent --no-open --no-ssh --skip-wait-init
   ```

   The command prints the play ID. Wait for the playground's init tasks (see the `labs-playground-tasks` skill):

   ```sh
   labctl playground tasks <play-id> --kind init --wait --fail-fast --timeout 5m -o yaml
   ```

   A failing or hanging init task is a blocker.

2. **Find the path to test.** The learning path is extracted to `shellgym.path` from `index.md` (`/opt/shellgym/path` by default).
   If its `unit.md` files have no `solve:` scripts, the daemon runs in live mode (only the gym's author gets a session without it), so copy the local `path/` folder into the VM and use the copy as the solver's `--path`:

   ```sh
   labctl cp -r <folder>/path <play-id>:/tmp/review-path
   ```

3. **Validate it:**

   ```sh
   labctl ssh <play-id> -- sudo /opt/shellgym/shellgym validate --path <path>
   ```

   Any error is a blocker.

4. **Solve it.** Run `/opt/shellgym/shellgym solve --path <path>` in the VM exactly as step 3 of the `labs-edit-shell-gym` skill describes: as the shell user (never with `sudo`), detached, with the SIGINT disposition reset, polling its log with short `labctl ssh` calls.
   Every unit must pass. A failing unit is a blocker - either its check or its `solve:` script is broken.

5. **Check for flakiness.** Stop the session (`labctl shell-gym stop <name>`), start a fresh one, and solve the path again. Both runs must pass.

6. **Clean up** with `labctl shell-gym stop <name>`.

Follow the SSH tunnel discipline from the `labs-playgrounds` skill: one foreground `labctl ssh` call at a time.

## 6. Report

Report the findings in this format (in the conversation, or in a file if the user asks for one):

```markdown
# Review: <Shell Gym Title>

**Verdict: ready for publication**        <- or: requires adjustment before publication

<findings - see below>
```

- **Start with the findings right after the verdict.** No opening praise paragraph, no restating what the gym is about.
- **The verdict is `requires adjustment before publication`** when there is at least one blocker - from security (step 2), training value (step 3), checks (step 4), or the live test (step 5). Lead with an intro line like "One change is required before publication:", then a numbered list with one item per blocker. Each item starts with a **bold one-sentence claim**, followed by the evidence (the unit id, the script line, the solver output) and a concrete fix the author can act on. After that, list the non-blocking items under a line like "Optional polish, not blocking:". End the report there.
- **For a training-value blocker,** name the units involved and propose the change: which reps to add or merge, how to vary them, and which skill area to focus on.
- **The verdict is `ready for publication`** only when nothing blocks. List the observations under an intro line like "A few small suggestions, none blocking:".
- Keep the bullets concrete and factual: name the unit or file in backticks, state what was observed and why it matters, and point at a fix direction. Skip trivia that doesn't help the author.
- Follow the general style guides: simple International English, hyphens (never em dashes), no hooks or filler.

After the report, summarize the live test results (init tasks, `validate`, and both solver runs with their per-unit results).
