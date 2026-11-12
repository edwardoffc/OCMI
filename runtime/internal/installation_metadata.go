package internal

import (
	"errors"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// installationRead is a bounded, no-link read of metadata beneath the selected home.
func installationRead(root, relative string, limit int) ([]byte, error) {
	if !installationMetadataName(relative) {
		return nil, errors.New("metadata path")
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 || info.Size() > int64(limit) || plainComponents(root, filepath.FromSlash(relative), false) != nil {
		return nil, errors.New("metadata ancestors")
	}
	before, err := localNativeID(path)
	if err != nil {
		return nil, err
	}
	opened, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer opened.Close()
	f, err := opened.Open(filepath.FromSlash(relative))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	openedID, err := localNativeIDFile(f)
	if err != nil || openedID != before {
		return nil, errors.New("metadata opened identity")
	}
	raw, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(raw) > limit {
		return nil, errors.New("metadata read")
	}
	after, err := localNativeID(path)
	if err != nil || after != before {
		return nil, errors.New("metadata changed")
	}
	return raw, nil
}

func installationWrite(root, relative string, raw []byte) error {
	if !installationMetadataName(relative) {
		return errors.New("metadata path")
	}
	rootID, err := localNativeID(root)
	if err != nil {
		return err
	}
	held, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer held.Close()
	if err = installationEnsureParent(held, filepath.ToSlash(filepath.Dir(relative))); err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	if info, err := os.Lstat(path); err == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return errors.New("metadata leaf")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	temp := relative + ".pending-" + uuid.NewString()
	f, err := held.OpenFile(filepath.FromSlash(temp), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if id, err := localNativeID(root); err != nil || id != rootID {
		return errors.New("metadata root changed")
	}
	return held.Rename(filepath.FromSlash(temp), filepath.FromSlash(relative))
}

func installationMetadataName(relative string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relative)))
	return relative != "" && clean == relative && !filepath.IsAbs(filepath.FromSlash(relative)) && !strings.HasPrefix(clean, "../") && clean != "."
}
func installationEnsureParent(held *os.Root, parent string) error {
	if parent == "." {
		return nil
	}
	current := ""
	for _, part := range strings.Split(parent, "/") {
		if current == "" {
			current = part
		} else {
			current = current + "/" + part
		}
		info, err := held.Lstat(current)
		if err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return errors.New("metadata parent")
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err = held.Mkdir(current, 0700); err != nil {
			return err
		}
	}
	return nil
}

func installationAdmission(context layoutContext) (func(), error) {
	if environmentPlainAncestors(context.Root) != nil {
		return nil, errors.New("unsafe home")
	}
	return setupLock(filepath.Join(context.Root, filepath.FromSlash(context.Paths["bindings"])))
}
