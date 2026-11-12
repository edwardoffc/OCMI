package internal

// Independent regression for the qualified native Windows installed-tool junction failure.
import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEnvironmentAcceptanceInstalledJunctionBinding(t *testing.T) {
	tool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "installed-tool-alias")
	environmentAcceptanceLink(t, alias, filepath.Dir(tool))
	selected := filepath.Join(alias, filepath.Base(tool))
	resolved, id, digest, err := environmentExecutable(project, selected)
	if err != nil {
		t.Fatalf("ordinary installed junction binding: %v", err)
	}
	if resolved == "" || id == "" || digest == "" || environmentPlainAncestors(resolved) != nil {
		t.Fatalf("binding retained a linked/unknown canonical path: %q %q %q", resolved, id, digest)
	}
	f, err := os.Open(tool)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	wantID, err := localNativeIDFile(f)
	if err != nil || id != wantID {
		t.Fatalf("bound a different installed file: %q %q %v", id, wantID, err)
	}
	if !environmentExecutableUnchanged(project, resolved, id, digest) {
		t.Fatal("stable installed tool changed")
	}
}

func TestEnvironmentAcceptanceJunctionCannotAdmitProjectOrPrivateTool(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	private := filepath.Join(base, ".local", "coordinator")
	for _, target := range []string{project, private} {
		if err := os.MkdirAll(target, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "tool.exe"), []byte("fictional nonexecuted tool"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for i, target := range []string{project, private} {
		alias := filepath.Join(base, []string{"project-alias", "private-alias"}[i])
		environmentAcceptanceLink(t, alias, target)
		if _, _, _, err := environmentExecutable(project, filepath.Join(alias, "tool.exe")); err == nil {
			t.Fatalf("junction admitted forbidden target: %s", alias)
		}
	}
}
