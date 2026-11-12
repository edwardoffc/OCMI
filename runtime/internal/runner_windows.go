//go:build windows

package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runOwned creates suspended, attaches to a kill-on-close Job Object, then resumes.
// This closes the CreateProcess-to-assignment escape window for non-detaching descendants.
func runOwned(ctx context.Context, spec commandSpec) (commandResult, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return commandResult{}, err
	}
	defer windows.CloseHandle(job)
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return commandResult{}, err
	}
	readOutput, writeOutput, err := os.Pipe()
	if err != nil {
		return commandResult{}, err
	}
	defer readOutput.Close()
	defer writeOutput.Close()
	outputHandle := windows.Handle(writeOutput.Fd())
	if err = windows.SetHandleInformation(outputHandle, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
		return commandResult{}, err
	}
	var inputHandle windows.Handle
	if spec.Stdin != nil {
		info, inputErr := spec.Stdin.Stat()
		if inputErr != nil || !info.Mode().IsRegular() {
			return commandResult{StoppedVerified: true}, fmt.Errorf("owned stdin unavailable")
		}
		borrowed := windows.Handle(spec.Stdin.Fd())
		if err = windows.DuplicateHandle(windows.CurrentProcess(), borrowed, windows.CurrentProcess(), &inputHandle, 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
			return commandResult{}, err
		}
		defer windows.CloseHandle(inputHandle)
	}
	app, err := windows.UTF16PtrFromString(spec.Executable)
	if err != nil {
		return commandResult{}, err
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{spec.Executable}, spec.Args...)))
	if err != nil {
		return commandResult{}, err
	}
	cwd, err := windows.UTF16PtrFromString(spec.Cwd)
	if err != nil {
		return commandResult{}, err
	}
	env := utf16.Encode([]rune(strings.Join(effectiveEnv(spec.Env), "\x00") + "\x00\x00"))
	handles := []windows.Handle{outputHandle}
	if inputHandle != 0 {
		handles = append(handles, inputHandle)
	}
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return commandResult{}, err
	}
	defer attributes.Delete()
	if err = attributes.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		return commandResult{}, err
	}
	startup := windows.StartupInfoEx{StartupInfo: windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{})), Flags: windows.STARTF_USESTDHANDLES, StdInput: inputHandle, StdOutput: outputHandle, StdErr: outputHandle}, ProcThreadAttributeList: attributes.List()}
	var pi windows.ProcessInformation
	if err = windows.CreateProcess(app, line, nil, nil, true, windows.CREATE_SUSPENDED|windows.CREATE_NO_WINDOW|windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT, &env[0], cwd, &startup.StartupInfo, &pi); err != nil {
		return commandResult{}, err
	}
	defer windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)
	if err = windows.AssignProcessToJobObject(job, pi.Process); err != nil {
		_ = windows.TerminateProcess(pi.Process, 1)
		_, _ = windows.WaitForSingleObject(pi.Process, 5000)
		return commandResult{}, err
	}
	if spec.Registered != nil {
		if err = spec.Registered(int(pi.ProcessId), 0); err != nil {
			if stopErr := windows.TerminateJobObject(job, 1); stopErr != nil {
				return commandResult{Unresolved: true}, stopErr
			}
			if status, waitErr := windows.WaitForSingleObject(pi.Process, 5000); waitErr != nil || status != windows.WAIT_OBJECT_0 {
				return commandResult{Unresolved: true}, err
			}
			if !jobStopped(job, nativeTeardownBudget) {
				return commandResult{Unresolved: true}, err
			}
			return commandResult{StoppedVerified: true}, err
		}
	}
	if _, err = windows.ResumeThread(pi.Thread); err != nil {
		return commandResult{}, err
	}
	_ = writeOutput.Close()
	buffer := &limitedBuffer{limit: spec.OutputLimit}
	pumped := make(chan error, 1)
	go func() { _, copyErr := io.Copy(limitedWriter{buffer}, readOutput); pumped <- copyErr }()
	for {
		timeout := uint32(50)
		if err := ctx.Err(); err != nil {
			if stopErr := windows.TerminateJobObject(job, 1); stopErr != nil {
				return commandResult{Unresolved: true}, stopErr
			}
			if status, waitErr := windows.WaitForSingleObject(pi.Process, 5000); waitErr != nil || status != windows.WAIT_OBJECT_0 {
				return commandResult{TimedOut: true, Pending: true, Unresolved: true}, err
			}
			verified := jobStopped(job, nativeTeardownBudget)
			var code uint32
			_ = windows.GetExitCodeProcess(pi.Process, &code)
			pumpErr := waitPump(pumped, 2*time.Second)
			if pumpErr != nil {
				return commandResult{}, pumpErr
			}
			return commandResult{ExitCode: int(code), Output: buffer.String(), Truncated: buffer.truncated, TimedOut: true, Pending: true, StoppedVerified: verified, Unresolved: !verified}, err
		}
		status, waitErr := windows.WaitForSingleObject(pi.Process, timeout)
		if waitErr != nil {
			return commandResult{}, waitErr
		}
		if status == windows.WAIT_OBJECT_0 {
			var code uint32
			if err = windows.GetExitCodeProcess(pi.Process, &code); err != nil {
				return commandResult{}, err
			}
			activeSeen, stopped := jobGrace(job, 150*time.Millisecond)
			pumpDone := pumpFinished(pumped)
			// A root can remain momentarily in Job accounting after its handle
			// signals. Treat that lone observation as a descendant only when its
			// inherited output is still open after the native grace window.
			forced := activeSeen && !pumpDone
			if !stopped {
				forced = true
				if err = windows.TerminateJobObject(job, 1); err != nil {
					return commandResult{Unresolved: true}, err
				}
			}
			verified := jobStopped(job, nativeTeardownBudget)
			if !pumpDone {
				if pumpErr := waitPump(pumped, 2*time.Second); pumpErr != nil {
					return commandResult{ExitCode: int(code), Truncated: true, Pending: true}, nil
				}
			}
			// Completion/teardown authority is distinct from an incomplete
			// command transcript. A truncated inherited stream remains pending
			// even when Job accounting proves every owned process stopped.
			result := commandResult{ExitCode: int(code), Output: buffer.String(), Truncated: buffer.truncated, Pending: !verified || buffer.truncated || forced, StoppedVerified: verified, Unresolved: !verified}
			return result, nil
		}
		if status != uint32(windows.WAIT_TIMEOUT) {
			return commandResult{}, fmt.Errorf("unexpected process wait result %d", status)
		}
		time.Sleep(time.Millisecond)
	}
}

