package internal

import (
	"bytes"
	"errors"
)

const checkpointGitBatchHeaderMax = 64

// checkpointGitBatchOIDInput frames the exact request stream accepted by
// git cat-file --batch. The caller's order is preserved deliberately: output
// framing is bound to that order and duplicate requests are refused.
func checkpointGitBatchOIDInput(want []string) ([]byte, error) {
	if !checkpointGitBatchWantValid(want) {
		return nil, errors.New("batch oids")
	}
	out := make([]byte, 0, len(want)*41)
	for _, oid := range want {
		out = append(out, oid...)
		out = append(out, '\n')
	}
	return out, nil
}

// checkpointGitParseBatch accepts only exact cat-file --batch blob framing:
// "<oid> blob <size>\\n<raw bytes>\\n" once for each requested OID.
func checkpointGitParseBatch(raw []byte, want []string) (map[string][]byte, error) {
	if !checkpointGitBatchWantValid(want) || int64(len(raw)) > gitMaxTotal+int64(len(want))*checkpointGitBatchHeaderMax {
		return nil, errors.New("batch framing")
	}
	got := make(map[string][]byte, len(want))
	var total int64
	pos := 0
	for _, oid := range want {
		if pos >= len(raw) {
			return nil, errors.New("batch missing")
		}
		lineEnd := bytes.IndexByte(raw[pos:], '\n')
		if lineEnd < 0 || lineEnd > checkpointGitBatchHeaderMax {
			return nil, errors.New("batch header")
		}
		lineEnd += pos
		prefix := oid + " blob "
		header := raw[pos:lineEnd]
		if !bytes.HasPrefix(header, []byte(prefix)) {
			return nil, errors.New("batch header")
		}
		size, ok := checkpointGitBatchSize(header[len(prefix):])
		if !ok || size > gitMaxBlob || total > gitMaxTotal-size {
			return nil, errors.New("batch size")
		}
		pos = lineEnd + 1
		end := pos + int(size)
		if end < pos || end >= len(raw) || raw[end] != '\n' {
			return nil, errors.New("batch payload")
		}
		payload := raw[pos:end]
		if checkpointGitOID(payload) != oid {
			return nil, errors.New("batch oid")
		}
		got[oid] = payload
		total += size
		pos = end + 1
	}
	if pos != len(raw) {
		return nil, errors.New("batch trailing")
	}
	return got, nil
}

func checkpointGitBatchWantValid(want []string) bool {
	if len(want) > gitMaxFiles {
		return false
	}
	seen := make(map[string]bool, len(want))
	for _, oid := range want {
		if !checkpointGitOIDCanonical(oid) || seen[oid] {
			return false
		}
		seen[oid] = true
	}
	return true
}

func checkpointGitBatchSize(raw []byte) (int64, bool) {
	if len(raw) == 0 || (len(raw) > 1 && raw[0] == '0') {
		return 0, false
	}
	var size int64
	for _, b := range raw {
		if b < '0' || b > '9' || size > (gitMaxBlob-int64(b-'0'))/10 {
			return 0, false
		}
		size = size*10 + int64(b-'0')
	}
	return size, true
}
