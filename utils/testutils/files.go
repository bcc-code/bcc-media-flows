package testutils

import (
	"os"
)

// CopyFile is a dumb utility function to copy a file
// Do not use it to copy large files as it may load the whole thing into ram
func CopyFile(src string, dst string) error {
	// Read all content of src to data, may cause OOM for a large file.
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Write data to dst
	return os.WriteFile(dst, data, 0644)
}
