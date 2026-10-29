---
name: labs-review-challenge
description: Reviews an iximiuz Labs challenge before publication - security, learning value, writing and authoring rules, and a test run on a live playground - and reports the blockers and suggestions in the format of the iximiuz Labs team's review. Use when asked to review, check, or vet a challenge before submitting or publishing it, and after creating a challenge or substantially changing one, before reporting the work as done.
argument-hint: <challenge-name-or-url>
---

Review the challenge `$0` (strip the `https://labs.iximiuz.com/challenges/` prefix if a URL was given) and report everything that would block its publication.
Refer to the challenge as `<name>` below.

The iximiuz Labs team runs the same review on community submissions before listing them in the catalog, so every blocker found here should be fixed before submitting.
The deliverable is the report (step 6).
When you review a challenge that you are authoring for the user (e.g., right after creating it), fix the blockers you can, re-run the affected checks, and report what is left.
Otherwise, don't change the challenge files unless the user asks for it.

## 1. Read the source

Find the local folder, or pull the challenge if there's none (see the `labs-content` skill).
Read `index.md`, every solution file (`solution*.md`, `.solution*.sh`), and every startup file under `__static__/` (and the folders that `labctl` archives into them).
Also read the `challenge-authoring` and `challenge-format-docs` rules and the general style guides (`labctl skills show <rule-name>` prints any of them if they are not in your context).

## 2. Security

Look for malicious content in the tasks, the startup files, and the solutions: keyloggers, cryptominers, XSS in the markdown, reverse shells, credential or data exfiltration, downloading and executing remote code, suspicious base64 or otherwise obfuscated code.
Report the findings explicitly, including "nothing found".

Each of the following is a **blocker**:

- **Malicious or suspicious code** anywhere in the challenge.
- **Unpinned external software that can plausibly break the challenge** - an install script (`curl ... | sh`), a `latest` download, or a moving image tag of a fast-moving tool in active development, whose next release can change what the challenge depends on (a CLI flag, an output format, a config schema, a bundled tool). The challenge can break or change under the learner later. Pin these versions (or digests) and prefer official sources.

Assess every unpinned install case by case. Stable software tolerates it, and an unpinned install is usually better than a pinned version, which ages and needs regular bumps. Examples are official images like `alpine`, `busybox`, or `nginx:alpine`, and the official install script or `latest` release of a mature tool. Don't flag these unless the challenge depends on a detail that changes between releases, for example output that the checks compare or a package that a newer release may rename. An install script or download from an unofficial source is a security question first: review it as possible malicious code.

## 3. Learning value

A challenge that works technically but teaches little is **not** ready for publication.
Read the title, the description, the scenario, the task section, and the hints from the learner's seat, and judge them as a strict editor would.
Each of the following is a **blocker**:

- **The framing promises a skill that the task never exercises.** A "Troubleshoot ...", "Debug ...", or "Find out why ..." challenge whose scenario already names the cause, or whose task section spells out the fix (the resource kind, type, field, and values to set). The test: can a learner pass by following the text literally, without doing the diagnosis or making the decision the title promises? If yes, it is a blocker.
- **The problem statement is a recipe.** A numbered list that restates a docs page ("create X with these properties, add field Y, apply it") leaves the learner nothing to figure out. Constraints the checks need (a resource name, a namespace, a port) are fine to give. The approach (which mechanism, which command, which field) belongs to the learner and the hints.
- **The scenario is implausible or inconsistent.** The story should be a situation that happens in real work, and its details should hold on the live playground (cross-check them in step 5).
- **The hints or the text give the solution away.** A hint may name a command and its flags, but the exact copy/pastable command that solves the task is a no-go.

Don't downgrade these to optional polish because the tasks pass, the write-up is polished, or the solution is thorough.
When unsure whether a non-technical issue blocks, treat it as a blocker and explain why.

## 4. Writing and authoring rules

Confirm that the challenge is clearly written and reasonable.
Flag authoring issues even when the platform tolerates them, for example:

