package internal

import (
	"strings"
	"testing"
)

func TestContextWorkerR2BoundsAssignmentAndCacheIdentity(t *testing.T) {
	tooLong := task{Worker: string(make([]byte, 257)), Reviewer: "main", Contract: "contract.md", Inputs: `[]`, BindingFingerprint: `["contract:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]`}
	if _, ok := contextAssignment(tooLong); ok {
		t.Fatal("unbounded assignment metadata accepted")
	}
	contextCacheDrop()
	f := contextFile{path: "src/main.go", sha: "same-content", native: "native-a", raw: []byte("func compute() {}\n")}
	first := contextSnapshot{rootID: "root-a", projectID: "project", deviceID: "device-a", copyID: "copy", task: task{ID: "task", Revision: 1}}
	_, reused, _, _ := contextCacheFor("root", first, []contextFile{f})
	if reused != 0 {
		t.Fatal("cold cache reused")
	}
	changed := first
	changed.deviceID = "device-b"
	_, reused, _, _ = contextCacheFor("root", changed, []contextFile{f})
	if reused != 0 {
		t.Fatal("cache crossed device identity")
	}
	changed = first
	f.native = "native-b"
	_, reused, _, _ = contextCacheFor("root", changed, []contextFile{f})
	if reused != 0 {
		t.Fatal("cache crossed file identity")
	}
}

func TestContextWorkerR6DenseWindowBuffer(t *testing.T) {
	f := contextFile{path: "src/dense.go", sha: "x", raw: []byte("package fixture\n" + strings.Repeat("func caller() { compute() }\n", contextWindowLimit+80) + "func compute() {}\n")}
	windows, limited := contextWindows(f, []string{"compute"})
	if !limited || len(windows) > contextWindowLimit {
		t.Fatalf("candidate buffer exceeded or hid truncation: limited=%v windows=%d", limited, len(windows))
	}
	if !strings.Contains(windows[0]["text"].(string), "func compute") {
		t.Fatal("definition was displaced by dense references")
	}
}

func TestContextWorkerR2LiteralWindows(t *testing.T) {
	f := contextFile{path: "src/main.go", sha: "x", raw: []byte("func precompute() {}\nfunc compute() {}\nfunc caller() { compute() }\n")}
	windows, limited := contextWindows(f, []string{"compute"})
	if limited {
		t.Fatal("small literal fixture was unexpectedly limited")
	}
	if len(windows) != 2 || windows[0]["kind"] != "definition-candidate" || windows[1]["kind"] != "reference-candidate" {
		t.Fatalf("literal definition/reference windows=%v", windows)
	}
}
