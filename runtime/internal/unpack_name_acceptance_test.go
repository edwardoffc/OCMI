package internal

import (
	"strings"
	"testing"
)

// Distinct file names may still alias their parent directory on another platform.
// Reject that ambiguity before a writer is called, while retaining case-sensitive
// spelling within a consistently named directory.
func TestMainUnpackDirectorySpelling(t *testing.T) {
	for _, control := range []struct {
		first, second string
		refuse        bool
	}{
		{"system/selected/a", "system/selected/b", false},
		{"system/selected/a", "system/SELECTED/b", true},
		{"system/SELECTED/a", "system/selected/b", true},
		{"system/Selected/a", "system/Selected/b", false},
		{"system/selected-one/a", "system/selected-two/b", false},
	} {
		t.Run(control.first+"_"+control.second, func(t *testing.T) {
			collector := unpackCollector{names: map[string]bool{}, metadata: map[string][]byte{}}
			if err := collector.file(control.first, 1, 0644, strings.NewReader("a")); err != nil {
				t.Fatal("valid first member refused", err)
			}
			err := collector.file(control.second, 1, 0644, strings.NewReader("b"))
			if (err != nil) != control.refuse {
				t.Fatalf("directory spelling ambiguity: refuse=%v, err=%v", control.refuse, err)
			}
		})
	}
}
