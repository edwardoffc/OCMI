package internal

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type environmentDeclaration struct{ Kind, Declared string }

func environmentParse(kind string, raw []byte) ([]environmentDeclaration, error) {
	switch kind {
	case "go-mod":
		text := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(raw), "")
		re := regexp.MustCompile(`(?m)^\s*go\s+([^\s]+)(?:\s*//.*)?\s*$`)
		m := re.FindAllStringSubmatch(text, -1)
		if len(m) != 1 || !validVersion(m[0][1]) {
			return []environmentDeclaration{{Kind: "go"}}, nil
		}
		return []environmentDeclaration{{Kind: "go", Declared: ">=" + m[0][1]}}, nil
	case "node-package":
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || string(fields["engines"]) == "null" || string(fields["packageManager"]) == "null" {
			return nil, strconv.ErrSyntax
		}
		var v struct {
			Engines *struct {
				Node json.RawMessage `json:"node"`
			} `json:"engines"`
			PackageManager json.RawMessage `json:"packageManager"`
		}
		if !strictEnvironmentJSON(raw, &v) {
			return nil, strconv.ErrSyntax
		}
		out := []environmentDeclaration{{Kind: "node"}}
		if v.Engines != nil && len(v.Engines.Node) > 0 {
			var s string
			if json.Unmarshal(v.Engines.Node, &s) != nil || s == "" {
				return nil, strconv.ErrSyntax
			}
			out[0].Declared = s
		}
		if len(v.PackageManager) > 0 {
			var s string
			if json.Unmarshal(v.PackageManager, &s) != nil || s == "" {
				return nil, strconv.ErrSyntax
			}
			out = append(out, environmentDeclaration{Kind: "package-manager", Declared: s})
		}
		return out, nil
	case "python-project":
		var v struct {
			Project struct {
				Requires string `toml:"requires-python"`
			} `toml:"project"`
		}
		if toml.Unmarshal(raw, &v) != nil {
			return nil, strconv.ErrSyntax
		}
		return []environmentDeclaration{{Kind: "python", Declared: v.Project.Requires}}, nil
	case "guide":
		return nil, nil
	}
	return nil, strconv.ErrSyntax
}

func strictEnvironmentJSON(raw []byte, target any) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func() bool
	walk = func() bool {
		t, e := d.Token()
		if e != nil {
			return false
		}
		x, ok := t.(json.Delim)
		if !ok {
			return true
		}
		if x == '{' {
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return false
				}
				s, ok := k.(string)
				if !ok || seen[s] {
					return false
				}
				seen[s] = true
				if !walk() {
					return false
				}
			}
			_, e = d.Token()
			return e == nil
		}
		for d.More() {
			if !walk() {
				return false
			}
		}
		_, e = d.Token()
		return e == nil
	}
	return walk() && json.Unmarshal(raw, target) == nil
}

var constraintRE = regexp.MustCompile(`^(>=|<=|>|<|==|=)([0-9]+(?:\.[0-9]+)*)$`)

func environmentConstraint(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(parts) == 0 || strings.HasPrefix(strings.TrimSpace(s), ",") || strings.HasSuffix(strings.TrimSpace(s), ",") {
		return false
	}
	for _, x := range parts {
		if validVersion(x) {
			continue
		}
		if !constraintRE.MatchString(x) {
			return false
		}
		if !validVersion(constraintRE.FindStringSubmatch(x)[2]) {
			return false
		}
	}
	return true
}
func validVersion(s string) bool {
	for _, p := range strings.Split(s, ".") {
		if p == "" {
			return false
		}
		if _, e := strconv.ParseUint(p, 10, 32); e != nil {
			return false
		}
	}
	return true
}
func environmentMatches(v, c string) bool {
	if !validVersion(v) || !environmentConstraint(c) {
		return false
	}
	for _, x := range strings.FieldsFunc(c, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		m := constraintRE.FindStringSubmatch(x)
		if len(m) == 0 {
			m = []string{"", "=", x}
		}
		n := cmpVersion(v, m[2])
		if (m[1] == ">" && n <= 0) || (m[1] == ">=" && n < 0) || (m[1] == "<" && n >= 0) || (m[1] == "<=" && n > 0) || ((m[1] == "=" || m[1] == "==") && n != 0) {
			return false
		}
	}
	return true
}
func cmpVersion(a, b string) int {
	aa, bb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(aa) || i < len(bb); i++ {
		x, y := uint64(0), uint64(0)
		if i < len(aa) {
			x, _ = strconv.ParseUint(aa[i], 10, 32)
		}
		if i < len(bb) {
			y, _ = strconv.ParseUint(bb[i], 10, 32)
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}
func environmentConflict(ds []string) bool {
	lo := "0"
	loStrict := false
	hi := ""
	hiStrict := false
	for _, d := range ds {
		if !environmentConstraint(d) {
			continue
		}
		for _, x := range strings.FieldsFunc(d, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
			m := constraintRE.FindStringSubmatch(x)
			if len(m) == 0 {
				if validVersion(x) {
					m = []string{"", "=", x}
				} else {
					continue
				}
			}
			op, v := m[1], m[2]
			if op == ">" || op == ">=" {
				c := cmpVersion(v, lo)
				if c > 0 || (c == 0 && op == ">") {
					lo = v
					loStrict = op == ">"
				}
			}
			if op == "<" || op == "<=" {
				if hi == "" || cmpVersion(v, hi) < 0 || (cmpVersion(v, hi) == 0 && op == "<") {
					hi = v
					hiStrict = op == "<"
				}
			}
			if op == "=" || op == "==" {
				if (cmpVersion(v, lo) < 0) || (hi != "" && cmpVersion(v, hi) > 0) || (cmpVersion(v, lo) == 0 && loStrict) || (hi != "" && cmpVersion(v, hi) == 0 && hiStrict) {
					return true
				}
				lo = v
				hi = v
				loStrict = false
				hiStrict = false
			}
		}
	}
	if hi == "" {
		return false
	}
	c := cmpVersion(lo, hi)
	return c > 0 || c == 0 && (loStrict || hiStrict)
}
func environmentParseObserved(kind, output string) (string, bool) {
	s := strings.TrimSpace(output)
	re := map[string]*regexp.Regexp{"go": regexp.MustCompile(`^go version go([0-9]+(?:\.[0-9]+)+)(?:\s+(?:windows|linux|darwin)/[A-Za-z0-9_]+)?$`), "node": regexp.MustCompile(`^v([0-9]+(?:\.[0-9]+)+)$`), "python": regexp.MustCompile(`^Python ([0-9]+(?:\.[0-9]+)+)$`)}
	r, ok := re[kind]
	if !ok {
		return "", false
	}
	m := r.FindStringSubmatch(s)
	if len(m) != 2 {
		return "", false
	}
	return m[1], validVersion(m[1])
}
