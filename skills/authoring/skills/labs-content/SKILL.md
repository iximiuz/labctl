---
name: labs-content
description: Creates, lists, pulls, and pushes iximiuz Labs content (challenges, tutorials, courses, skill paths, Shell Gyms) with labctl, and maps remote content names to local folders. Use to scaffold new content, sync local edits to the server, fetch remote sources, or check what already exists.
argument-hint: "[create | list | pull | push] <kind> <name>"
---

Manage iximiuz Labs content with `labctl`. If arguments were given, do what they ask: $ARGUMENTS

The content kinds are `challenge`, `tutorial`, `course`, `skill-path`, and `shell-gym`.
Run all commands below from the project root.

## Find the local folder

Every piece of content lives in a folder named after the **pluralized kind** - `challenges/`, `tutorials/`, `courses/`, `skill-paths/`, or `shell-gyms/` - with `index.md` as its main file.
Remote names sometimes carry a random hex suffix (e.g., `docker-101-container-exec-6e812f1f`), while the local folder omits it (`challenges/docker-101-container-exec/`), or the other way around.
To find the local folder for `<kind> <name>`, look for these files in order and take the first match:

1. `<kind>s/<name>/index.md`
2. `<kind>s/<name>-*/index.md` (the local folder has the suffix)
3. `<kind>s/<name-without-the-hex-suffix>/index.md` (only the remote name has it)

If nothing matches, there is no local copy yet.
List the remote content (see below) to find the exact remote name.

## Create

Each content kind has its own authoring rules (e.g., `challenge-authoring` and `challenge-format-docs` for a challenge).
They may load on their own only once a file of that kind is open, so read the rules for the kind **before** choosing the name and the title of new content.
The `labctl skills show <rule-name>` command prints any of them (`labctl skills list` lists them all).

```sh
labctl content create <kind> <name> --dir <kind>s/<name>
```

This creates the content on the server (visible only to you until you publish it) and scaffolds the local folder:

- `index.md` - the main file
- `__static__/` - static files (images, startup files, archives)
- `path/` - a minimal shellgym learning path (Shell Gyms only)

Content must exist on the server before it can be pushed.
Read the generated `index.md` before making the edits the user asked for.
Once a new challenge or Shell Gym is written and pushed, review it with the `labs-review-challenge` or `labs-review-shell-gym` skill before reporting it as done.

## List what exists

```sh
labctl content list                  # all authored content (YAML)
labctl content list --kind <kind>
```

## Pull

```sh
labctl content pull <kind> <name> --dir <folder> --force
```

Pull into the detected local folder, or into `<kind>s/<name>` if there is none.
The `--force` flag overwrites local files without asking.

## Push

```sh
labctl content push <kind> <name> --dir <folder> --force                                   # the whole folder
labctl content push <kind> <name> --dir <folder> --force --file index.md --file <file>     # only these files
labctl content push <kind> <name> --dir <folder> --force --watch                           # push on every change
```

- When you know which files changed, push only those. The `--file` paths are relative to the content folder (e.g., `index.md`, `__static__/image.png`).
- With `--force`, a whole-folder push overwrites remote files and **deletes remote files that don't exist locally** without asking. Pull first if the remote copy may have files you don't have.
- The `--watch` mode runs until interrupted, so start it in the background.
- Before every push, `labctl` builds `__static__/<folder>.tar.gz` startup file archives from their sibling `<folder>/` directories (`path/` for Shell Gyms). Never build or edit these archives by hand.
- Files matching `.labctlignore` patterns are not pushed, and neither are the `.git*` files and the AI agent folders (`.claude/`, `.agents/`, `.cursor/`, ...).
- A push doesn't change running playgrounds. Restart the challenge, tutorial, or Shell Gym to pick up the changes.
