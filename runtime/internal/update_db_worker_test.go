package internal

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerUpdateDBReady(t *testing.T) {
	cases := []struct {
		name, sql string
		ok        bool
	}{
		{"clean", "", true},
		{"stop", "UPDATE project SET stopped=1", true},
		{"identity", "UPDATE project SET device_id='foreign'", false},
		{"schema", "UPDATE meta SET schema_version=3", false},
		{"operation", `INSERT INTO task VALUES('t','o',1,'active','w','r','c','[]','[]','','','', 'target'); INSERT INTO operation VALUES('op','t','running',NULL,'',0,'','','now')`, false},
		{"unknown-operation", `INSERT INTO task VALUES('t','o',1,'active','w','r','c','[]','[]','','','', 'target'); INSERT INTO operation VALUES('op','t','surprise',NULL,'',0,'','','now')`, false},
		{"active-use", `INSERT INTO resource_use(resource_id,task_id,task_revision,actor,access,reason,active) VALUES('ocmi-state','t',1,'w','read','test',1)`, false},
		{"resource-pending", `UPDATE resource_ledger SET observed_state='intent' WHERE id='ocmi-state'`, false},
		{"resource-applying", `UPDATE resource_ledger SET applying=1 WHERE id='ocmi-state'`, false},
		{"resource-unknown", `UPDATE resource_ledger SET observed_state='surprise' WHERE id='ocmi-state'`, false},
		{"transfer-pending", `CREATE TABLE transfer(id TEXT PRIMARY KEY,generation INTEGER NOT NULL,state TEXT NOT NULL,package_json TEXT NOT NULL,destination TEXT NOT NULL DEFAULT ''); INSERT INTO transfer(id,generation,state,package_json) VALUES('x',1,'exported','{}')`, false},
		{"transfer-complete", `CREATE TABLE transfer(id TEXT PRIMARY KEY,generation INTEGER NOT NULL,state TEXT NOT NULL,package_json TEXT NOT NULL,destination TEXT NOT NULL DEFAULT ''); INSERT INTO transfer(id,generation,state,package_json) VALUES('x',1,'acknowledged','{}')`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "home")
			acceptanceUpdateFactory(t, home, strings.Repeat("a", 40))
			if r := InstallHome(home, false); !r.OK {
				t.Fatal(r)
			}
			context, code := resolveLayout(home, "installation")
			if code != "" {
				t.Fatal(code)
			}
			receipt, err := readInstallationReceiptContext(context)
			if err != nil {
				t.Fatal(err)
			}
			if tc.sql != "" {
				db, err := openLayout(context)
				if err != nil {
					t.Fatal(err)
				}
				_, err = db.Exec(tc.sql)
				_ = db.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			writer, err := acquireUpdateRecoveryWriter(context)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Release()
			err = updateDBReady(writer, receipt)
			if (err == nil) != tc.ok {
				t.Fatalf("ready=%v error=%v", tc.ok, err)
			}
		})
	}
}
