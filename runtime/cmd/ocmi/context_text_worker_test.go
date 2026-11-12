package main

import (
	"encoding/json"
	"github.com/edwardoffc/OCMI/runtime/internal"
	"io"
	"os"
	"strings"
	"testing"
)

func TestWorkerContextTextAdmission(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		action, payload string
		ok              bool
	}{
		{"context.inspect", `{"mode":"start","taskId":"t","taskRevision":1,"worker":"w"}`, true},
		{"context.inspect", `{"mode":"compact","files":["--format","text"]}`, true},
		{"run", `{"mode":"compact"}`, false},
		{"context.inspect", `{"mode":"summary"}`, false},
		{"context.inspect", `{"mode":"compact","Mode":"start"}`, false},
		{"context.inspect", `{"mode":"compact","mode":"start"}`, false},
	} {
		if got := textRequestOK(tc.action, []byte(tc.payload)); got != tc.ok {
			t.Fatal(tc, got)
		}
	}
	large := `{"mode":"compact","receipt":"` + strings.Repeat("a", 17000) + `"}`
	if !textRequestOK("context.inspect", []byte(large)) {
		t.Fatal("transport imposed a smaller request limit")
	}
	for _, args := range [][]string{
		{"--root", root, "--actor", "main", "--action", "init", "--revision", "0", "--format", "text", "--payload", `{"mode":"compact"}`},
		{"--root", root, "--actor", "main", "--action", "context.inspect", "--format", "json"},
		{"--root", root, "--actor", "main", "--action", "context.inspect", "--format", "text", "--format", "text"},
	} {
		r, _, _ := callFormatted(args)
		if r.OK || r.Code != "USAGE" {
			t.Fatal(r)
		}
	}
	entries, e := os.ReadDir(root)
	if e != nil || len(entries) != 0 {
		t.Fatal("invalid format created state", entries, e)
	}
}

func TestWorkerContextTextRendering(t *testing.T) {
	render := func(r internal.Response, expected string) (string, bool) {
		old := os.Stdout
		rd, wr, e := os.Pipe()
		if e != nil {
			t.Fatal(e)
		}
		os.Stdout = wr
		ok := writeContextText(r, expected)
		wr.Close()
		os.Stdout = old
		out, e := io.ReadAll(rd)
		rd.Close()
		if e != nil {
			t.Fatal(e)
		}
		return string(out), ok
	}
	r := internal.Response{OK: true, Code: "CONTEXT_COMPACT", Revision: 7, Data: map[string]any{"inputFreshness": "verified", "complete": true, "state": map[string]any{"stopped": true, "taskState": "ready", "nextAction": "stop"}, "bindingRef": "abc", "retainedFiles": []string{"rules.md"}, "receipt": "new-token", "context": "complete source\n"}}
	out, ok := render(r, "abc")
	if !ok || !strings.Contains(out, "STOP=true") || !strings.Contains(out, "binding matched") || !strings.Contains(out, "receipt=new-token") || !strings.HasSuffix(out, "complete source\n") {
		t.Fatal(out, ok)
	}
	r.Data["context"] = "package p\n\tconst x = 1\n"
	if out, ok = render(r, ""); !ok || !strings.HasSuffix(out, "package p\n\tconst x = 1\n") {
		t.Fatal("TAB must survive", out, ok)
	}
	for _, unsafe := range []string{"unsafe\x1b[31m", "unsafe\u009b", "unsafe\u202e"} {
		r.Data["context"] = unsafe
		if out, ok = render(r, ""); ok || out != "" {
			t.Fatal(out, ok)
		}
	}
	r.OK = false
	if out, ok = render(r, ""); ok || out != "" {
		t.Fatal(out, ok)
	}
	s := internal.Response{OK: true, Code: "CONTEXT_START", Revision: 3, Data: map[string]any{"start": "exact START", "inputFreshness": "verified", "deliveryRequired": true}}
	if out, ok = render(s, ""); !ok || out != "exact START" {
		t.Fatal(out, ok)
	}
}

func TestWorkerContextTextJSONFallback(t *testing.T) {
	want := "retained \u009b\u202e\U0001f600"
	r := internal.Response{OK: false, Code: "REFUSED", Data: map[string]any{"reason": want}}
	old := os.Stdout
	rd, wr, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	os.Stdout = wr
	writeContextJSON(r)
	wr.Close()
	os.Stdout = old
	raw, e := io.ReadAll(rd)
	rd.Close()
	if e != nil {
		t.Fatal(e)
	}
	if !terminalSafe(string(raw)) {
		t.Fatal("unsafe fallback", string(raw))
	}
	var actual internal.Response
	if e = json.Unmarshal(raw, &actual); e != nil || actual.Data["reason"] != want {
		t.Fatal(string(raw), e)
	}
}
