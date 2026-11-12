package internal

import "os"

type commandSpec struct {
	Executable, Cwd string
	Args            []string
	OutputLimit     int
}

type commandResult struct {
	ExitCode  int
	Output    string
	Truncated bool
	TimedOut  bool
	Pending   bool
}

func boundedEnv() []string {
	keys := []string{"SystemRoot", "ComSpec", "TEMP", "TMP", "TMPDIR"}
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			result = append(result, key+"="+value)
		}
	}
	return result
}
