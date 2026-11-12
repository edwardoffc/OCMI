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
	env := utf16.Encode([]rune(strings.Join(boundedEnv(), "\x00") + "\x00\x00"))
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{})), Flags: windows.STARTF_USESTDHANDLES, StdOutput: outputHandle, StdErr: outputHandle}
	var pi windows.ProcessInformation
	if err = windows.CreateProcess(app, line, nil, nil, true, windows.CREATE_SUSPENDED|windows.CREATE_NO_WINDOW|windows.CREATE_UNICODE_ENVIRONMENT, &env[0], cwd, &startup, &pi); err != nil {
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
			return commandResult{}, err
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
			_ = windows.TerminateJobObject(job, 1)
			windows.WaitForSingleObject(pi.Process, 5000)
			var code uint32
			_ = windows.GetExitCodeProcess(pi.Process, &code)
			pumpErr := waitPump(pumped, 2*time.Second)
			if pumpErr != nil {
				return commandResult{}, pumpErr
			}
			return commandResult{ExitCode: int(code), Output: buffer.String(), Truncated: buffer.truncated, TimedOut: true}, err
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
			var accounting jobAccounting
			for deadline := time.Now().Add(time.Second); ; {
				if err = windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil); err != nil {
					return commandResult{}, err
				}
				if accounting.ActiveProcesses == 0 || time.Now().After(deadline) {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if accounting.ActiveProcesses > 0 {
				_ = windows.TerminateJobObject(job, 1)
			}
			if pumpErr := waitPump(pumped, 2*time.Second); pumpErr != nil {
				return commandResult{ExitCode: int(code), Truncated: true, Pending: true}, nil
			}
			result := commandResult{ExitCode: int(code), Output: buffer.String(), Truncated: buffer.truncated, Pending: accounting.ActiveProcesses > 0}
			return result, nil
		}
		if status != uint32(windows.WAIT_TIMEOUT) {
			return commandResult{}, fmt.Errorf("unexpected process wait result %d", status)
		}
		time.Sleep(time.Millisecond)
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