func pumpFinished(pumped <-chan error) bool {
	select {
	case <-pumped:
		return true
	default:
		return false
	}
}

// jobGrace distinguishes a root's transient zero accounting from the child
// it just launched.  Any observed active member requires forced cleanup and
// therefore an incomplete result even once accounting reaches zero.
func jobGrace(job windows.Handle, budget time.Duration) (activeSeen, stopped bool) {
	deadline := time.Now().Add(budget)
	for {
		var accounting jobAccounting
		if windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil) != nil {
			return activeSeen, false
		}
		if accounting.ActiveProcesses > 0 {
			activeSeen = true
		}
		if time.Now().After(deadline) {
			return activeSeen, accounting.ActiveProcesses == 0
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func jobStopped(job windows.Handle, budget time.Duration) bool {
	deadline := time.Now().Add(budget)
	for {
		var accounting jobAccounting
		if windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil) != nil {
			return false
		}
		if accounting.ActiveProcesses == 0 {
			return true
		}
		if budget == 0 || time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type limitedWriter struct{ buffer *limitedBuffer }

func (w limitedWriter) Write(p []byte) (int, error) { return w.buffer.Write(p) }
func waitPump(pumped <-chan error, limit time.Duration) error {
	select {
	case err := <-pumped:
		return err
	case <-time.After(limit):
		return fmt.Errorf("bounded output drain expired")
	}
}

type jobAccounting struct {
	TotalUserTime, TotalKernelTime, ThisPeriodTotalUserTime, ThisPeriodTotalKernelTime int64
	TotalPageFaultCount, TotalProcesses, ActiveProcesses, TotalTerminatedProcesses     uint32
}
