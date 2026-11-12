package internal

import "testing"

func TestEnvironmentParseWorker(t *testing.T) {
	cases := []struct {
		k, s string
		ok   bool
	}{{"go-mod", "module x\ngo 1.20\n", true}, {"node-package", `{"engines":{"node":">=20 <23"},"packageManager":"npm@10"}`, true}, {"python-project", "[project]\nrequires-python=\">=3.10,<4\"\ndescription=\"\"\"[project]\nrequires-python=\"<1\"\"\"", true}}
	for _, c := range cases {
		d, e := environmentParse(c.k, []byte(c.s))
		if (e == nil) != c.ok || len(d) == 0 {
			t.Fatal(c.k)
		}
	}
}
func TestEnvironmentConstraintWorker(t *testing.T) {
	if !environmentMatches("1.20.1", ">=1.20 <2") || environmentMatches("1.20", ">=23 <20") || !environmentConflict([]string{">=23 <20"}) || environmentConstraint(">=999999999999999999999") {
		t.Fatal("constraint")
	}
	if _, ok := environmentParseObserved("go", "go version go1.26.0 windows/amd64"); !ok {
		t.Fatal("go")
	}
	if _, ok := environmentParseObserved("go", "error 1.26"); ok {
		t.Fatal("shape")
	}
}
func TestEnvironmentParserWorkerEdges(t *testing.T) {
	if environmentConstraint(",") || !environmentConflict([]string{"20", "21"}) || environmentConflict([]string{">=20 <23", "20"}) {
		t.Fatal("interval")
	}
	d, _ := environmentParse("go-mod", []byte("/*\ngo 99\n*/\ngo 1.20 // ok\n"))
	if d[0].Declared != ">=1.20" {
		t.Fatal("comments")
	}
	if _, ok := environmentParseObserved("go", "go version go1.26.0 windows/amd64\nFAIL"); ok {
		t.Fatal("suffix")
	}
}
