package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkerFactoryNameClosesPrivateNamespaces(t *testing.T) {
	for _, name := range []string{"unowned.txt", "system/OCMI_Sync/coordinator/private.txt", "system/local/bindings/private.json", "system/docs/memory/private.md"} {
		if factoryName(name) {
			t.Fatalf("accepted private or unowned factory member %q", name)
		}
	}
	for _, name := range []string{"README.md", "system/VERSION.md", "system/packaging/home/text.json", "system/templates/work/docs/memory/INDEX.md", "system/locales/ru/templates/work/docs/memory/INDEX.md"} {
		if !factoryName(name) {
			t.Fatalf("rejected public factory member %q", name)
		}
	}
}

func TestWorkerInstallationReadFactoryMetadata(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "home with spaces Ω")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "SHA256SUMS")
	if err := os.WriteFile(path, []byte("fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := plainComponents(root, "SHA256SUMS", false); err != nil {
		t.Fatalf("components: %v", err)
	}
	if _, err := localNativeID(root); err != nil {
		t.Fatalf("root identity: %v", err)
	}
	if err := environmentPlainAncestors(root); err != nil {
		t.Fatalf("plain ancestors: %v", err)
	}
	opened, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	_ = opened.Close()
	if raw, err := installationRead(root, "SHA256SUMS", 1024); err != nil || string(raw) != "fixture\n" {
		t.Fatalf("installation read raw=%q err=%v", raw, err)
	}
}

func TestWorkerInstallationMetadataWriterAndAdmission(t *testing.T) {
	root := t.TempDir()
	if err := installationWrite(root, "local/bindings/value.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if got, err := installationRead(root, "local/bindings/value.json", 16); err != nil || string(got) != "{}" {
		t.Fatalf("read=%q err=%v", got, err)
	}
	if _, err := installationRead(root, "local/bindings/missing.json", 16); !os.IsNotExist(err) {
		t.Fatalf("missing err=%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.json"), make([]byte, 17), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := installationRead(root, "large.json", 16); err == nil {
		t.Fatal("accepted oversize")
	}
	context := layoutContext{Root: root, Paths: map[string]string{"bindings": "local/bindings"}}
	first, err := installationAdmission(context)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installationAdmission(context); err == nil {
		t.Fatal("second admission acquired")
	}
	first()
	if second, err := installationAdmission(context); err != nil {
		t.Fatal(err)
	} else {
		second()
	}
	ancestor := filepath.Join(root, "unsafe")
	if err := os.WriteFile(ancestor, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := installationWrite(root, "unsafe/child.json", []byte("x")); err == nil {
		t.Fatal("accepted unsafe ancestor")
	}
}
