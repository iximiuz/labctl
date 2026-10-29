---
paths:
  - "**/shell-gyms/**"
---

# Shell Gym Format Documentation

A Shell Gym is `shell-gyms/<name>/index.md` (the front page: frontmatter + markdown description) next to a `path/` folder holding the shellgym learning path. Below is a self-documenting `index.md` sample - it is the whole front-page format.

```yaml
---
kind: shell-gym

title: Meet the Linux Shell

description: |
  Your first session in a Linux terminal, drilled until it sticks.

categories:
  - linux

tagz:
  - shell
  - bash

createdAt: 2026-09-04
updatedAt: 2026-09-07  # optional

cover: __static__/cover.png  # optional; a default cover is used when omitted

# PLAYGROUND CONFIGURATION
#
# Exactly like a challenge's playground: any base playground (see
# https://labs.iximiuz.com/api/playgrounds?filter=base), optional machines,
# initTasks, extra startupFiles, and so on. The playground must have at least
# one machine - the first machine is the gym machine, and its default login
# user (usually laborant) is the observed shell user. Prefer a plain single-VM
# playground unless the path needs tools the base image lacks.
playground:
  name: ubuntu-26-04

  # The learning path travels as an ordinary startup file: `labctl content push`
  # builds __static__/path.tar.gz from the sibling path/ folder on every push
  # (never hand-make or edit the archive), and the platform extracts it into
  # `path` before the machine boots. This entry is required, and its `path`
  # must be identical to `shellgym.path` below - the platform refuses to start
  # a gym whose startup files do not extract anything there.
  startupFiles:
    - path: /opt/shellgym/path
      source: __static__/path.tar.gz
      extract: true
      owner: root

# SHELLGYM TOOL CONFIGURATION
#
# The whole section and every key in it are optional; the values below are
# the defaults.
shellgym:
  version: latest         # or a pinned release tag, e.g. v0.0.4
  path: /opt/shellgym/path  # the learning path directory in the VM
  port: 63636             # the gym web UI port (1024-65535)

# SPACED REPETITION SCHEDULE
#
# Days between sessions, 1-12 integers of 1-365 days each; the last interval
# repeats forever. Optional; the values below are the default.
repetition: [1, 3, 7, 14, 30]
---

The markdown body is the gym's front page: a short, factual description of
what skills the gym builds and why they matter. The reps themselves live in
the units under `path/`, not here.

The body supports the same expression means as tutorials (diagrams, image
boxes, callouts, ...) - see the tutorial sample for the full markdown reference:
https://raw.githubusercontent.com/iximiuz/labs-content-samples/refs/heads/main/sample-tutorial/index.md
```

The learning path under `path/` (`path.yaml`, numbered `NNN.name` module folders with `module.md`, `NNN.name` unit folders with `unit.md` - lowercase letters, digits, and dashes only) follows the shellgym tool's own format - path, modules, units, tasks, vars, checks - documented by the tool itself:

- `shellgym skills authoring` (embedded, agent-oriented; also at https://github.com/iximiuz/shellgym/blob/main/skills/authoring.md)
- https://github.com/iximiuz/shellgym/blob/main/docs/authoring-guide.md (human reference)
- https://github.com/iximiuz/shellgym/blob/main/docs/checks.md (every built-in `wait_*` check)

Do not confuse shellgym unit `tasks:` with Labs playground tasks - they are different formats checked by different engines, and the path's markdown files are never validated as Labs content.
