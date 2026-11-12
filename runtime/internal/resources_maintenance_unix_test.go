//go:build !windows

package internal

import "os"

// Extending a regular file with truncate leaves its new range as a hole.
func maintenanceMarkSparse(*os.File) error { return nil }
