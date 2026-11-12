package internal

import (
	stdcontext "context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
)

type ownedSetupLock struct{ path, nativeID string }

func setupLockOwned(parent string) (*ownedSetupLock, error) {
	if !filepath.IsAbs(parent) {
		return nil, errors.New("unsafe setup parent")
	}
	ancestor := parent
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return nil, errors.New("unsafe setup parent")
		}
		ancestor = next
	}
	if environmentPlainAncestors(ancestor) != nil {
		return nil, errors.New("unsafe setup parent")
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	if environmentPlainAncestors(parent) != nil || plainPath(parent, true) != nil {
		return nil, errors.New("unsafe setup parent")
	}
	path := filepath.Join(parent, ".setup-lock")
	if err := os.Mkdir(path, 0700); err != nil {
		return nil, err
	}
	id, err := localNativeID(path)
	if err != nil || id == "" {
		return nil, errors.New("setup identity unavailable")
	}
	return &ownedSetupLock{path: path, nativeID: id}, nil
}

func (lock *ownedSetupLock) Release() error {
	if lock == nil || lock.path == "" || lock.nativeID == "" || environmentPlainAncestors(lock.path) != nil || plainPath(lock.path, true) != nil {
		return errors.New("unsafe setup lock")
	}
	id, err := localNativeID(lock.path)
	if err != nil || id != lock.nativeID {
		return errors.New("setup lock identity changed")
	}
	entries, err := os.ReadDir(lock.path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("setup lock not empty")
	}
	id, err = localNativeID(lock.path)
	if err != nil || id != lock.nativeID {
		return errors.New("setup lock identity changed")
	}
	return os.Remove(lock.path)
}

// heldUpdateWriter is acquired from the actual installation database.  A
// decoded path or journal alone cannot construct recovery deletion authority.
type heldUpdateWriter struct {
	db     *sql.DB
	conn   *sql.Conn
	homeID string
}

func acquireUpdateRecoveryWriter(context layoutContext) (*heldUpdateWriter, error) {
	id, err := localNativeID(context.Root)
	if err != nil {
		return nil, err
	}
	db, err := openLayout(context)
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(stdcontext.Background())
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err = conn.ExecContext(stdcontext.Background(), "BEGIN IMMEDIATE"); err != nil {
		_ = conn.Close()
		_ = db.Close()
		return nil, err
	}
	return &heldUpdateWriter{db: db, conn: conn, homeID: id}, nil
}
func (writer *heldUpdateWriter) Release() {
	if writer != nil && writer.db != nil {
		_, _ = writer.conn.ExecContext(stdcontext.Background(), "ROLLBACK")
		_ = writer.conn.Close()
		_ = writer.db.Close()
		writer.db = nil
		writer.conn = nil
	}
}

func releaseRecordedSetupLock(context layoutContext, nativeID string, writer *heldUpdateWriter) error {
	if writer == nil || writer.db == nil || writer.conn == nil || nativeID == "" {
		return errors.New("writer required")
	}
	id, err := localNativeID(context.Root)
	if err != nil || id != writer.homeID {
		return errors.New("home identity changed")
	}
	lock := ownedSetupLock{path: filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"]), ".setup-lock"), nativeID: nativeID}
	return lock.Release()
}

// updateDBReady reads through the connection already holding BEGIN IMMEDIATE.
// Failure categories are bounded and contain no database contents.
func updateDBReady(writer *heldUpdateWriter, receipt installationReceipt) error {
	if writer == nil || writer.conn == nil || writer.db == nil || !receiptValid(receipt) || writer.homeID != receipt.HomeID {
		return errors.New("invalid admission")
	}
	ctx := stdcontext.Background()
	var count, version int
	if err := writer.conn.QueryRowContext(ctx, "SELECT count(*),COALESCE(min(schema_version),0) FROM meta").Scan(&count, &version); err != nil || count != 1 || version != stateSchemaVersion {
		return errors.New("state schema")
	}
	var project, device, copyID, owner string
	if err := writer.conn.QueryRowContext(ctx, "SELECT count(*) FROM project").Scan(&count); err != nil || count != 1 {
		return errors.New("installation identity")
	}
	if err := writer.conn.QueryRowContext(ctx, "SELECT id,device_id,copy_id,owner FROM project").Scan(&project, &device, &copyID, &owner); err != nil || project != receipt.InstallationID || device != receipt.DeviceID || copyID != receipt.CopyID || owner != "coordinator" {
		return errors.New("installation identity")
	}
	queries := []string{
		"SELECT count(*) FROM operation WHERE state IS NULL OR state NOT IN ('succeeded','failed','incomplete')",
		"SELECT count(*) FROM resource_use WHERE active IS NULL OR active NOT IN (0,1) OR active=1",
		"SELECT count(*) FROM resource_ledger WHERE applying IS NULL OR applying NOT IN (0,1) OR applying=1 OR observed_state IS NULL OR observed_state NOT IN ('active','keep','removed','stopped')",
	}
	for _, query := range queries {
		if err := writer.conn.QueryRowContext(ctx, query).Scan(&count); err != nil || count != 0 {
			return errors.New("unresolved state")
		}
	}
	if err := writer.conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='transfer'").Scan(&count); err != nil || count > 1 {
		return errors.New("transfer state")
	}
	if count == 1 {
		if err := writer.conn.QueryRowContext(ctx, "SELECT count(*) FROM transfer WHERE state IS NULL OR state NOT IN ('acknowledged','imported')").Scan(&count); err != nil || count != 0 {
			return errors.New("transfer state")
		}
	}
	return nil
}
