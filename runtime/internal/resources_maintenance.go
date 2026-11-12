package internal

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

type resourceMaintenanceInput struct {
	ResourceID     string `json:"resourceId"`
	SizeLimitBytes int64  `json:"sizeLimitBytes"`
	AgeLimitDays   int64  `json:"ageLimitDays"`
	MaxEntries     int    `json:"maxEntries"`
	MaxBytes       int64  `json:"maxBytes"`
}

// resourceMaintenanceRead measures selected original resources, never content
// or deletion eligibility. All uncertain observations remain explicitly incomplete.
func resourceMaintenanceRead(r Request) Response {
	fail := func(code string, rev int64) Response {
		out := bad(code, rev)
		out.Data = map[string]any{"complete": false, "mutationSupported": false, "nextAction": "review selected resource and preserve unchanged"}
		return out
	}
	db, err := openExistingFor(r)
	if err != nil {
		return fail("STATE_UNAVAILABLE", 0)
	}
	defer db.Close()
	var rev int64
	version, err := databaseVersion(db)
	if err != nil {
		return fail("STATE_CORRUPT", 0)
	}
	if version != stateSchemaVersion {
		return fail("UPGRADE_REQUIRED", 0)
	}
	if db.QueryRow("SELECT revision FROM project").Scan(&rev) != nil {
		return fail("STATE_CORRUPT", rev)
	}
	var in resourceMaintenanceInput
	if !toolStorageExact(r.Payload, &in, "resourceId", "sizeLimitBytes", "ageLimitDays", "maxEntries", "maxBytes") || in.ResourceID == "" || len(in.ResourceID) > 1024 || in.SizeLimitBytes < 1 || in.SizeLimitBytes > 1<<50 || in.AgeLimitDays < 1 || in.AgeLimitDays > 36500 || in.MaxEntries < 1 || in.MaxEntries > 1024 || in.MaxBytes < 1 || in.MaxBytes > 1<<50 {
		return fail("INVALID_RESOURCE_MAINTENANCE", rev)
	}
	var provider, kind, ownership, mode, state, locator, native, inventory, purpose, ownerRef, lifetime, transfer string
	var held, active, imported, applying int
	err = db.QueryRow(`SELECT l.provider,l.kind,l.ownership,l.mode,l.observed_state,l.locator,l.native_identity,l.inventory,l.purpose,l.owner_ref,l.lifetime,l.held,l.imported,l.applying,l.transfer_id
	 FROM resource_ledger l JOIN project p ON l.project_id=p.id AND l.copy_id=p.copy_id AND l.device_id=p.device_id WHERE l.id=?`, in.ResourceID).Scan(&provider, &kind, &ownership, &mode, &state, &locator, &native, &inventory, &purpose, &ownerRef, &lifetime, &held, &imported, &applying, &transfer)
	if err != nil || len(purpose) > 1024 || len(ownerRef) > 1024 || len(lifetime) > 1024 || db.QueryRow("SELECT count(*) FROM resource_use WHERE resource_id=? AND active=1", in.ResourceID).Scan(&active) != nil {
		return fail("RESOURCE_MAINTENANCE_INCOMPLETE", rev)
	}
	protected := in.ResourceID == "ocmi-state"
	data := map[string]any{"complete": false, "mutationSupported": false, "activeUses": active, "held": held != 0, "protected": protected || active != 0 || held != 0, "purpose": purpose, "owner": ownerRef, "lifetime": lifetime, "nextAction": "review selected resource and preserve unchanged"}
	incomplete := func() Response {
		return Response{SchemaVersion: schemaVersion, Code: "RESOURCE_MAINTENANCE_INCOMPLETE", Revision: rev, Data: data}
	}
	if ownership != "project" || imported != 0 || applying != 0 || transfer != "" {
		return incomplete()
	}
	layout, code := selectedLayout(r)
	if code != "" {
		return incomplete()
	}
	measurement := resourceMaintenanceMeasurement{input: in, now: time.Now()}
	if protected {
		if provider != "internal" || kind != "state" || mode != "use" || state != "keep" || locator != "" || native != "" || inventory != "" {
			return incomplete()
		}
		parent := filepath.Dir(layout.Store)
		rel, err := filepath.Rel(layout.Root, parent)
		if err != nil || plainComponents(layout.Root, rel, false) != nil {
			return incomplete()
		}
		root, parentID, err := openHeldResourceRoot(layout.Root, rel, "")
		if err != nil {
			return incomplete()
		}
		defer root.Close()
		name := filepath.Base(layout.Store)
		for i, suffix := range []string{"", "-wal", "-shm"} {
			if err = measurement.file(root, name+suffix, i != 0); err != nil {
				return incomplete()
			}
		}
		currentParentID, placementErr := localNativeID(parent)
		if measurement.verify(root) != nil || placementErr != nil || currentParentID != parentID || plainComponents(layout.Root, rel, false) != nil {
			return incomplete()
		}
	} else {
		if provider != "local" || mode != "create" || (kind != "directory" && kind != "file") || (state != "active" && state != "keep") || native == "" || inventory == "" {
			return incomplete()
		}
		name, err := localName(locator, trustedWorkspaceLocator(in.ResourceID, locator))
		if err != nil || plainComponents(layout.Root, name, false) != nil {
			return incomplete()
		}
		parent, _, err := openHeldResourceRoot(layout.Root, filepath.Dir(name), "")
		if err != nil {
			return incomplete()
		}
		defer parent.Close()
		leaf := filepath.Base(name)
		info, err := parent.Lstat(leaf)
		if err != nil || !resourceMaintenancePlain(info) || (kind == "file" && !info.Mode().IsRegular()) || (kind == "directory" && !info.IsDir()) {
			return incomplete()
		}
		f, err := parent.Open(leaf)
		if err != nil {
			return incomplete()
		}
		id, idErr := localNativeIDFile(f)
		opened, statErr := f.Stat()
		closeErr := f.Close()
		if idErr != nil || statErr != nil || closeErr != nil || id != native || !os.SameFile(info, opened) {
			return incomplete()
		}
		if kind == "file" {
			err = measurement.file(parent, leaf, false)
			if err == nil {
				err = measurement.verify(parent)
			}
		} else {
			var root *os.Root
			root, _, err = openHeldResourceRoot(layout.Root, name, native)
			if err == nil {
				err = measurement.walk(root, ".")
				if err == nil {
					err = measurement.verify(root)
				}
				if closeErr := root.Close(); err == nil {
					err = closeErr
				}
			}
		}
		current, placementErr := parent.Lstat(leaf)
		currentID, identityErr := localNativeID(filepath.Join(layout.Root, name))
		if err != nil || placementErr != nil || identityErr != nil || currentID != native || !resourceMaintenanceSame(info, current) || plainComponents(layout.Root, name, false) != nil {
			return incomplete()
		}
	}
	data["complete"], data["observedEntries"], data["observedBytes"], data["oldestAgeDays"] = true, measurement.entries, measurement.bytes, measurement.oldest
	data["sizeExceeded"], data["ageExceeded"] = measurement.bytes > in.SizeLimitBytes, measurement.oldest > in.AgeLimitDays
	data["maintenanceRequired"] = measurement.bytes > in.SizeLimitBytes || measurement.oldest > in.AgeLimitDays
	data["nextAction"] = "review retention indicators; thresholds grant no deletion authority"
	return good("RESOURCE_MAINTENANCE_INSPECTED", rev, data)
}

