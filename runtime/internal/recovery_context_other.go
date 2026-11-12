//go:build !windows && !linux && !darwin

package internal

import "errors"

func recoveryContext() (string, error) { return "", errors.New("native context unsupported") }
