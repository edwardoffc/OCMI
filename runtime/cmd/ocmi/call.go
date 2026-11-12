package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/edwardoffc/OCMI/runtime/internal"
	"github.com/google/uuid"
	"path/filepath"
	"strconv"
)

func observation(action string) bool {
	switch action {
	case "status", "layout.inspect", "context.inspect", "task.history", "operation.inspect", "operation.output", "resource.inspect", "resource.list", "resource.maintain.inspect", "environment.inspect", "checkpoint.inspect", "tool-storage.inspect":
		return true
	}
	return false
}

func call(args []string) internal.Response {
	r, _, _ := callFormatted(args)
	return r
}

func callFormatted(args []string) (internal.Response, bool, string) {
	fail := func() internal.Response {
		return internal.Response{SchemaVersion: 1, Code: "USAGE", Data: map[string]any{"nextAction": "help call"}}
	}
	values := map[string]string{}
	for len(args) > 0 {
		if len(args) < 2 || args[1] == "" {
			return fail(), false, ""
		}
		k, v := args[0], args[1]
		args = args[2:]
		switch k {
		case "--root", "--actor", "--action", "--revision", "--payload", "--request-id", "--root-kind", "--format":
		default:
			return fail(), false, ""
		}
		if _, ok := values[k]; ok {
			return fail(), false, ""
		}
		values[k] = v
	}
	if !filepath.IsAbs(values["--root"]) || values["--actor"] == "" || values["--action"] == "" {
		return fail(), false, ""
	}
	kind := values["--root-kind"]
	if kind != "" && kind != "project" && kind != "installation" {
		return fail(), false, ""
	}
	rev := int64(0)
	if s, ok := values["--revision"]; ok {
		var err error
		rev, err = strconv.ParseInt(s, 10, 64)
		if err != nil || rev < 0 {
			return fail(), false, ""
		}
	} else if !observation(values["--action"]) {
		return fail(), false, ""
	}
	payload := values["--payload"]
	if payload == "" {
		payload = "{}"
	}
	if len(payload) > 8<<20 || !json.Valid([]byte(payload)) {
		return fail(), false, ""
	}
	format, specified := values["--format"]
	if specified && (format != "text" || !textRequestOK(values["--action"], []byte(payload))) {
		return fail(), false, ""
	}
	expected := ""
	if specified {
		var v struct {
			Expected string `json:"expectedBinding"`
		}
		_ = json.Unmarshal([]byte(payload), &v)
		expected = v.Expected
	}
	id := values["--request-id"]
	if id == "" {
		if observation(values["--action"]) {
			id = uuid.NewString()
		} else {
			var err error
			id, err = mutatingCallID(values["--root"], kind, values["--actor"], values["--action"], rev, []byte(payload))
			if err != nil {
				return fail(), false, ""
			}
		}
	}
	r := internal.Handle(internal.Request{SchemaVersion: 1, ProjectRoot: values["--root"], RootKind: kind, Actor: values["--actor"], Action: values["--action"], ExpectedRevision: rev, RequestID: id, Payload: json.RawMessage(payload)})
	if r.Data == nil {
		r.Data = map[string]any{}
	}
	r.Data["requestId"] = id
	return r, specified, expected
}

// The caller can reconstruct this key from the retained invocation even when
// the whole response is lost. This is independent of persisted core request
// fingerprints: numbers here must never pass through float64.
func mutatingCallID(root, kind, actor, action string, revision int64, payload []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return "", err
	}
	raw, err := json.Marshal(struct {
		Root, RootKind, Actor, Action string
		Revision                      int64
		Payload                       any
	}{root, kind, actor, action, revision, normalized})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return "call-v1-" + hex.EncodeToString(digest[:]), nil
}
