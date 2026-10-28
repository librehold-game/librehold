package game

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

// The files in testdata/golden are snapshots of the rules. After a deliberate rule change, re-record them with
// `go test ./internal/game/ -update` and review the diff.
var update = flag.Bool("update", false, "re-record testdata/golden from this build")

// Updating reports whether the run re-records the golden files.
func Updating() bool { return *update }

// Golden compares got, marshalled to JSON, with testdata/golden/<name>.json; with -update it writes the file.
func Golden(t testing.TB, name string, got any) {
	t.Helper()
	goldenAt(t, filepath.Join("testdata", "golden", name+".json"), got)
}

func goldenAt(t testing.TB, path string, got any) {
	t.Helper()
	out, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out = append(out, '\n')
	if *update {
		if err := os.WriteFile(path, out, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (record it with -update)", path, err)
	}
	var g, want any
	if err := json.Unmarshal(out, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	// A missing unit key and a zero one read the same everywhere.
	if at := firstDiff(dropZeroPartialUnits(g), dropZeroPartialUnits(want), "$"); at != "" {
		t.Errorf("%s differs from this build at %s; after a deliberate change re-record with -update and review the diff", path, at)
	}
}

// firstDiff is the path of the first value where got and want differ by jsonNear, or "".
func firstDiff(got, want any, path string) string {
	switch x := got.(type) {
	case map[string]any:
		y, ok := want.(map[string]any)
		if !ok {
			return path
		}
		for _, k := range slices.Sorted(maps.Keys(x)) {
			if _, ok := y[k]; !ok {
				return path + "." + k + " (only in this build)"
			}
			if at := firstDiff(x[k], y[k], path+"."+k); at != "" {
				return at
			}
		}
		for k := range y {
			if _, ok := x[k]; !ok {
				return path + "." + k + " (only in the file)"
			}
		}
		return ""
	case []any:
		y, ok := want.([]any)
		if !ok || len(x) != len(y) {
			return path + " (length)"
		}
		for i := range x {
			if at := firstDiff(x[i], y[i], path+"["+strconv.Itoa(i)+"]"); at != "" {
				return at
			}
		}
		return ""
	}
	if !jsonNear(got, want) {
		return fmt.Sprintf("%s: %v, file has %v", path, got, want)
	}
	return ""
}

type failRecorder struct {
	testing.TB
	failed bool
}

func (f *failRecorder) Errorf(string, ...any) { f.failed = true }
func (f *failRecorder) Fatalf(string, ...any) { f.failed = true }

func TestGoldenHelperRejectsDrift(t *testing.T) {
	if *update {
		t.Skip("recording")
	}
	path := filepath.Join(t.TempDir(), "g.json")
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	drift := &failRecorder{TB: t}
	goldenAt(drift, path, map[string]int{"a": 2})
	if !drift.failed {
		t.Error("a changed value passed")
	}
	same := &failRecorder{TB: t}
	goldenAt(same, path, map[string]int{"a": 1})
	if same.failed {
		t.Error("an equal value failed")
	}
}
