//go:build darwin

package internal

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func unpackPublish(parent *os.Root, source, destination, parentID, sourceID string) error {
	if !unpackPublicationNames(source, destination) {
		return errors.New("publication names")
	}
	p, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer p.Close()
	id, err := localNativeIDFile(p)
	if err != nil || id != parentID {
		return errors.New("parent identity")
	}
	info, err := parent.Lstat(source)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("source type")
	}
	f, err := parent.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	id, err = localNativeIDFile(f)
	if err != nil || id != sourceID {
		return errors.New("source identity")
	}
	return unix.RenameatxNp(int(p.Fd()), source, int(p.Fd()), destination, unix.RENAME_EXCL|unix.RENAME_NOFOLLOW_ANY)
}
