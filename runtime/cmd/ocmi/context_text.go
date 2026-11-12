package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/edwardoffc/OCMI/runtime/internal"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// ASCII JSON escapes preserve every string value while keeping terminal control
// characters, including C1 and Unicode format controls, out of the raw stream.
func writeContextJSON(r internal.Response) {
	raw, e := json.Marshal(r)
	if e != nil {
		write(r)
		return
	}
	var b strings.Builder
	for _, ch := range string(raw) {
		if ch < 128 {
			b.WriteRune(ch)
		} else if ch <= 0xffff {
			fmt.Fprintf(&b, "\\u%04x", ch)
		} else {
			a, z := utf16.EncodeRune(ch)
			fmt.Fprintf(&b, "\\u%04x\\u%04x", a, z)
		}
	}
	_, _ = io.WriteString(os.Stdout, b.String())
}

// Admission happens before Handle, including duplicate or aliased mode fields.
func textRequestOK(action string, raw []byte) bool {
	if action != "context.inspect" || len(raw) > 8<<20 {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	tok, e := d.Token()
	if e != nil || tok != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	mode := ""
	for d.More() {
		token, e := d.Token()
		if e != nil {
			return false
		}
		key, ok := token.(string)
		if !ok {
			return false
		}
		folded := strings.ToLower(key)
		if seen[folded] {
			return false
		}
		seen[folded] = true
		var val json.RawMessage
		if d.Decode(&val) != nil {
			return false
		}
		if folded == "mode" {
			if key != "mode" || json.Unmarshal(val, &mode) != nil {
				return false
			}
		}
	}
	if _, e = d.Token(); e != nil {
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return false
	}
	return mode == "compact" || mode == "start"
}

func terminalSafe(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r != '\n' && r != '\t' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029') {
			return false
		}
	}
	return true
}

// Fall back to the complete original JSON on refusal, malformed output or
// unsafe terminal content. No source is shortened or rewritten.
func writeContextText(r internal.Response, expected string) bool {
	if !r.OK {
		return false
	}
	var b strings.Builder
	switch r.Code {
	case "CONTEXT_START":
		start, ok := r.Data["start"].(string)
		if !ok || start == "" || r.Data["inputFreshness"] != "verified" || r.Data["deliveryRequired"] != true {
			return false
		}
		b.WriteString(start)
	case "CONTEXT_COMPACT":
		context, ok := r.Data["context"].(string)
		if !ok || r.Data["complete"] != true || r.Data["inputFreshness"] != "verified" {
			return false
		}
		state, ok := r.Data["state"].(map[string]any)
		if !ok {
			return false
		}
		stopped, ok := state["stopped"].(bool)
		if !ok {
			return false
		}
		task, ok := state["taskState"].(string)
		if !ok {
			return false
		}
		next, ok := state["nextAction"].(string)
		if !ok {
			return false
		}
		ref, ok := r.Data["bindingRef"].(string)
		if !ok || ref == "" {
			return false
		}
		fmt.Fprintf(&b, "%s revision=%d verified complete=true\nSTOP=%t taskState=%s nextAction=%s\n", r.Code, r.Revision, stopped, task, next)
		if expected != "" && expected == ref {
			b.WriteString("binding matched\n")
		} else {
			fmt.Fprintf(&b, "bindingRef=%s\n", ref)
		}
		retained, ok := r.Data["retainedFiles"].([]string)
		if !ok {
			return false
		}
		fmt.Fprintf(&b, "retainedFiles=[%s]\n", strings.Join(retained, ", "))
		if token, exists := r.Data["receipt"]; exists {
			s, ok := token.(string)
			if !ok || s == "" {
				return false
			}
			fmt.Fprintf(&b, "receipt=%s\n", s)
		}
		if r.Data["receiptUnchanged"] == true {
			b.WriteString("receiptUnchanged=true\n")
		}
		b.WriteString(context)
	default:
		return false
	}
	rendered := b.String()
	if !terminalSafe(rendered) {
		return false
	}
	_, _ = io.WriteString(os.Stdout, rendered)
	return true
}
