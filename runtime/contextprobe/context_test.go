package contextprobe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

const flag = "--ocmi-context-probe"

func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == flag {
		value, err := nativeContext()
		if err != nil || value == "" {
			os.Exit(90)
		}
		hash := sha256.Sum256([]byte(value))
		fmt.Fprint(os.Stdout, hex.EncodeToString(hash[:]))
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestNativeContextStableAcrossFreshProcesses(t *testing.T) {
	value, err := nativeContext()
	if err != nil || value == "" {
		t.Fatalf("native context unavailable: %v", err)
	}
	expected := sha256.Sum256([]byte(value))
	want := hex.EncodeToString(expected[:])
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := exec.CommandContext(ctx, os.Args[0], flag).Output()
		cancel()
		if err != nil || len(result) != 64 || string(result) != want {
			t.Fatal("native context differs or helper unavailable; identifiers are intentionally not logged")
		}
	}
}
