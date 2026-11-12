package internal

// Independent behavioral acceptance for selected installation roots. Requests are
// decoded through the public JSON shape so the old implementation remains a
// compilable, causally failing control. No implementation resolver is imported.
import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

func acceptanceHomeDeclaration() map[string]any {
	return map[string]any{
		"schemaVersion": 1, "layoutVersion": 1, "kind": "installation",
		"paths": map[string]string{
			"system": "system", "bin": "system/bin", "sync": "OCMI_Sync",
			"coordinator": "OCMI_Sync/coordinator", "personal": "OCMI_Sync/personal",
			"recovery": "OCMI_Sync/recovery", "local": "local", "state": "local/state",
			"store": "local/state/state.sqlite", "bindings": "local/bindings",
			"cache": "local/cache", "runs": "local/runs",
		},
	}
}

func acceptanceWriteHome(t *testing.T, root string, declaration any) {
	t.Helper()
	raw, err := json.Marshal(declaration)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".ocmi-home.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func acceptanceRootRequest(t *testing.T, root, kind, action, key string, revision int64, payload any) Response {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "projectRoot": root, "rootKind": kind, "actor": "main",
		"action": action, "requestId": key, "expectedRevision": revision, "payload": payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	var request Request
	if err = json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	return Handle(request)
}

func acceptanceInit(t *testing.T, root, kind, key string) Response {
	t.Helper()
	return acceptanceRootRequest(t, root, kind, "init", key, 0, map[string]string{
		"projectId": "fictional-root", "deviceId": "device", "copyId": "copy", "owner": "main",
	})
}

func acceptanceAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected artifact at %s: %v", path, err)
	}
}

func TestRootLayoutAcceptanceProjectCompatibility(t *testing.T) {
	root := t.TempDir()
	first := acceptanceInit(t, root, "", "legacy-init")
	if !first.OK {
		t.Fatal(first)
	}
	if _, err := os.Stat(filepath.Join(root, ".ocmi", "local", "state.sqlite")); err != nil {
		t.Fatal(err)
	}
	if replay := acceptanceInit(t, root, "", "legacy-init"); !replay.OK || replay.Revision != first.Revision {
		t.Fatal(replay)
	}
	if status := acceptanceRootRequest(t, root, "project", "status", "legacy-status", 0, map[string]any{}); status.OK || status.Code != "TASK_UNBOUND" {
		t.Fatal(status)
	}
	acceptanceAbsent(t, filepath.Join(root, "local"))
}

func TestRootLayoutAcceptanceInspectionDoesNotInitialize(t *testing.T) {
	for _, kind := range []string{"project", "installation"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if kind == "installation" {
				acceptanceWriteHome(t, root, acceptanceHomeDeclaration())
			}
			response := acceptanceRootRequest(t, root, kind, "layout.inspect", "inspect", 0, map[string]any{})
			if !response.OK || response.Data["rootKind"] != kind {
				t.Fatal(response)
			}
			raw, _ := json.Marshal(response.Data["paths"])
			paths := map[string]string{}
			if json.Unmarshal(raw, &paths) != nil {
				t.Fatal(response)
			}
			expected := ".ocmi/local/state.sqlite"
			if kind == "installation" {
				expected = "local/state/state.sqlite"
			}
			if paths["store"] != expected {
				t.Fatal(response)
			}
			acceptanceAbsent(t, filepath.Join(root, ".ocmi"))
			acceptanceAbsent(t, filepath.Join(root, "local"))
			acceptanceAbsent(t, filepath.Join(root, "OCMI_Sync"))
		})
	}
}

func TestRootLayoutAcceptanceHomeState(t *testing.T) {
	root := t.TempDir()
	acceptanceWriteHome(t, root, acceptanceHomeDeclaration())
	first := acceptanceInit(t, root, "installation", "home-init")
	if !first.OK {
		t.Fatal(first)
	}
	if _, err := os.Stat(filepath.Join(root, "local", "state", "state.sqlite")); err != nil {
		t.Fatalf("installation init did not use the declared state path: %v", err)
	}
	if replay := acceptanceInit(t, root, "installation", "home-init"); !replay.OK || replay.Revision != first.Revision {
		t.Fatal(replay)
	}
	if status := acceptanceRootRequest(t, root, "installation", "status", "home-status", 0, map[string]any{}); status.OK || status.Code != "TASK_UNBOUND" {
		t.Fatal(status)
	}
	for _, kind := range []string{"", "project", "unexpected"} {
		if response := acceptanceInit(t, root, kind, "wrong-kind-"+kind); response.OK {
			t.Fatalf("incorrect profile admitted: %s: %+v", kind, response)
		}
	}
	acceptanceAbsent(t, filepath.Join(root, ".ocmi"))
	acceptanceAbsent(t, filepath.Join(root, "OCMI_Sync"))
}

