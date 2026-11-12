package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnvironmentSourcesReadAndRecheck(t *testing.T) {
	root := t.TempDir()
	writeEnvironmentSource(t, root, "go.mod", "module fixture\n\ngo 1.26\n")
	set, err := environmentReadSources(root, []environmentSource{{Kind: "go-mod", Path: "go.mod"}})
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()
	if len(set.Files) != 1 || set.Files[0].SHA == "" || set.Files[0].NativeID == "" {
		t.Fatalf("files=%+v", set.Files)
	}
	if err := set.Recheck(); err != nil {
		t.Fatal(err)
	}
	writeEnvironmentSource(t, root, "go.mod", "module fixture\n\ngo 1.27\n")
	if err := set.Recheck(); err == nil {
		t.Fatal("source drift accepted")
	}
}

func TestEnvironmentWindowsSpecialToolPathRefusedBeforeOpen(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path guard")
	}
	if environmentSelectedExecutablePath(`\\.\NUL`) == nil {
		t.Fatal("device namespace accepted")
	}
	if environmentSelectedExecutablePath(`\\?\PIPE\fixture`) == nil {
		t.Fatal("pipe namespace accepted")
	}
}

func TestEnvironmentUnixInstalledToolSymlinkBinding(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has native pre-open guard")
	}
	tool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	link := filepath.Join(root, "installed-go")
	if err := os.Symlink(tool, link); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	if _, _, _, err := environmentExecutable(project, link); err != nil {
		t.Fatalf("installed symlink refused: %v", err)
	}
}

func TestEnvironmentSourcesRejectRootReplacementAndAncestorLink(t *testing.T) {
	base := t.TempDir()
	project, outside := filepath.Join(base, "project"), filepath.Join(base, "outside")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	writeEnvironmentSource(t, project, "go.mod", "module fixture\ngo 1.26\n")
	writeEnvironmentSource(t, outside, "go.mod", "module fixture\ngo 1.26\n")
	set, err := environmentReadSources(project, []environmentSource{{Kind: "go-mod", Path: "go.mod"}})
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()
	if err := os.Rename(project, project+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	writeEnvironmentSource(t, project, "go.mod", "module fixture\ngo 1.26\n")
	if err := set.Recheck(); err == nil {
		t.Fatal("same-byte root replacement accepted")
	}
	environmentAcceptanceLink(t, filepath.Join(project, "linked"), outside)
	if linked, err := environmentReadSources(project, []environmentSource{{Kind: "go-mod", Path: "linked/go.mod"}}); err == nil {
		linked.Close()
		t.Fatal("ancestor link accepted")
	}
}

func TestEnvironmentSourcesRejectUnsafeAndDuplicate(t *testing.T) {
	root := t.TempDir()
	writeEnvironmentSource(t, root, "go.mod", "module fixture\n")
	for _, selected := range [][]environmentSource{
		{{Kind: "go-mod", Path: ".env"}},
		{{Kind: "go-mod", Path: "go.mod"}, {Kind: "guide", Path: "go.mod"}},
		{{Kind: "unknown", Path: "go.mod"}},
	} {
		if set, err := environmentReadSources(root, selected); err == nil {
			set.Close()
			t.Fatalf("unsafe selection accepted: %+v", selected)
		}
	}
	writeEnvironmentSource(t, root, "binary.txt", string([]byte{'x', 0}))
	if set, err := environmentReadSources(root, []environmentSource{{Kind: "guide", Path: "binary.txt"}}); err == nil {
		set.Close()
		t.Fatal("binary source accepted")
	}
}

func TestEnvironmentScratchRejectsOverlap(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := environmentScratchAllowed(root, outside); err != nil {
		t.Fatal(err)
	}
	if err := environmentScratchAllowed(root, filepath.Join(root, "scratch")); err == nil {
		t.Fatal("project descendant scratch accepted")
	}
	if err := environmentScratchAllowed(filepath.Join(root, "child"), root); err == nil {
		t.Fatal("project ancestor scratch accepted")
	}
}

func TestEnvironmentExecutableBindingAndDrift(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	resolved, id, digest, err := environmentExecutable(root, goTool)
	if err != nil || resolved == "" || id == "" || digest == "" {
		t.Fatalf("binding: %q %q %q %v", resolved, id, digest, err)
	}
	if !environmentExecutableUnchanged(root, resolved, id, digest) {
		t.Fatal("stable executable changed")
	}
	projectTool := filepath.Join(root, "tool")
	if err := os.WriteFile(projectTool, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := environmentExecutable(root, projectTool); err == nil {
		t.Fatal("project executable accepted")
	}
}

func writeEnvironmentSource(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
