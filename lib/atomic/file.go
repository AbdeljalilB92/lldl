// Package atomic provides atomic file write operations using a write-to-temp-then-rename pattern.
// This ensures that file writes are atomic: either the new content is fully written,
// or the original file remains intact.
package atomic

import (
	"os"
	"path/filepath"
)

// WriteFile writes data to path atomically by first writing to a temporary file
// and then renaming it to the final destination. If the write or rename fails,
// the temporary file is cleaned up.
//
// A unique temp file is created via os.CreateTemp to avoid collisions when
// multiple processes write to the same path concurrently.
// The rename provides atomicity on the same filesystem. Sync is called before
// rename to ensure data is flushed to stable storage.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// CreateTemp generates a unique filename in the target directory,
	// preventing concurrent writers from clobbering each other's temp files.
	f, err := os.CreateTemp(dir, ".lldl-tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()

	// Set the requested permissions (CreateTemp uses 0600 by default).
	if err := f.Chmod(perm); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}

	if _, err = f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}

	// Flush to stable storage before rename so the new file is never empty
	if err = f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}

	if err = f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	if err = os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}

	return nil
}
