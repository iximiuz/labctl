---
paths:
  - "**/shell-gyms/**"
---

# Shell Gym Authoring

A Shell Gym is a content kind that wraps a [shellgym](https://github.com/iximiuz/shellgym) learning path: the student gets a Playground VM with the `shellgym` trainer daemon running the author's path, in a split view (terminal on the left, the gym UI on the right), plus a spaced repetition schedule that brings the student back to the gym.

Naming in copy: the content kind is "Shell Gym" (two words); `shellgym` refers to the tool only.

## Folder layout

```
shell-gyms/<name>/
  index.md            # the front page: frontmatter + markdown description
  __static__/
    cover.png
    path.tar.gz       # built by labctl from path/ - NEVER hand-made, never edited
  path/               # the shellgym learning path
    path.yaml
    010.first-module/
      module.md
      010.some-unit/unit.md
      020.other-unit/unit.md
    020.second-module/
      ...
```

The learning path itself follows the shellgym format (`path.yaml`, numbered module folders, `unit.md` with `tasks:`, `vars:`, `init:`/`check:`/`hint:`/`solve:` scripts). Module and unit folders are `NNN.name` where `name` is lowercase letters, digits, and dashes only (`010.warm-up/020.make-a-directory`) - the platform tracks per-session progress by unit id and refuses to start a gym with other characters in folder names. The authoritative reference is the tool's embedded skill: run `shellgym skills authoring` (or read https://github.com/iximiuz/shellgym/blob/main/skills/authoring.md) before writing units. Do not confuse shellgym unit `tasks:` with Labs playground tasks - they are different formats checked by different engines.

## `index.md` frontmatter

```yaml
---
kind: shell-gym
title: Meet the Linux Shell
description: Your first session in a Linux terminal, drilled until it sticks.
createdAt: 2026-09-04
categories: [linux]
tagz: [shell, bash]
cover: __static__/cover.png

playground:                  # exactly like challenges
  name: ubuntu-26-04
  startupFiles:
    - path: /opt/shellgym/path   # where the learning path lands in the VM
      source: __static__/path.tar.gz   # labctl builds it from the sibling path/ folder
      extract: true
      owner: root

shellgym:                    # the tool section - every key optional
  version: latest            # or a pinned release tag, e.g. v0.0.4
  path: /opt/shellgym/path   # the learning path directory in the VM (must match the startup file); default
  port: 63636                # the gym web UI port; default 63636

repetition: [1, 3, 7, 14, 30]   # days between sessions (default); the last interval repeats
---

Markdown description of the gym: what it drills, who it is for, what the student
should know already. Diagrams and image boxes work like in tutorials.
```

The learning path travels as an ordinary startup file - the same folder-to-archive mechanism challenges use: `labctl content push` sees `source: __static__/path.tar.gz` next to a `path/` folder and builds the archive itself (on every push, and on every change in `--watch` mode); the platform extracts it into the entry's `path` before boot and starts the daemon on `shellgym.path` (as a systemd/OpenRC service, with `--live` for students so solution scripts are stripped). Keep the startup file's `path` and `shellgym.path` identical - the platform refuses to start a gym whose startup files do not extract anything there. The path's markdown files are pushed as regular files too, so they are searchable on the platform.

The playground must have at least one machine; the first machine is the gym machine and its default login user (usually `laborant`) is the observed shell user. Prefer a plain single-VM playground (`ubuntu-26-04`, `debian-stable`, ...) unless the path needs tools the base image lacks; extra tooling goes into `playground.initTasks`/`startupFiles` as for challenges.

## Style

- The front page description is short and factual: what skills the gym builds and why they matter. The reps themselves live in the units, not in `index.md`.
- Unit texts are one-action assignments in the imperative mood ("Create a file named ...", "Free port ..."). Keep them terse - the student reads them in a narrow pane next to a terminal.
- Design for repetition: units should be quick (seconds, not minutes), parametric where it helps (random names, tokens, ports via `vars:`) so a second session is not a copy of the first, and independent unless declared with `needs:`.
- Every unit must carry a `solve:` script so `shellgym solve` can prove the path is solvable.
- Never start a sentence with a command name, an option, or any other code token (`seq`, `--help`, `/text`, `0`). Such tokens are almost always lower-case, so a sentence that opens with one looks like a typo, and the reader stumbles at exactly the moment they should be reading fast. Give the sentence an ordinary first word instead: "The `seq` command prints one number per line.", "Run `date` plain first.", "The `--help` output can scroll past quickly.", "Inside the pager, `/text` searches forward.". This applies everywhere prose appears: unit bodies, `module.md`, tips, and task slot messages.
- Never call a unit a "rep" in student-facing text (unit bodies, `module.md`, tips, task slot messages, and hint messages). "Rep" is authoring vocabulary from the curriculum notes and the shellgym docs. The student sees units, so a hint that needs to name the current exercise says "this unit", or better, states what the exercise asks for ("Move away and come back with `cd` on its own.") without naming it at all.
- Keep task completion messages (the `#completed` slot of `::task`) to one short sentence that confirms what happened ("The prompt is back.", "That is tomorrow's date."). Never put an explainer there. Most units have a single task, and the gym auto-advances to the next unit as soon as the unit completes, so the green box is on screen for a moment only. A paragraph that appears and vanishes in that moment confuses the student and is never read. In a multi-task unit, the completion message of a non-final task stays visible until the unit completes, so it may add one short pointer to the next step ("The nap is over. Now ask for the verdict."), but the final task follows the one-sentence rule. Anything worth teaching goes where the student can read it while working: the unit body above the task box (as the expectation, e.g. "The answer is a real file on disk"), `module.md`, or a `::tip`.

## Workflow

1. `labctl content create shell-gym <name> --dir shell-gyms/<name>` scaffolds `index.md` (with the startup file entry already declared), `__static__/` (with `path.tar.gz` already built, so the gym starts right away), and a minimal `path/`.
2. Edit `index.md` and the units.
3. `labctl content push shell-gym <name> --dir shell-gyms/<name> --force` (add `--watch` to keep pushing on every change); the archive is rebuilt automatically before each push.
4. Test in a real session - see the `labs-edit-shell-gym` skill for the two ways of picking up changes in a running gym and how to run `shellgym validate` / `shellgym solve` in the VM.
5. When the gym is new or substantially changed, review it with the `labs-review-shell-gym` skill before reporting the work as done, and fix the blockers it finds. The iximiuz Labs team runs the same review on community submissions. Small edits (a typo, a reworded hint) don't need a full review.