- malformed YAML indentation in `tasks:`, broken `needs:` chains, or a task depending on another machine's init task without listing it in `needs:` (cross-machine `needs:` is supported but never implicit);
- a negative-condition verification task (e.g., "the port is unreachable") without an init task that first confirms the opposite, so it can pass before the learner does anything;
- `ssh` in a task without `-nT`, a `hintcheck:` that can exit non-zero, a `failcheck:` with the wrong exit code logic;
- scripts, servers, or configs longer than a dozen lines inlined into a task's `run:` instead of shipped as startup files.

Also flag **narrative/diagnostic mismatches**: the symptom the text tells the learner to look for differs from what actually happens (cross-check against the live state in step 5).
For example, a "the pod is OOMKilled, run `kubectl describe pod`" story when the pod is in fact *rejected at admission* and never exists to describe.
The fix the challenge teaches may still be correct while the explanation misleads the learner.

Then go through the `challenge-authoring` rules and the general style guides section by section and flag every violation, naming the rule: the title and name conventions, the task conventions, user input tasks, hints, completion and welcome messages, solution files, and the prose style.

## 5. Test it on a live playground

A challenge attempt always runs the server's version of the content.
If the local folder has changes that aren't pushed yet, ask the user whether to push them before testing.

1. **Start a fresh attempt** as a free-tier learner would see it:

   ```sh
   labctl challenge start <name> --as-free-tier-user --safety-disclaimer-consent --no-open --no-ssh --skip-wait-init
   ```

   Then wait for the init tasks (see the `labs-playground-tasks` skill):

   ```sh
   labctl playground tasks <play-id> --kind init --wait --fail-fast --timeout 5m -o yaml
   ```

   A failing or hanging init task is a blocker.

2. **Check the initial state.** Confirm with `labctl ssh <play-id> [--machine <machine>] -- <diagnostic>` that the playground matches the scenario (the broken service is broken, the files and processes the text mentions exist, and so on).
   Run `labctl playground tasks <play-id> -o yaml`: a verification task that is already `completed` before the learner did anything is a blocker.

3. **Solve it with the author's solution.** Apply `.solution*.sh` if there is one:

   ```sh
   { echo 'set -exuo pipefail'; cat <folder>/.solution.sh; } | labctl ssh <play-id> -- bash -s
   ```

   Otherwise, follow `solution*.md` step by step with `labctl ssh`.
   For multi-machine challenges, run each part on its machine (`--machine <machine>`).
   Then wait for all tasks:

   ```sh
   labctl playground tasks <play-id> --wait --fail-fast --timeout 10m -o yaml
   ```

   A verification task that the advertised solution can't pass is a blocker. So is a solution step that doesn't work as written.

4. **Check for flakiness.** Stop the attempt (`labctl challenge stop <name>`), start a fresh one, and solve it again. Both runs must pass.

5. **Clean up** with `labctl challenge stop <name>` (see the `labs-debug-challenge` skill if the playground lingers).

Follow the SSH tunnel discipline from the `labs-playgrounds` skill: one foreground `labctl ssh` call at a time.

## 6. Report

Report the findings in this format (in the conversation, or in a file if the user asks for one):

```markdown
# Review: <Challenge Title>

**Verdict: ready for publication**        <- or: requires adjustment before publication

<findings - see below>
```

- **Start with the findings right after the verdict.** No opening praise paragraph, no restating what the challenge is about.
- **The verdict is `requires adjustment before publication`** when there is at least one blocker - from security (step 2), learning value (step 3), or the live test (step 5). Lead with an intro line like "One change is required before publication:", then a numbered list with one item per blocker. Each item starts with a **bold one-sentence claim**, followed by the evidence and a concrete fix the author can act on. After that, list the non-blocking items under a line like "Optional polish, not blocking:". End the report there.
- **For a learning-value blocker,** quote the sentences that give the solution away, name the gap between what the title promises and what the task asks, and propose a reworked plot: what the learner sees, what they must find out, and how the checks should change to accept it.
- **The verdict is `ready for publication`** only when nothing blocks. List the observations under an intro line like "A few small suggestions, none blocking:".
- Keep the bullets concrete and factual: name the task or file in backticks, state what was observed and why it matters, and point at a fix direction. Skip trivia that doesn't help the author.
- Follow the general style guides: simple International English, hyphens (never em dashes), no hooks or filler.

After the report, summarize the live test results (init tasks, the initial state, and both solution runs).
