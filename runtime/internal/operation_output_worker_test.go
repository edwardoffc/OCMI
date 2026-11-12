package internal

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkerOutputAliasesAndPayloadCap(t *testing.T) {
	root, run := outputFixture(t, "shareable", "large", 2048)
	op := run.Data["operationId"]
	id, _ := json.Marshal(op)
	for _, payload := range []string{
		`{"operationId":` + string(id) + `,"OperationId":` + string(id) + `}`,
		`{"OperationId":` + string(id) + `}`,
		`{"operationId":` + string(id) + `,"limitBytes":8,"LimitBytes":16}`,
		`{"operationId":` + string(id) + `,"LimitBytes":16}`,
		`{"operationId":` + string(id) + `,"limitBytes":8}` + strings.Repeat(" ", 4096),
	} {
		r := operationOutput(Request{ProjectRoot: root, Actor: "main", Payload: []byte(payload)})
		if r.OK || r.Code != "INVALID_PAYLOAD" || r.Data["output"] != nil {
			t.Fatal(r)
		}
	}
	db, e := openExisting(root)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, command := range []string{
		`{"outputPolicy":"private","OutputPolicy":"shareable","outputLimitBytes":2048}`,
		`{"OutputPolicy":"shareable","outputLimitBytes":2048}`,
		`{"outputPolicy":"shareable","outputLimitBytes":2048,"OutputLimitBytes":4096}`,
		`{"outputPolicy":"shareable","OutputLimitBytes":2048}`,
	} {
		if _, e = db.Exec("UPDATE task_revision SET command_json=?", command); e != nil {
			t.Fatal(e)
		}
		r := request(root, "operation.output", "main", 0, map[string]any{"operationId": op})
		if r.OK || r.Code != "OUTPUT_WITHHELD" || r.Data["output"] != nil {
			t.Fatal(r)
		}
	}
}

func TestWorkerOutputScopeAndBounds(t *testing.T) {
	root, run := outputFixture(t, "shareable", "large", 2048)
	op := run.Data["operationId"]
	db, e := openExisting(root)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, payload := range []string{`{}`, `{"operationId":null}`, `{"operationId":"x","limitBytes":-1}`, `[]`} {
		r := Handle(Request{SchemaVersion: 1, ProjectRoot: root, Actor: "main", RequestID: "read", Action: "operation.output", Payload: []byte(payload)})
		if r.OK {
			t.Fatal(r)
		}
	}
	// Neither a mutable current command nor another outcome can promote evidence.
	if _, e = db.Exec("UPDATE task_revision SET command_json=replace(command_json,'shareable','private')"); e != nil {
		t.Fatal(e)
	}
	r := request(root, "operation.output", "main", 0, map[string]any{"operationId": op})
	if r.OK {
		t.Fatal("used mutable current command", r)
	}
	if _, e = db.Exec("UPDATE task_revision SET command_json=replace(command_json,'private','shareable')"); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{"é" + strings.Repeat("x", 5), string([]byte{0xff, 'x'})} {
		if _, e = db.Exec("UPDATE operation SET output=?", raw); e != nil {
			t.Fatal(e)
		}
		r = request(root, "operation.output", "main", 0, map[string]any{"operationId": op, "limitBytes": 1})
		if !r.OK || r.Data["complete"] != false || r.Data["output"] != "" {
			t.Fatal(r)
		}
	}
	if _, e = db.Exec("UPDATE operation_revision SET outcome_id='foreign'"); e != nil {
		t.Fatal(e)
	}
	if r = request(root, "operation.output", "main", 0, map[string]any{"operationId": op}); r.OK {
		t.Fatal(r)
	}
}