type resourceMaintenanceSnapshot struct {
	name string
	info os.FileInfo // nil records a specifically observed absent SQLite sidecar.
}

type resourceMaintenanceMeasurement struct {
	input   resourceMaintenanceInput
	now     time.Time
	entries int
	bytes   int64
	oldest  int64
	seen    []resourceMaintenanceSnapshot
}

func resourceMaintenancePlain(info os.FileInfo) bool {
	return info != nil && (info.IsDir() || info.Mode().IsRegular()) && info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0
}

func resourceMaintenanceSame(a, b os.FileInfo) bool {
	return resourceMaintenancePlain(a) && resourceMaintenancePlain(b) && os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func (m *resourceMaintenanceMeasurement) record(name string, info os.FileInfo) error {
	if !resourceMaintenancePlain(info) {
		return errors.New("unverified metadata")
	}
	if info.Mode().IsRegular() {
		if info.Size() < 0 || info.Size() > m.input.MaxBytes-m.bytes {
			return errors.New("measurement bound")
		}
		m.bytes += info.Size()
		age := int64(m.now.Sub(info.ModTime()).Hours() / 24)
		if age > m.oldest {
			m.oldest = age
		}
	}
	m.seen = append(m.seen, resourceMaintenanceSnapshot{name, info})
	return nil
}

func (m *resourceMaintenanceMeasurement) file(root *os.Root, name string, optional bool) error {
	info, err := root.Lstat(name)
	if os.IsNotExist(err) && optional {
		m.seen = append(m.seen, resourceMaintenanceSnapshot{name, nil})
		return nil
	}
	if err != nil || !resourceMaintenancePlain(info) || !info.Mode().IsRegular() {
		return errors.New("unverified metadata")
	}
	if m.entries >= m.input.MaxEntries {
		return errors.New("measurement bound")
	}
	m.entries++
	return m.record(name, info)
}

func (m *resourceMaintenanceMeasurement) walk(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if err != nil || !resourceMaintenancePlain(info) {
		return errors.New("unverified metadata")
	}
	if err = m.record(name, info); err != nil || !info.IsDir() {
		return err
	}
	dir, err := root.Open(name)
	if err != nil {
		return err
	}
	opened, statErr := dir.Stat()
	// Reserve every enumerated entry before descent: total listing work is bounded.
	var list []os.DirEntry
	var readErr error
	for {
		batch, e := dir.ReadDir(m.input.MaxEntries - m.entries - len(list) + 1)
		list = append(list, batch...)
		if len(list) > m.input.MaxEntries-m.entries {
			readErr = errors.New("measurement bound")
			break
		}
		if e == io.EOF {
			break
		}
		if e != nil || len(batch) == 0 {
			readErr = errors.New("unverified enumeration")
			break
		}
	}
	closeErr := dir.Close()
	if statErr != nil || !resourceMaintenanceSame(info, opened) || readErr != nil || closeErr != nil {
		return errors.New("unverified metadata")
	}
	if len(list) > m.input.MaxEntries-m.entries {
		return errors.New("measurement bound")
	}
	m.entries += len(list)
	for _, entry := range list {
		if err := m.walk(root, filepath.Join(name, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (m *resourceMaintenanceMeasurement) verify(root *os.Root) error {
	for _, snapshot := range m.seen {
		info, err := root.Lstat(snapshot.name)
		if snapshot.info == nil && os.IsNotExist(err) {
			continue
		}
		if err != nil || !resourceMaintenanceSame(snapshot.info, info) {
			return errors.New("metadata changed")
		}
	}
	return nil
}
