//go:build windows

package content

import (
	"strings"
)

// archiveFileModes returns the permission bits of the archived files, keyed
// by entry name. Windows doesn't track Unix permissions, so the executable
// bit comes from the git index instead (set it with
// `git update-index --chmod=+x <file>`). Files not in the index get 0644.
func archiveFileModes(folder string, entries []archiveEntry) map[string]int64 {
	indexModes, err := gitIndexModes(folder)
	if err != nil {
		warnOnce(folder, "Couldn't read file modes from the git index; archiving all files as non-executable (0644)",
			"folder", folder, "error", err)
	}

	// Windows file systems (and git's core.ignorecase) are case-insensitive,
	// so a case-only rename leaves the index entry under the old name.
	foldedModes := make(map[string]string, len(indexModes))
	for name, mode := range indexModes {
		foldedModes[strings.ToLower(name)] = mode
	}

	modes := make(map[string]int64, len(entries))
	var untracked []string
	for _, entry := range entries {
		mode, ok := indexModes[entry.name]
		if !ok {
			mode, ok = foldedModes[strings.ToLower(entry.name)]
		}
		if !ok {
			untracked = append(untracked, entry.name)
		}

		if mode == "100755" {
			modes[entry.name] = 0755
		} else {
			modes[entry.name] = 0644
		}
	}

	if err == nil && len(untracked) > 0 {
		warnOnce(folder+"\x00"+strings.Join(untracked, "\x00"),
			"Files not in the git index are archived as non-executable (0644); "+
				"`git add` them (and `git update-index --chmod=+x` scripts) to set their mode; "+
				"in watch mode, save any file afterwards to rebuild the archive",
			"folder", folder, "files", untracked)
	}

	return modes
}
