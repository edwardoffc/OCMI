package internal

// Independent contract checks owned by the orchestrator; workers do not edit.
import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnvironmentAcceptanceParsing(t *testing.T) {
	for _, c := range []string{"", " ", ",", " , \t", ">=999999999999999999999", "^20 || ^22"} {
		if environmentConstraint(c) {
			t.Errorf("unsupported/empty constraint accepted: %q", c)
		}
	}
	for _, c := range []string{"1.20.0", "=1.20.0", "==1.20.0", ">=1.20 <2", ">=1.20,<2"} {
		if !environmentConstraint(c) || !environmentMatches("1.20.0", c) {
			t.Errorf("supported constraint rejected: %q", c)
		}
	}
	for _, ds := range [][]string{{">=23 <20"}, {">=23", "=20"}, {"=20", "=21"}, {"20", "21"}, {">20", "<=20"}} {
		if !environmentConflict(ds) {
			t.Errorf("conflict missed: %v", ds)
		}
	}
	for _, ds := range [][]string{{">=20", "<23"}, {"=20", ">=20", "<=20"}, {"20", "20.0"}} {
		if environmentConflict(ds) {
			t.Errorf("compatible declarations conflict: %v", ds)
		}
	}
	for _, raw := range []string{`{"name":"synthetic","scripts":{"postinstall":"inert"},"dependencies":{"example":"1"},"engines":{"node":">=20"}}`, `{"engines":{"node":">=20"},"metadata":{"nested":[{},1]}}`} {
		if _, err := environmentParse("node-package", []byte(raw)); err != nil {
			t.Errorf("ordinary package refused: %v", err)
		}
	}
	for _, raw := range []string{`{"engines":{"node":">=20","node":"<1"}}`, `{"meta":{"a":1,"a":2}}`, `{"engines":[]}`, `{"engines":null}`, `{"engines":{"node":null}}`, `{"packageManager":null}`} {
		if _, err := environmentParse("node-package", []byte(raw)); err == nil {
			t.Errorf("ambiguous/invalid known field accepted: %s", raw)
		}
	}
	for _, raw := range []string{"module fixture\n/*\ngo 1.20\n*/\n", "module fixture\ngo 1.20\ngo 1.21\n"} {
		d, err := environmentParse("go-mod", []byte(raw))
		if err == nil && (len(d) != 1 || d[0].Declared != "") {
			t.Errorf("ambiguous Go declaration trusted: %+v", d)
		}
	}
	d, err := environmentParse("go-mod", []byte("module fixture\ngo 1.20 // documented minimum\n"))
	if err != nil || len(d) != 1 || d[0].Declared != ">=1.20" {
		t.Errorf("valid Go comment: %+v %v", d, err)
	}
	for _, row := range [][3]string{{"go", "go version go1.26.2 windows/amd64", "1.26.2"}, {"node", "v22.1.0\n", "22.1.0"}, {"python", "Python 3.12.14\n", "3.12.14"}} {
		if got, ok := environmentParseObserved(row[0], row[1]); !ok || got != row[2] {
			t.Errorf("fixed shape: %v %q %v", row, got, ok)
		}
	}
	for _, row := range [][2]string{{"go", "go version go1.26.2 windows/amd64\nFAIL arbitrary"}, {"go", "error: 1.26"}, {"go", "go version go999999999999999999999.2 os/arch"}, {"unknown", "1.2"}, {"node", "v22.1.0 extra"}} {
		if _, ok := environmentParseObserved(row[0], row[1]); ok {
			t.Errorf("untrusted output shape accepted: %v", row)
		}
	}
}

func environmentAcceptanceLink(t *testing.T, link, target string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		p := exec.Command("cmd", "/c", "mklink", "/J", link, target)
		if out, err := p.CombinedOutput(); err != nil {
			t.Fatalf("owned junction premise: %s %v", out, err)
		}
	} else if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			t.Error(err)
		}
	})
}

func TestEnvironmentAcceptanceSources(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	outside := filepath.Join(base, "outside")
	for _, p := range []string{project, outside} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{filepath.Join(project, "go.mod"), filepath.Join(outside, "go.mod")} {
		if err := os.WriteFile(p, []byte("module fixture\ngo 1.20\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	set, err := environmentReadSources(project, []environmentSource{{Kind: "go-mod", Path: "go.mod"}})
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()
	if err := os.Rename(project, project+"-saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), set.Files[0].Raw, 0600); err != nil {
		t.Fatal(err)
	}
	if set.Recheck() == nil {
		t.Fatal("same-byte root replacement accepted")
	}
	link := filepath.Join(project, "linked")
	environmentAcceptanceLink(t, link, outside)
	if x, err := environmentReadSources(project, []environmentSource{{Kind: "go-mod", Path: "linked/go.mod"}}); err == nil {
		x.Close()
		t.Fatal("ancestor link accepted")
	}
	if err := environmentScratchAllowed(project, link); err == nil {
		t.Fatal("linked scratch accepted")
	}
	for name, raw := range map[string][]byte{"invalid.md": {255, 255}, "large.md": []byte(strings.Repeat("x", 256*1024+1)), "secret.pem": []byte("fictional"), ".env": []byte("fictional")} {
		if err := os.WriteFile(filepath.Join(project, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
		if x, err := environmentReadSources(project, []environmentSource{{Kind: "guide", Path: name}}); err == nil {
			x.Close()
			t.Errorf("unsafe source accepted: %s", name)
		}
	}
	tool := filepath.Join(outside, "fixture-tool")
	if err := os.WriteFile(tool, []byte("owned synthetic nonexecuted tool"), 0700); err != nil {
		t.Fatal(err)
	}
	resolved, id, digest, err := environmentExecutable(project, tool)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, []byte("changed synthetic nonexecuted tool"), 0700); err != nil {
		t.Fatal(err)
	}
	if environmentExecutableUnchanged(project, resolved, id, digest) {
		t.Fatal("tool digest drift accepted")
	}
}
