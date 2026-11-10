//go:build !windows

package content

// archiveFileModes returns the permission bits of the archived files, keyed
// by entry name. On Unix-like systems, the local file mode is the source of
// truth.
func archiveFileModes(_ string, entries []archiveEntry) map[string]int64 {
	modes := make(map[string]int64, len(entries))
	for _, entry := range entries {
		modes[entry.name] = int64(entry.info.Mode().Perm())
	}
	return modes
}
