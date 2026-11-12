package internal

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestCheckpointGitBatchWorkerRoundTripBinaryFraming(t *testing.T) {
	values := [][]byte{[]byte("line\n"), []byte{0, 0xff, '\n', 0x10}, []byte{}}
	want, raw := workerGitBatchFrame(values)
	input, err := checkpointGitBatchOIDInput(want)
	if err != nil || string(input) != strings.Join(want, "\n")+"\n" {
		t.Fatalf("input framing: %q, %v", input, err)
	}
	got, err := checkpointGitParseBatch(raw, want)
	if err != nil {
		t.Fatalf("valid batch refused: %v", err)
	}
	for i, oid := range want {
		if !bytes.Equal(got[oid], values[i]) {
			t.Fatalf("payload %d changed", i)
		}
	}
}

func TestCheckpointGitBatchWorkerRejectsInvalidFraming(t *testing.T) {
	values := [][]byte{[]byte("one"), []byte("two")}
	want, raw := workerGitBatchFrame(values)
	tests := []struct {
		name string
		raw  []byte
		want []string
	}{
		{"wrong order", raw, []string{want[1], want[0]}},
		{"wrong type", bytes.Replace(raw, []byte(" blob "), []byte(" tree "), 1), want},
		{"unknown oid", bytes.Replace(raw, []byte(want[0]), []byte(strings.Repeat("a", 40)), 1), want},
		{"oversize header", bytes.Replace(raw, []byte(" blob 3\n"), []byte(" blob 33554433\n"), 1), want},
		{"header overflow", []byte(want[0] + " blob " + strings.Repeat("1", 65) + "\n"), want},
		{"trailing bytes", append(append([]byte{}, raw...), 'x'), want},
		{"truncated payload", raw[:len(raw)-1], want},
		{"duplicate want", raw, []string{want[0], want[0]}},
		{"noncanonical want", raw, []string{strings.ToUpper(want[0]), want[1]}},
	}
	if _, err := checkpointGitBatchOIDInput([]string{want[0], want[0]}); err == nil {
		t.Fatal("duplicate batch request accepted")
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := checkpointGitParseBatch(tc.raw, tc.want); err == nil {
				t.Fatal("invalid batch accepted")
			}
		})
	}

	badHash := append([]byte{}, raw...)
	payloadOffset := bytes.IndexByte(badHash, '\n') + 1
	badHash[payloadOffset] ^= 1
	if _, err := checkpointGitParseBatch(badHash, want); err == nil {
		t.Fatal("wrong payload hash accepted")
	}
}

func workerGitBatchFrame(values [][]byte) ([]string, []byte) {
	want := make([]string, 0, len(values))
	var raw bytes.Buffer
	for _, value := range values {
		oid := checkpointGitOID(value)
		want = append(want, oid)
		fmt.Fprintf(&raw, "%s blob %d\n", oid, len(value))
		raw.Write(value)
		raw.WriteByte('\n')
	}
	return want, raw.Bytes()
}
