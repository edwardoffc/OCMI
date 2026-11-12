package internal

import (
	"encoding/json"
	"testing"
)

func rawStopRequest(root, id string, revision int64, payload string) Request {
	return Request{SchemaVersion: 1, Action: "stop", ProjectRoot: root, Actor: "main", RequestID: id, ExpectedRevision: revision, Payload: json.RawMessage(payload)}
}

func TestStopRequiresPresentJSONBooleanAndPreservesActiveStop(t *testing.T) {
	root, revision := setup(t, "0", 5000, 4096)
	enabled := Handle(rawStopRequest(root, "stop-enable", revision, `{"enabled":true}`))
	if !enabled.OK || enabled.Code != "STOP_UPDATED" {
		t.Fatalf("enable=%+v", enabled)
	}
	if replay := Handle(rawStopRequest(root, "stop-enable", revision, `{"enabled":true}`)); !replay.OK || replay.Revision != enabled.Revision {
		t.Fatalf("idempotent enable replay=%+v", replay)
	}
	for index, payload := range []string{`{}`, `{"enabled":null}`, `{"enabled":"false"}`, `{"enabled":0}`} {
		response := Handle(rawStopRequest(root, "invalid-stop-"+string(rune('a'+index)), enabled.Revision, payload))
		if response.OK || response.Code != "INVALID_STOP" || response.Revision != enabled.Revision {
			t.Fatalf("payload %s response=%+v", payload, response)
		}
		status := request(root, "status", "main", enabled.Revision, map[string]any{})
		if !status.OK || status.Revision != enabled.Revision || status.Data["stopped"] != true {
			t.Fatalf("payload %s changed active STOP: %+v", payload, status)
		}
	}
	cleared := Handle(rawStopRequest(root, "stop-clear", enabled.Revision, `{"enabled":false}`))
	if !cleared.OK || cleared.Code != "STOP_UPDATED" || cleared.Revision != enabled.Revision+1 {
		t.Fatalf("clear=%+v", cleared)
	}
	status := request(root, "status", "main", cleared.Revision, map[string]any{})
	if !status.OK || status.Data["stopped"] != false {
		t.Fatalf("false did not clear explicit STOP: %+v", status)
	}
}
