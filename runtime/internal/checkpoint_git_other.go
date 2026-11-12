//go:build !windows

package internal

func checkpointGitReparse(string) bool { return false }
