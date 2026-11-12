package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestWorkerCallIDLosslessNormalization(t *testing.T) {
	key := func(root, kind, actor, action string, revision int64, payload string) string {
		t.Helper()
		id, err := mutatingCallID(root, kind, actor, action, revision, []byte(payload))
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^call-v1-[a-f0-9]{64}$`).MatchString(id) {
			t.Fatal("unbounded or nonopaque key", id)
		}
		return id
	}
	root := "fictional-selected-root"
	base := key(root, "project", "worker", "resource.write", 7, `{"nested":{"b":[9007199254740993,9223372036854775808123456789],"a":"fictional bytes"},"flag":true}`)
	equivalent := key(root, "project", "worker", "resource.write", 7, " { \"flag\" : true, \"nested\" : {\"a\":\"fictional bytes\",\"b\":[9007199254740993,9223372036854775808123456789]} } ")
	if base != equivalent {
		t.Fatal("key order or whitespace changed request ID")
	}
	for _, payload := range []string{`{"n":9007199254740992}`, `{"n":9007199254740993}`, `{"n":9223372036854775808123456789}`, `{"n":9223372036854775808123456790}`, `{"n":1e9999}`} {
		id := key(root, "project", "worker", "resource.write", 7, payload)
		for _, other := range []string{`{"n":9007199254740992}`, `{"n":9007199254740993}`, `{"n":9223372036854775808123456789}`, `{"n":9223372036854775808123456790}`, `{"n":1e9999}`} {
			if other != payload && id == key(root, "project", "worker", "resource.write", 7, other) {
				t.Fatal("lossy JSON number collision")
			}
		}
	}
	seen := map[string]bool{}
	for _, input := range []struct {
		root, kind, actor, action string
		revision                  int64
	}{
		{root, "project", "worker", "run", 7}, {root + "-other", "project", "worker", "run", 7},
		{root, "installation", "worker", "run", 7}, {root, "", "worker", "run", 7},
		{root, "project", "main", "run", 7}, {root, "project", "worker", "submit", 7}, {root, "project", "worker", "run", 8},
	} {
		id := key(input.root, input.kind, input.actor, input.action, input.revision, `{}`)
		if seen[id] {
			t.Fatal("invocation component omitted from key")
		}
		seen[id] = true
	}
}

func TestWorkerCallIDValidatedFlagsAndReadDefaults(t *testing.T) {
	root := t.TempDir()
	first := []string{"--root", root, "--actor", "worker", "--action", "submit", "--revision", "7", "--payload", `{"b":2,"a":1}`}
	second := []string{"--payload", ` { "a":1, "b":2 } `, "--revision", "7", "--action", "submit", "--actor", "worker", "--root", root}
	a, b := call(first), call(second)
	if a.Data["requestId"] == nil || a.Data["requestId"] != b.Data["requestId"] {
		t.Fatal("validated equivalent flags changed key", a, b)
	}
	if r := call(append(append([]string{}, first...), "--request-id", "chosen-explicit-key")); r.Data["requestId"] != "chosen-explicit-key" {
		t.Fatal("explicit ID changed", r)
	}
	defaultPayload := []string{"--root", root, "--actor", "worker", "--action", "submit", "--revision", "7"}
	if call(defaultPayload).Data["requestId"] != call(append(defaultPayload, "--payload", `{}`)).Data["requestId"] {
		t.Fatal("default payload changed key")
	}
	read := []string{"--root", root, "--actor", "main", "--action", "status"}
	x, y := call(read), call(read)
	if x.Data["requestId"] == nil || x.Data["requestId"] == y.Data["requestId"] || strings.HasPrefix(x.Data["requestId"].(string), "call-v1-") {
		t.Fatal("observation auto-ID changed semantics")
	}
	for _, invalid := range [][]string{
		{"--root", root, "--actor", "worker", "--action", "submit"},
		append(append([]string{}, first...), "--revision", "8"),
		{"--root", root, "--actor", "worker", "--action", "submit", "--revision", "7", "--payload", "{"},
	} {
		if r := call(invalid); r.Code != "USAGE" || r.Data["requestId"] != nil {
			t.Fatal("invalid flags reached dispatch", r)
		}
	}
}
