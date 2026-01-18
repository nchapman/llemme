package fileutil

import "os"

// AtomicWriteFile writes data to a temp file then renames it to path.
// This ensures the file is never partially written on process crash.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
