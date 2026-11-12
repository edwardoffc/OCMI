//go:build !windows

package internal

// Other platforms do not infer process quiescence from an executable open.
func probeInstalledImage(path string) error { return nil }
