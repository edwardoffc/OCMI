//go:build windows

package spike

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsJobClosesOnSupervisorExit(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := helperCommand(ctx, "supervisor-crash", root)
	must(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	waitFile(t, filepath.Join(root, "ready"))
	data, err := os.ReadFile(filepath.Join(root, "ready"))
	must(t, err)
	pid, err := strconv.Atoi(string(data))
	must(t, err)
	// Open while the known child is alive; retained handle avoids PID-reuse inference.
	child, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	must(t, err)
	defer windows.CloseHandle(child)
	state, err := windows.WaitForSingleObject(child, 0)
	must(t, err)
	if state != uint32(windows.WAIT_TIMEOUT) {
		t.Fatal("child did not start alive")
	}
	must(t, os.WriteFile(filepath.Join(root, "release"), []byte("crash"), 0600))
	if err = cmd.Wait(); err == nil || cmd.ProcessState.ExitCode() != 86 {
		t.Fatalf("unexpected supervisor result: %v", err)
	}
	state, err = windows.WaitForSingleObject(child, 5000)
	must(t, err)
	if state != windows.WAIT_OBJECT_0 {
		t.Fatal("descendant survived supervisor crash")
	}
}

type ownedProcess struct {
	job, process windows.Handle
	closed       bool
}

// Win32 JOBOBJECT_BASIC_ACCOUNTING_INFORMATION layout (four LARGE_INTEGERs, four DWORDs).
type jobAccounting struct {
	TotalUserTime, TotalKernelTime, ThisPeriodTotalUserTime, ThisPeriodTotalKernelTime int64
	TotalPageFaultCount, TotalProcesses, ActiveProcesses, TotalTerminatedProcesses     uint32
}

func startOwned(root, mode string) (*ownedProcess, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	app, err := windows.UTF16PtrFromString(os.Args[0])
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{os.Args[0]}, helperArgs(mode, root)...)))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	cwd, err := windows.UTF16PtrFromString(root)
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	env := utf16.Encode([]rune(strings.Join(minimalEnv(), "\x00") + "\x00\x00"))
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	var pi windows.ProcessInformation
	// Suspended creation closes the exec.Start -> assign-job escape window for this fixture.
	err = windows.CreateProcess(app, line, nil, nil, false, windows.CREATE_SUSPENDED|windows.CREATE_NO_WINDOW|windows.CREATE_UNICODE_ENVIRONMENT, &env[0], cwd, &startup, &pi)
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	defer windows.CloseHandle(pi.Thread)
	if err = windows.AssignProcessToJobObject(job, pi.Process); err != nil {
		windows.TerminateProcess(pi.Process, 1)
		windows.WaitForSingleObject(pi.Process, 3000)
		windows.CloseHandle(pi.Process)
		windows.CloseHandle(job)
		return nil, err
	}
	p := &ownedProcess{job: job, process: pi.Process}
	if _, err = windows.ResumeThread(pi.Thread); err != nil {
		p.close()
		return nil, err
	}
	return p, nil
}

func (p *ownedProcess) close() error {
	if p.closed {
		return nil
	}
	if err := windows.TerminateJobObject(p.job, 1); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		var info jobAccounting
		err := windows.QueryInformationJobObject(p.job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil)
		if err != nil {
			return err
		}
		if info.ActiveProcesses == 0 {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("owned job still has %d active processes", info.ActiveProcesses)
		}
		time.Sleep(10 * time.Millisecond)
	}
	result, err := windows.WaitForSingleObject(p.process, 3000)
	if err != nil {
		return err
	}
	if result != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("owned process wait incomplete: %d", result)
	}
	err = windows.CloseHandle(p.process)
	err2 := windows.CloseHandle(p.job)
	p.closed = true
	if err != nil {
		return err
	}
	return err2
}
