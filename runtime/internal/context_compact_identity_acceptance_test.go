package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainCompactCrossViewIdentity(t *testing.T) {
	root, rev := compactFixture(t)
	start := request(root, "context.inspect", "main", rev, workflowStartPayload())
	ref, _ := start.Data["bindingRef"].(string)
	if !start.OK || ref == "" {
		t.Fatal(start)
	}
	path := filepath.Join(root, "input.txt")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(path, path+".previous"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	p := compactPayload()
	p["expectedBinding"] = ref
	if r := request(root, "context.inspect", "worker", rev, p); r.OK || r.Data["context"] != nil {
		t.Fatal("same-byte bound replacement reused delivered START", r)
	}
}

func TestMainCompactReceiptIdentity(t *testing.T) {
	root, rev := compactFixture(t)
	first := request(root, "context.inspect", "worker", rev, compactPayload())
	receipt, _ := first.Data["receipt"].(string)
	if !first.OK || receipt == "" {
		t.Fatal(first)
	}
	p := compactPayload()
	p["receipt"] = receipt
	if r := request(root, "context.inspect", "main", rev, p); r.OK {
		t.Fatal("another actor reused context", r)
	}
	other, otherRev := compactFixture(t)
	if r := request(other, "context.inspect", "worker", otherRev, p); r.OK {
		t.Fatal("another copy reused context", r)
	}
	path := filepath.Join(root, "rules.md")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(path, path+".previous"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	r := request(root, "context.inspect", "worker", rev, p)
	if !r.OK || !strings.Contains(r.Data["context"].(string), "RULE-COMPACT-CANARY") || r.Data["receipt"] == receipt {
		t.Fatal("same-byte rule replacement reused receipt", r)
	}
}
