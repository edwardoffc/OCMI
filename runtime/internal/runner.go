package internal

import (
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const nativeTeardownBudget = 5 * time.Second

type commandSpec struct {
	Executable, Cwd string
	Args            []string
	OutputLimit     int
	Env             []string
	// Stdin is an already-open owned regular file. Nil keeps the existing
	// no-stdin behaviour; callers never pass a path or a pipe.
	Stdin      *os.File
	Registered func(pid, group int) error
}

type commandResult struct {
	ExitCode        int
	Output          string
	Truncated       bool
	TimedOut        bool
	Pending         bool
	Unresolved      bool
	StoppedVerified bool
	// Observation is sanitized diagnostic state only; it never carries command,
	// path, output, or native error text and does not alter lifecycle decisions.
	Phase            string
	ErrorClass       string
	GroupObservation string
	PumpObservation  string
	WaitCalled       *bool
	NativeErrno      *int
}

// outputPumpState observes the pump without receiving from its completion
// channel. The runner still owns that channel and its existing lifecycle.
type outputPumpState struct {
	done   atomic.Bool
	failed atomic.Bool
}

func (s *outputPumpState) finish(err error) {
	s.failed.Store(err != nil)
	s.done.Store(true)
}

func (s *outputPumpState) observation() string {
	if !s.done.Load() {
		return "open"
	}
	if s.failed.Load() {
		return "error"
	}
	return "eof"
}

func observedBool(value bool) *bool { return &value }

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

// effectiveEnv gives command overrides one authoritative entry per variable.
// Windows is case-insensitive; treating keys this way on Unix keeps the two
// runners' scoped environment intention identical.
func effectiveEnv(overrides []string) []string {
	values := make(map[string]string)
	order := make([]string, 0, len(boundedEnv())+len(overrides))
	add := func(entry string) {
		i := strings.IndexByte(entry, '=')
		if i <= 0 {
			return
		}
		key := entry[:i]
		canonical := strings.ToUpper(key)
		if _, exists := values[canonical]; !exists {
			order = append(order, canonical)
		}
		values[canonical] = key + "=" + entry[i+1:]
	}
	for _, entry := range boundedEnv() {
		add(entry)
	}
	for _, entry := range overrides {
		add(entry)
	}
	sort.Strings(order) // CreateProcess requires a sorted environment block.
	result := make([]string, 0, len(order))
	for _, key := range order {
		result = append(result, values[key])
	}
	return result
}