func TestRootLayoutAcceptanceDescriptorRefusal(t *testing.T) {
	for _, mutation := range []string{"missing", "malformed", "version", "escape", "unknown", "directory"} {
		t.Run(mutation, func(t *testing.T) {
			root := t.TempDir()
			declaration := acceptanceHomeDeclaration()
			switch mutation {
			case "version":
				declaration["layoutVersion"] = 99
			case "escape":
				declaration["paths"].(map[string]string)["store"] = "../outside.sqlite"
			case "unknown":
				declaration["override"] = "elsewhere"
			}
			if mutation != "missing" && mutation != "directory" {
				acceptanceWriteHome(t, root, declaration)
			}
			if mutation == "malformed" {
				if err := os.WriteFile(filepath.Join(root, ".ocmi-home.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mutation == "directory" {
				if err := os.Mkdir(filepath.Join(root, ".ocmi-home.json"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if response := acceptanceInit(t, root, "installation", "invalid-init"); response.OK {
				t.Fatal(response)
			}
			acceptanceAbsent(t, filepath.Join(root, ".ocmi"))
			acceptanceAbsent(t, filepath.Join(root, "local"))
		})
	}
}

func TestRootLayoutAcceptanceNoImplicitMigrationOrReservedResources(t *testing.T) {
	root := t.TempDir()
	if response := acceptanceInit(t, root, "", "legacy"); !response.OK {
		t.Fatal(response)
	}
	path := filepath.Join(root, ".ocmi", "local", "state.sqlite")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	acceptanceWriteHome(t, root, acceptanceHomeDeclaration())
	if response := acceptanceInit(t, root, "installation", "do-not-migrate"); response.OK {
		t.Fatal(response)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatal("legacy state changed", err)
	}
	acceptanceAbsent(t, filepath.Join(root, "local"))

	home := t.TempDir()
	acceptanceWriteHome(t, home, acceptanceHomeDeclaration())
	if response := acceptanceInit(t, home, "installation", "home"); !response.OK {
		t.Fatal(response)
	}
	for i, locator := range []string{".ocmi-home.json", "system/unowned.txt", "local/state/unowned.txt", "local/bindings/unowned.txt", "local/cache/unowned.txt"} {
		response := acceptanceRootRequest(t, home, "installation", "resource.intent", "reserved-"+strconv.Itoa(i), 0, map[string]any{
			"resourceId": "reserved-" + strconv.Itoa(i), "provider": "local", "kind": "file", "mode": "create",
			"locator": locator, "purpose": "fictional scope check", "lifetime": "task", "disposition": "remove", "ownerRef": "main",
		})
		if response.OK {
			t.Fatalf("reserved installation resource admitted: %s", locator)
		}
	}
}

func TestRootLayoutAcceptanceConcurrentProfiles(t *testing.T) {
	var group sync.WaitGroup
	for i := 0; i < 12; i++ {
		root := t.TempDir()
		kind := "project"
		if i%2 == 0 {
			kind = "installation"
			acceptanceWriteHome(t, root, acceptanceHomeDeclaration())
		}
		group.Add(1)
		go func(root, kind string) {
			defer group.Done()
			if response := acceptanceInit(t, root, kind, "parallel-init"); !response.OK {
				t.Error(response)
				return
			}
			store := filepath.Join(root, ".ocmi", "local", "state.sqlite")
			if kind == "installation" {
				store = filepath.Join(root, "local", "state", "state.sqlite")
			}
			if _, err := os.Stat(store); err != nil {
				t.Errorf("profile %s redirected: %v", kind, err)
			}
		}(root, kind)
	}
	group.Wait()
}

func TestRootLayoutAcceptanceMarkerDriftAndLinkedState(t *testing.T) {
	t.Run("descriptor-change", func(t *testing.T) {
		root := t.TempDir()
		acceptanceWriteHome(t, root, acceptanceHomeDeclaration())
		if response := acceptanceInit(t, root, "installation", "first"); !response.OK {
			t.Fatal(response)
		}
		store := filepath.Join(root, "local", "state", "state.sqlite")
		before, err := os.ReadFile(store)
		if err != nil {
			t.Fatal(err)
		}
		changed := acceptanceHomeDeclaration()
		changed["layoutVersion"] = 99
		acceptanceWriteHome(t, root, changed)
		if response := acceptanceRootRequest(t, root, "installation", "status", "changed", 0, map[string]any{}); response.OK || response.Code == "TASK_UNBOUND" {
			t.Fatal("changed descriptor was ignored", response)
		}
		after, err := os.ReadFile(store)
		if err != nil || string(before) != string(after) {
			t.Fatal("invalid descriptor changed existing state", err)
		}
		acceptanceAbsent(t, filepath.Join(root, ".ocmi"))
		acceptanceWriteHome(t, root, acceptanceHomeDeclaration())
		if response := acceptanceInit(t, root, "installation", "first"); !response.OK {
			t.Fatal("original state did not remain recoverable", response)
		}
	})
	t.Run("state-link-into-sync", func(t *testing.T) {
		root := t.TempDir()
		acceptanceWriteHome(t, root, acceptanceHomeDeclaration())
		syncRoot := filepath.Join(root, "OCMI_Sync")
		if err := os.Mkdir(syncRoot, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(syncRoot, filepath.Join(root, "local")); err != nil {
			t.Skipf("directory-link creation unavailable: %v", err)
		}
		if response := acceptanceInit(t, root, "installation", "linked"); response.OK {
			t.Fatal("live state was admitted through a link into sync", response)
		}
		acceptanceAbsent(t, filepath.Join(syncRoot, "state"))
		acceptanceAbsent(t, filepath.Join(root, ".ocmi"))
	})
}
