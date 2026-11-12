package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func updateWithin(parent, child string) bool {
	if runtime.GOOS == "windows" {
		parent, child = strings.ToLower(parent), strings.ToLower(child)
	}
	rel, err := filepath.Rel(parent, child)
	return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

func updateExternalCaller(home string) error {
	if !filepath.IsAbs(home) || environmentPlainAncestors(home) != nil {
		return errors.New("unsafe home")
	}
	exe, err := os.Executable()
	if err != nil {
		return errors.New("caller unavailable")
	}
	exe, err = filepath.Abs(exe)
	if err != nil || environmentPlainAncestors(exe) != nil || updateWithin(filepath.Clean(home), filepath.Clean(exe)) {
		return errors.New("caller not external")
	}
	installed := filepath.Join(home, "system", "bin", "ocmi")
	if runtime.GOOS == "windows" {
		installed += ".exe"
	}
	if targetID, err := localNativeID(installed); err == nil {
		callerID, err := localNativeID(exe)
		if err != nil || callerID == targetID {
			return errors.New("caller not external")
		}
	} else if !os.IsNotExist(err) {
		return errors.New("installed identity")
	}
	return nil
}

func updateReleaseVersion(root string) ([3]uint64, error) {
	var out [3]uint64
	raw, err := installationRead(root, "system/build-info.json", 1<<20)
	if err != nil {
		return out, err
	}
	var info struct {
		SystemVersion string `json:"systemVersion"`
	}
	if json.Unmarshal(raw, &info) != nil {
		return out, errors.New("release version")
	}
	parts := strings.Split(info.SystemVersion, ".")
	if len(parts) != 3 {
		return out, errors.New("release version")
	}
	for i, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return out, errors.New("release version")
		}
		if strings.Trim(part, "0123456789") != "" {
			return out, errors.New("release version")
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return out, errors.New("release version")
		}
		out[i] = n
	}
	return out, nil
}

func preflightUpdateCandidate(home, candidate, expectedSource, expectedCandidate string) (layoutContext, factoryInfo, layoutContext, factoryInfo, error) {
	var empty layoutContext
	var noInfo factoryInfo
	if !filepath.IsAbs(home) || !filepath.IsAbs(candidate) || environmentPlainAncestors(home) != nil || environmentPlainAncestors(candidate) != nil {
		return empty, noInfo, empty, noInfo, errors.New("unsafe paths")
	}
	home = filepath.Clean(home)
	candidate = filepath.Clean(candidate)
	if updateWithin(home, candidate) || updateWithin(candidate, home) {
		return empty, noInfo, empty, noInfo, errors.New("overlapping paths")
	}
	oldCtx, oldInfo, err := verifyFactoryHome(home)
	if err != nil {
		return empty, noInfo, empty, noInfo, errors.New("home payload")
	}
	if _, err := currentInstallation(oldCtx, oldInfo); err != nil {
		return empty, noInfo, empty, noInfo, errors.New("installation context")
	}
	newCtx, newInfo, err := updateCandidate(candidate)
	if err != nil {
		return empty, noInfo, empty, noInfo, errors.New("candidate payload")
	}
	if err := toolStorageReaderGuard(oldCtx, newCtx.Root); err != nil {
		return empty, noInfo, empty, noInfo, err
	}
	if !validCommit(expectedSource) || !validCommit(expectedCandidate) || oldInfo.SourceCommit != expectedSource || newInfo.SourceCommit != expectedCandidate {
		return empty, noInfo, empty, noInfo, errors.New("source mismatch")
	}
	for _, name := range []string{"local", "OCMI_Sync"} {
		if _, err := os.Lstat(filepath.Join(candidate, name)); err == nil || !os.IsNotExist(err) {
			return empty, noInfo, empty, noInfo, errors.New("initialized candidate")
		}
	}
	if err := updateExternalCaller(home); err != nil {
		return empty, noInfo, empty, noInfo, err
	}
	exe, err := os.Executable()
	if err != nil {
		return empty, noInfo, empty, noInfo, errors.New("caller unavailable")
	}
	entry := filepath.Join(candidate, "system", "bin", "ocmi")
	if runtime.GOOS == "windows" {
		entry += ".exe"
	}
	actualID, err := localNativeID(exe)
	if err != nil {
		return empty, noInfo, empty, noInfo, errors.New("caller identity")
	}
	candidateID, err := localNativeID(entry)
	if err != nil || actualID != candidateID {
		return empty, noInfo, empty, noInfo, errors.New("caller identity")
	}
	oldVersion, err := updateReleaseVersion(home)
	if err != nil {
		return empty, noInfo, empty, noInfo, err
	}
	newVersion, err := updateReleaseVersion(candidate)
	if err != nil {
		return empty, noInfo, empty, noInfo, err
	}
	for i := range oldVersion {
		if newVersion[i] < oldVersion[i] {
			return empty, noInfo, empty, noInfo, errors.New("release downgrade")
		}
		if newVersion[i] > oldVersion[i] {
			break
		}
	}
	installed := filepath.Join(home, "system", "bin", "ocmi")
	if runtime.GOOS == "windows" {
		installed += ".exe"
	}
	if err := probeInstalledImage(installed); err != nil {
		return empty, noInfo, empty, noInfo, errors.New("installed image busy")
	}
	return oldCtx, oldInfo, newCtx, newInfo, nil
}
