// Package spike qualifies storage/process premises only. It is not the OCMI runtime.
package spike

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const helperFlag = "--ocmi-premise-helper"

func TestMain(m *testing.M) {
	if len(os.Args) >= 4 && os.Args[1] == helperFlag {
		if err := helper(os.Args[2], os.Args[3], os.Args[4:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(71)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func openStore(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", filepath.ToSlash(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	// A single connection makes these per-connection pragmas explicit for this probe.
	_, err = db.Exec("PRAGMA busy_timeout=3000; PRAGMA foreign_keys=ON; PRAGMA synchronous=FULL;")
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func fixture(t *testing.T) (string, *sql.DB) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "synthetic project Ω")
	must(t, os.Mkdir(root, 0700))
	db, err := openStore(filepath.Join(root, "state.sqlite"))
	must(t, err)
	t.Cleanup(func() { must(t, db.Close()) })
	_, err = db.Exec(`PRAGMA journal_mode=WAL;
		CREATE TABLE state(id TEXT PRIMARY KEY, revision INTEGER NOT NULL, owner TEXT NOT NULL);
		INSERT INTO state VALUES ('synthetic-project', 0, '');
		CREATE TABLE effects(owner TEXT NOT NULL);`)
	must(t, err)
	return root, db
}

func helperArgs(mode, root string, args ...string) []string {
	return append([]string{helperFlag, mode, root}, args...)
}

func minimalEnv() []string {
	var env []string
	for _, key := range []string{"SystemRoot", "TEMP", "TMP", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func helperCommand(ctx context.Context, mode, root string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], helperArgs(mode, root, args...)...)
	cmd.Dir, cmd.Env = root, minimalEnv()
	cmd.WaitDelay = time.Second
	return cmd
}

func helper(mode, root string, args []string) error {
	switch mode {
	case "exit":
		if err := os.WriteFile(filepath.Join(root, "effect"), []byte(args[1]), 0600); err != nil {
			return err
		}
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		os.Exit(n)
	case "leaf":
		f, err := os.OpenFile(filepath.Join(root, "heartbeat"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		ready := filepath.Join(root, "ready")
		if err = os.WriteFile(ready+".pending", []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			return err
		}
		// Publish only complete PID bytes. Existence alone is not write completion.
		if err = os.Rename(ready+".pending", ready); err != nil {
			return err
		}
		deadline := time.Now().Add(25 * time.Second)
		for time.Now().Before(deadline) {
			if _, err = f.Write([]byte(".")); err != nil {
				return err
			}
			time.Sleep(20 * time.Millisecond)
		}
		return nil
	case "tree":
		// The descendant inherits the parent's job/process group; it never detaches.
		cmd := helperCommand(context.Background(), "leaf", root)
		return cmd.Run()
	case "supervisor-crash":
		// Deliberately exit without close: Windows must close the noninherited job handle.
		p, err := startOwned(root, "tree")
		if err != nil {
			return err
		}
		defer p.close()
		if err = waitFor(func() bool { _, e := os.Stat(filepath.Join(root, "release")); return e == nil }); err != nil {
			return err
		}
		os.Exit(86)
	case "claim", "crash", "read":
		db, err := openStore(filepath.Join(root, "state.sqlite"))
		if err != nil {
			return err
		}
		defer db.Close()
		if mode == "read" {
			var rev int
			var owner string
			if err = db.QueryRow("SELECT revision,owner FROM state WHERE id='synthetic-project'").Scan(&rev, &owner); err != nil {
				return err
			}
			fmt.Printf("%d:%s", rev, owner)
			return nil
		}
		if mode == "claim" {
			name := args[0]
			if err = os.WriteFile(filepath.Join(root, name+".ready"), []byte("ready"), 0600); err != nil {
				return err
			}
			if err = waitFor(func() bool { _, e := os.Stat(filepath.Join(root, "release")); return e == nil }); err != nil {
				return err
			}
			tx, err := db.Begin()
			if err != nil {
				return err
			}
			defer tx.Rollback()
			res, err := tx.Exec("UPDATE state SET revision=1,owner=? WHERE id='synthetic-project' AND revision=0", name)
			if err != nil {
				return err
			}
			rows, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if rows == 0 {
				fmt.Print("refused")
				return nil
			}
			if _, err = tx.Exec("INSERT INTO effects VALUES (?)", name); err != nil {
				return err
			}
			if err = tx.Commit(); err != nil {
				return err
			}
			fmt.Print("claimed")
			return nil
		}
		if _, err = db.Exec("UPDATE state SET revision=1,owner='committed' WHERE id='synthetic-project'"); err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err = tx.Exec("UPDATE state SET revision=2,owner='uncommitted'"); err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(root, "in-transaction"), []byte("ready"), 0600); err != nil {
			return err
		}
		time.Sleep(25 * time.Second)
		return fmt.Errorf("parent did not interrupt crash fixture")
	}
	return fmt.Errorf("unknown synthetic helper mode")
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func waitFor(condition func() bool) error {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("bounded fixture wait expired")
}
func waitFile(t *testing.T, path string) {
	t.Helper()
	must(t, waitFor(func() bool { _, e := os.Stat(path); return e == nil }))
}

func TestRealCommandExitAndArguments(t *testing.T) {
	for _, code := range []int{0, 7} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			root := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			want := "Unicode Ω with spaces and \"literal quotes\""
			cmd := helperCommand(ctx, "exit", root, strconv.Itoa(code), want)
			err := cmd.Run()
			if code == 0 {
				must(t, err)
			} else if err == nil || cmd.ProcessState.ExitCode() != code {
				t.Fatalf("exit=%v, err=%v", cmd.ProcessState, err)
			}
			data, err := os.ReadFile(filepath.Join(root, "effect"))
			must(t, err)
			if string(data) != want {
				t.Fatalf("argv changed: %q", data)
			}
		})
	}
}

func TestSQLiteTwoProcessClaim(t *testing.T) {
	root, db := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmds := []*exec.Cmd{helperCommand(ctx, "claim", root, "one"), helperCommand(ctx, "claim", root, "two")}
	outs := []strings.Builder{{}, {}}
	for i, c := range cmds {
		c.Stdout = &outs[i]
		c.Stderr = &outs[i]
		must(t, c.Start())
		t.Cleanup(func() {
			if c.ProcessState == nil {
				_ = c.Process.Kill()
				_ = c.Wait()
			}
		})
	}
	for _, name := range []string{"one", "two"} {
		waitFile(t, filepath.Join(root, name+".ready"))
	}
	must(t, os.WriteFile(filepath.Join(root, "release"), []byte("go"), 0600))
	for i, c := range cmds {
		if err := c.Wait(); err != nil {
			t.Fatalf("client %d: %v: %s", i, err, outs[i].String())
		}
	}
	a, b := outs[0].String(), outs[1].String()
	if !((a == "claimed" && b == "refused") || (b == "claimed" && a == "refused")) {
		t.Fatalf("claim outcomes %q %q", a, b)
	}
	var n int
	must(t, db.QueryRow("SELECT count(*) FROM effects").Scan(&n))
	if n != 1 {
		t.Fatalf("effects=%d", n)
	}
	var owner, effect string
	must(t, db.QueryRow("SELECT owner FROM state").Scan(&owner))
	must(t, db.QueryRow("SELECT owner FROM effects").Scan(&effect))
	if owner != effect {
		t.Fatal("owner and committed side effect disagree")
	}
	// A fresh process with the old revision must not create another effect.
	out, err := helperCommand(ctx, "claim", root, "stale").CombinedOutput()
	must(t, err)
	if string(out) != "refused" {
		t.Fatalf("stale claim: %s", out)
	}
	must(t, db.QueryRow("SELECT count(*) FROM effects").Scan(&n))
	if n != 1 {
		t.Fatalf("stale effect=%d", n)
	}
}

func TestSQLiteCrashAndSnapshot(t *testing.T) {
	root, db := fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := helperCommand(ctx, "crash", root)
	must(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	waitFile(t, filepath.Join(root, "in-transaction"))
	must(t, cmd.Process.Kill())
	if err := cmd.Wait(); err == nil {
		t.Fatal("crash fixture exited normally")
	}
	out, err := helperCommand(ctx, "read", root).CombinedOutput()
	must(t, err)
	if string(out) != "1:committed" {
		t.Fatalf("durable state=%q", out)
	}
	destination := filepath.Join(t.TempDir(), "other device Ω with spaces")
	must(t, os.Mkdir(destination, 0700))
	snapshot := filepath.Join(destination, "state.sqlite")
	// SQLite creates a standalone consistent database while the source remains open in WAL mode.
	_, err = db.Exec("VACUUM INTO ?", snapshot)
	must(t, err)
	out, err = helperCommand(ctx, "read", destination).CombinedOutput()
	must(t, err)
	if string(out) != "1:committed" {
		t.Fatalf("restored state=%q", out)
	}
	restored, err := openStore(snapshot)
	must(t, err)
	defer restored.Close()
	var integrity string
	must(t, restored.QueryRow("PRAGMA integrity_check").Scan(&integrity))
	if integrity != "ok" {
		t.Fatalf("snapshot integrity=%s", integrity)
	}
	if _, err = db.Exec("VACUUM INTO ?", snapshot); err == nil {
		t.Fatal("existing snapshot overwritten")
	}
}

func TestOwnedTreeCancellation(t *testing.T) {
	ownedRoot, foreignRoot := t.TempDir(), t.TempDir()
	owned, err := startOwned(ownedRoot, "tree")
	must(t, err)
	t.Cleanup(func() { must(t, owned.close()) })
	foreign, err := startOwned(foreignRoot, "leaf")
	must(t, err)
	t.Cleanup(func() { must(t, foreign.close()) })
	waitFile(t, filepath.Join(ownedRoot, "ready"))
	waitFile(t, filepath.Join(foreignRoot, "ready"))
	size := func(root string) int64 {
		s, e := os.Stat(filepath.Join(root, "heartbeat"))
		must(t, e)
		return s.Size()
	}
	must(t, waitFor(func() bool { return size(ownedRoot) > 2 && size(foreignRoot) > 2 }))
	// Cancellation owns a retained job/process-group handle, never a user-supplied PID.
	must(t, owned.close())
	beforeOwned, beforeForeign := size(ownedRoot), size(foreignRoot)
	must(t, waitFor(func() bool { return size(foreignRoot) > beforeForeign+5 }))
	if size(ownedRoot) != beforeOwned {
		t.Fatal("owned descendant still writes after cancellation")
	}
}
