//go:build !windows && !linux && !darwin

package internal

import (
	"errors"
	"os"
)

func unpackPublish(*os.Root, string, string, string, string) error {
	return errors.New("unsupported publication platform")
}
