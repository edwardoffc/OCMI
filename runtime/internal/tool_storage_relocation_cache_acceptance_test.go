package internal

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Native control with no relocation: distinguish consumer cache writes from
// a preserving transition's own effects. No profile/environment suppression.
func TestMainToolAuthoringRelocationOrdinaryCachePremise(t *testing.T) {
	f := mainRelocationSetup(t)
	var before map[string]string
	for i := 0; i < 3; i++ {
		root, ready := mainRelocationProject(t, f, fmt.Sprintf("cache-control-%d", i), f.packageDir)
		mainRelocationRun(t, root, ready, f.packageDir)
		after := mainRelocationSnapshot(t, f.data)
		if before != nil {
			left, right := map[string]string{}, map[string]string{}
			for key, value := range before {
				if !strings.HasPrefix(key, "go/build-cache/") {
					left[key] = value
				}
			}
			for key, value := range after {
				if !strings.HasPrefix(key, "go/build-cache/") {
					right[key] = value
				}
				if before[key] != value {
					t.Logf("ordinary Go changed/created %s", key)
				}
			}
			if !reflect.DeepEqual(left, right) {
				t.Fatal("ordinary control changed durable/offline fixture data")
			}
		}
		t.Logf("ordinary Go consumer %d observed %d source files", i+1, len(after))
		before = after
	}
}
