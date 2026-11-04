package internal

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// The first effect pauses after its durable intent and before any native call.
// A fresh CAS revision must not let a second request overwrite that issued intent.
func TestResourceConcurrentApplyCannotSupersedeIssuedEffect(t *testing.T) {
	root, rev := setup(t, "0", 5000, 4096)
	intent := request(root, "resource.intent", "main", rev, map[string]any{
		"resourceId": "concurrent", "provider": "local", "kind": "directory", "mode": "create",
		"locator": "concurrent", "purpose": "Fictional competing caller", "lifetime": "test",
		"disposition": "remove", "ownerRef": "main",
	})
	if !intent.OK {
		t.Fatal(intent)
	}
	entered, release, result := make(chan struct{}), make(chan struct{}), make(chan Response, 1)
	var calls atomic.Int32
	resourceEffectFault = func(stage string) error {
		if stage == "after-intent" && calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return nil
	}
	defer func() { resourceEffectFault = nil }()
	go func() {
		result <- request(root, "resource.apply", "main", intent.Revision, map[string]any{"resourceId": "concurrent"})
	}()
	finished := false
	defer func() {
		close(release)
		if !finished {
			select {
			case <-result:
			case <-time.After(5 * time.Second):
				t.Error("owned test effect did not finish after release")
			}
		}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("effect did not reach durable-intent barrier")
	}
	state := request(root, "status", "main", 0, map[string]any{})
	if !state.OK {
		t.Fatal(state)
	}
	second := request(root, "resource.apply", "main", state.Revision, map[string]any{"resourceId": "concurrent"})
	if second.OK {
		t.Errorf("second request superseded an issued effect: %+v", second)
	}
	if _, err := os.Stat(filepath.Join(root, "concurrent")); !os.IsNotExist(err) {
		t.Error("second caller performed a native effect before the original was released")
	}
	// Release through a single signal while preserving the deferred drain on failure.
	release <- struct{}{}
	select {
	case first := <-result:
		finished = true
		if !first.OK || first.Revision <= state.Revision {
			t.Errorf("original effect did not record a fresh completed revision: %+v", first)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("original effect did not finish")
	}
}
