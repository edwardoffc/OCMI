//go:build !windows

package internal

import "path/filepath"

func environmentCanonicalExecutable(path string) (string, error) { return filepath.EvalSymlinks(path) }
func environmentSelectedExecutablePath(string) error             { return nil }
