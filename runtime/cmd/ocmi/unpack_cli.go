package main

import (
	"path/filepath"
	"strings"

	"github.com/edwardoffc/OCMI/runtime/internal"
)

func unpackUsage() internal.Response {
	return internal.Response{SchemaVersion: 1, Code: "USAGE", Data: map[string]any{
		"reason": "invalid_unpack_options", "stage": "cli", "nextAction": "review unpack command options",
		"usage": "unpack --archive ABS --destination ABS --expected-sha256 HEX64 | unpack --destination ABS --inspect",
	}}
}

func unpackFlags(args []string) (archive, destination, digest string, inspect, ok bool) {
	seen := map[string]bool{}
	for len(args) > 0 {
		key := args[0]
		if seen[key] {
			return
		}
		seen[key] = true
		args = args[1:]
		if key == "--inspect" {
			inspect = true
			continue
		}
		if len(args) == 0 || args[0] == "" {
			return
		}
		value := args[0]
		args = args[1:]
		switch key {
		case "--archive":
			archive = value
		case "--destination":
			destination = value
		case "--expected-sha256":
			digest = value
		default:
			return
		}
	}
	if !filepath.IsAbs(destination) || filepath.Clean(destination) != destination || filepath.Dir(destination) == destination {
		return
	}
	if inspect {
		ok = archive == "" && digest == ""
		return
	}
	ok = filepath.IsAbs(archive) && filepath.Clean(archive) == archive && len(digest) == 64 && strings.Trim(digest, "0123456789abcdef") == ""
	return
}
