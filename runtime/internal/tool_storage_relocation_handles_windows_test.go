package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Holding A suspended keeps its output handle open in the parent while B is
// created. B must never inherit that unrelated handle and delay A's EOF/closure.
func TestMainToolAuthoringRelocationConcurrentHandleIsolation(t *testing.T) {
	root := t.TempDir()
	aCtx, cancelA := context.WithTimeout(context.Background(), 6*time.Second)
	bCtx, cancelB := context.WithTimeout(context.Background(), 6*time.Second)
	aReady, bReady, releaseA := make(chan struct{}), make(chan struct{}), make(chan struct{})
	aResult, bResult := make(chan commandResult, 1), make(chan commandResult, 1)
	aDone, bDone := false, false
	defer func() {
		cancelA()
		cancelB()
		if !aDone {
			select {
			case <-aResult:
			case <-time.After(8 * time.Second):
				t.Error("A fixture did not stop")
			}
		}
		if !bDone {
			select {
			case <-bResult:
			case <-time.After(8 * time.Second):
				t.Error("B fixture did not stop")
			}
		}
	}()
	go func() {
		r, err := runOwned(aCtx, commandSpec{Executable: os.Args[0], Args: []string{"--ocmi-cli-test-helper", root, "0"}, Cwd: root, OutputLimit: 1024,
			Registered: func(int, int) error {
				close(aReady)
				select {
				case <-releaseA:
					return nil
				case <-aCtx.Done():
					return aCtx.Err()
				}
			}})
		if err != nil {
			r.ErrorClass = "native_fixture_error"
		}
		aResult <- r
	}()
	select {
	case <-aReady:
	case r := <-aResult:
		aDone = true
		t.Fatal("A native setup failed", r)
	case <-time.After(3 * time.Second):
		t.Fatal("A registration barrier unavailable")
	}
	go func() {
		r, err := runOwned(bCtx, commandSpec{Executable: os.Args[0], Args: []string{"--runner-control", filepath.Join(root, "other-project-heartbeat")}, Cwd: root, OutputLimit: 1024, Registered: func(int, int) error { close(bReady); return nil }})
		if err != nil {
			r.ErrorClass = "native_fixture_error"
		}
		bResult <- r
	}()
	select {
	case <-bReady:
	case r := <-bResult:
		bDone = true
		t.Fatal("B native setup failed", r)
	case <-time.After(3 * time.Second):
		t.Fatal("B registration barrier unavailable")
	}
	close(releaseA)
	select {
	case r := <-aResult:
		aDone = true
		if r.ExitCode != 0 || !r.StoppedVerified || r.Pending || r.Truncated || r.Unresolved || r.ErrorClass != "" {
			t.Errorf("unrelated B inherited A's private completion handle: exit=%d stopped=%t pending=%t truncated=%t unresolved=%t class=%s", r.ExitCode, r.StoppedVerified, r.Pending, r.Truncated, r.Unresolved, r.ErrorClass)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("A completion waited for unrelated B")
	}
	select {
	case <-bResult:
		bDone = true
		t.Error("independent live B ended before its declared lifetime")
	default:
	}
	select {
	case r := <-bResult:
		bDone = true
		if r.ExitCode != 0 || !r.StoppedVerified || r.Pending || r.Truncated || r.Unresolved || r.ErrorClass != "" {
			t.Error("B native completion failed", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("B native completion unavailable")
	}
}

// Actual native prerequisite: extended startup plus a closed handle list starts
// a controlled suspended child, attaches its owned kill-on-close job and closes.
func TestMainToolAuthoringRelocationHandleListPremise(t *testing.T) {
	root := t.TempDir()
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		t.Fatal(err)
	}
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	h := windows.Handle(write.Fd())
	if err := windows.SetHandleInformation(h, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
		t.Fatal(err)
	}
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		t.Fatal(err)
	}
	defer attributes.Delete()
	handles := []windows.Handle{h}
	if err := attributes.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		t.Fatal(err)
	}
	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = windows.STARTF_USESTDHANDLES
	startup.StdOutput = h
	startup.StdErr = h
	startup.ProcThreadAttributeList = attributes.List()
	app, _ := windows.UTF16PtrFromString(os.Args[0])
	line, _ := windows.UTF16PtrFromString(windows.ComposeCommandLine([]string{os.Args[0], "--ocmi-cli-test-helper", root, "0"}))
	cwd, _ := windows.UTF16PtrFromString(root)
	env := utf16.Encode([]rune(strings.Join(effectiveEnv(nil), "\x00") + "\x00\x00"))
	var pi windows.ProcessInformation
	if err := windows.CreateProcess(app, line, nil, nil, true, windows.CREATE_SUSPENDED|windows.CREATE_NO_WINDOW|windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT, &env[0], cwd, &startup.StartupInfo, &pi); err != nil {
		t.Fatal("native extended startup rejected", err)
	}
	defer windows.CloseHandle(pi.Process)
	defer windows.CloseHandle(pi.Thread)
	defer func() {
		_ = windows.TerminateProcess(pi.Process, 1)
		_, _ = windows.WaitForSingleObject(pi.Process, 5000)
	}()
	if err := windows.AssignProcessToJobObject(job, pi.Process); err != nil {
		t.Fatal(err)
	}
	if _, err := windows.ResumeThread(pi.Thread); err != nil {
		t.Fatal(err)
	}
	_ = write.Close()
	if state, err := windows.WaitForSingleObject(pi.Process, 5000); err != nil || state != windows.WAIT_OBJECT_0 {
		t.Fatal("controlled native child did not stop", state, err)
	}
	var code uint32
	if err := windows.GetExitCodeProcess(pi.Process, &code); err != nil || code != 0 || !jobStopped(job, nativeTeardownBudget) {
		t.Fatal("native child/job proof unavailable", code, err)
	}
	if _, err := os.Stat(filepath.Join(root, "effect.txt")); err != nil {
		t.Fatal("controlled native child did not execute", err)
	}
}
