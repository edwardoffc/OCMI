package internal

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed installation-layout.json
var installationLayoutJSON []byte

const homeMarker = ".ocmi-home.json"

type layoutDeclaration struct {
	SchemaVersion int               `json:"schemaVersion"`
	LayoutVersion int               `json:"layoutVersion"`
	Kind          string            `json:"kind"`
	Paths         map[string]string `json:"paths"`
}

type layoutContext struct {
	Kind, Root, Local, State, Store, Runs, InitLock string
	Paths                                           map[string]string
}

func strictDeclaration(raw []byte) (layoutDeclaration, error) {
	var declaration layoutDeclaration
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&declaration) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return layoutDeclaration{}, errors.New("invalid declaration")
	}
	if declaration.SchemaVersion != 1 || declaration.LayoutVersion != 1 || declaration.Kind != "installation" || len(declaration.Paths) == 0 {
		return layoutDeclaration{}, errors.New("invalid declaration")
	}
	for key, value := range declaration.Paths {
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
		if key == "" || value == "" || filepath.IsAbs(filepath.FromSlash(value)) || clean != value || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return layoutDeclaration{}, errors.New("invalid declaration")
		}
	}
	return declaration, nil
}

func plainPath(path string, wantDirectory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 || (wantDirectory && !info.IsDir()) || (!wantDirectory && !info.Mode().IsRegular()) {
		return errors.New("unsafe layout path")
	}
	return nil
}

func resolveLayout(root, kind string) (layoutContext, string) {
	if kind == "" {
		kind = "project"
	}
	if kind != "project" && kind != "installation" {
		return layoutContext{}, "INVALID_ROOT_LAYOUT"
	}
	marker := filepath.Join(root, homeMarker)
	_, markerErr := os.Lstat(marker)
	if kind == "project" {
		if markerErr == nil {
			return layoutContext{}, "ROOT_KIND_REQUIRED"
		}
		if !os.IsNotExist(markerErr) {
			return layoutContext{}, "INVALID_ROOT_LAYOUT"
		}
		return layoutContext{Kind: kind, Root: root, Local: filepath.Join(root, ".ocmi", "local"), State: filepath.Join(root, ".ocmi", "local"), Store: filepath.Join(root, ".ocmi", "local", "state.sqlite"), Runs: filepath.Join(root, ".ocmi", "local", "runs"), InitLock: filepath.Join(root, ".ocmi", "local", ".init-lock"), Paths: map[string]string{"store": ".ocmi/local/state.sqlite", "state": ".ocmi/local", "runs": ".ocmi/local/runs"}}, ""
	}
	if err := plainPath(root, true); err != nil {
		return layoutContext{}, "INVALID_ROOT_LAYOUT"
	}
	if markerErr != nil || plainPath(marker, false) != nil {
		return layoutContext{}, "INVALID_ROOT_LAYOUT"
	}
	if _, err := os.Lstat(filepath.Join(root, ".ocmi")); err == nil {
		return layoutContext{}, "INVALID_ROOT_LAYOUT"
	} else if !os.IsNotExist(err) {
		return layoutContext{}, "INVALID_ROOT_LAYOUT"
	}
	declaration, err := strictDeclaration(installationLayoutJSON)
	if err != nil {
		return layoutContext{}, "INVALID_ROOT_LAYOUT"
	}
	raw, err := os.ReadFile(marker)
	if err != nil {
		return layoutContext{}, "INVALID_ROOT_LAYOUT"
	}
	actual, err := strictDeclaration(raw)
	if err != nil || !sameLayout(actual, declaration) {
		return layoutContext{}, "INVALID_ROOT_LAYOUT"
	}
	for _, relative := range []string{declaration.Paths["local"], declaration.Paths["state"], declaration.Paths["runs"]} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if _, err := os.Lstat(path); err == nil && plainPath(path, true) != nil {
			return layoutContext{}, "INVALID_ROOT_LAYOUT"
		} else if err != nil && !os.IsNotExist(err) {
			return layoutContext{}, "INVALID_ROOT_LAYOUT"
		}
	}
	paths := make(map[string]string, len(declaration.Paths))
	for key, value := range declaration.Paths {
		paths[key] = value
	}
	physical := func(key string) string { return filepath.Join(root, filepath.FromSlash(declaration.Paths[key])) }
	return layoutContext{Kind: kind, Root: root, Local: physical("local"), State: physical("state"), Store: physical("store"), Runs: physical("runs"), InitLock: filepath.Join(physical("state"), ".init-lock"), Paths: paths}, ""
}

func selectedLayout(r Request) (layoutContext, string) {
	if r.layout.Root != "" {
		return r.layout, ""
	}
	return resolveLayout(r.ProjectRoot, r.RootKind)
}

func sameLayout(a, b layoutDeclaration) bool {
	if a.SchemaVersion != b.SchemaVersion || a.LayoutVersion != b.LayoutVersion || a.Kind != b.Kind || len(a.Paths) != len(b.Paths) {
		return false
	}
	for key, value := range a.Paths {
		if b.Paths[key] != value {
			return false
		}
	}
	return true
}

func layoutInspect(context layoutContext) Response {
	paths := make(map[string]string, len(context.Paths))
	for key, value := range context.Paths {
		paths[key] = value
	}
	return good("LAYOUT_INSPECTED", 0, map[string]any{"rootKind": context.Kind, "paths": paths})
}

func homeResourceAllowed(locator string) bool {
	locator = filepath.ToSlash(filepath.Clean(filepath.FromSlash(locator)))
	return strings.HasPrefix(locator, "OCMI_Sync/") && locator != "OCMI_Sync"
}
