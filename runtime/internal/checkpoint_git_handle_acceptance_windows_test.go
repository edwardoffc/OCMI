//go:build windows

package internal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestCheckpointGitBorrowedInputHandleFlagsRemainUnchanged(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input")
	if err := os.WriteFile(path, []byte("native handle control"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	getFlags := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetHandleInformation")
	flags := func() uint32 {
		var value uint32
		ok, _, err := getFlags.Call(f.Fd(), uintptr(unsafe.Pointer(&value)))
		if ok == 0 {
			t.Fatal(err)
		}
		return value
	}
	before := flags()
	if before&windows.HANDLE_FLAG_INHERIT != 0 {
		t.Fatal("native control must begin with a non-inheritable borrowed input")
	}
	tool, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := runOwned(ctx, commandSpec{Executable: tool, Args: []string{"hash-object", "--stdin"},
		Cwd: root, Stdin: f, OutputLimit: 4096, Env: []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}})
	if err != nil || result.ExitCode != 0 || !result.StoppedVerified {
		t.Fatalf("native owned-input control failed: %+v %v", result, err)
	}
	if after := flags(); after != before {
		t.Fatalf("borrowed input inheritance changed: before=%x after=%x", before, after)
	}
}
