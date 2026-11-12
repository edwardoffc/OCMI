package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerUpdateAdmission(t *testing.T) {
	t.Run("normal-contention", func(t *testing.T) {
		parent := t.TempDir()
		lock, err := setupLockOwned(parent)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = setupLockOwned(parent); err == nil {
			t.Fatal("accepted occupied lock")
		}
		if err = lock.Release(); err != nil {
			t.Fatal(err)
		}
		if _, err = os.Lstat(lock.path); !os.IsNotExist(err) {
			t.Fatal("lock remains")
		}
	})
	t.Run("replacement", func(t *testing.T) {
		parent := t.TempDir()
		lock, err := setupLockOwned(parent)
		if err != nil {
			t.Fatal(err)
		}
		original := filepath.Join(parent, "original")
		if err = os.Rename(lock.path, original); err != nil {
			t.Fatal(err)
		}
		if err = os.Mkdir(lock.path, 0700); err != nil {
			t.Fatal(err)
		}
		if err = lock.Release(); err == nil {
			t.Fatal("accepted replacement")
		}
		if _, err = os.Stat(original); err != nil {
			t.Fatal("original lost")
		}
		if _, err = os.Stat(lock.path); err != nil {
			t.Fatal("replacement lost")
		}
	})
	t.Run("foreign-content", func(t *testing.T) {
		parent := t.TempDir()
		lock, err := setupLockOwned(parent)
		if err != nil {
			t.Fatal(err)
		}
		foreign := filepath.Join(lock.path, "foreign")
		if err = os.WriteFile(foreign, []byte("preserve"), 0600); err != nil {
			t.Fatal(err)
		}
		if err = lock.Release(); err == nil {
			t.Fatal("removed nonempty lock")
		}
		if raw, err := os.ReadFile(foreign); err != nil || string(raw) != "preserve" {
			t.Fatal("foreign content lost")
		}
	})
	t.Run("recovery-writer", func(t *testing.T) {
		base := t.TempDir()
		home := filepath.Join(base, "home")
		acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
		if r := InstallHome(home, false); !r.OK {
			t.Fatal(r)
		}
		context, code := resolveLayout(home, "installation")
		if code != "" {
			t.Fatal(code)
		}
		lock, err := setupLockOwned(filepath.Join(home, filepath.FromSlash(context.Paths["bindings"])))
		if err != nil {
			t.Fatal(err)
		}
		if err = releaseRecordedSetupLock(context, lock.nativeID, nil); err == nil {
			t.Fatal("released without writer")
		}
		writer, err := acquireUpdateRecoveryWriter(context)
		if err != nil {
			t.Fatal(err)
		}
		defer writer.Release()
		if competing, err := acquireUpdateRecoveryWriter(context); err == nil {
			competing.Release()
			t.Fatal("accepted live competing writer")
		}
		if err = releaseRecordedSetupLock(context, "foreign", writer); err == nil {
			t.Fatal("accepted foreign identity")
		}
		if err = releaseRecordedSetupLock(context, lock.nativeID, writer); err != nil {
			t.Fatal(err)
		}
	})
}
