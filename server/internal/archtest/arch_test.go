// Package archtest enforces the hexagonal layering: the rules (internal/game) and the use cases
// (internal/app) never import adapters, and never read the wall clock or math/rand.
package archtest

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func serverRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller info")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".") && first != "librehold"
}

func TestCoreImportsOnlyStdlibAndInward(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}}|{{join .Imports \" \"}}", "./internal/game/...", "./internal/app/...")
	cmd.Dir = serverRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	pkgs := 0
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		pkg, imports, _ := strings.Cut(line, "|")
		allowed := []string{"librehold/internal/game"}
		if strings.HasPrefix(pkg, "librehold/internal/app") {
			allowed = append(allowed, "librehold/internal/app")
		} else if !strings.HasPrefix(pkg, "librehold/internal/game") {
			t.Fatalf("unexpected package %q", pkg)
		}
		pkgs++
		for imp := range strings.FieldsSeq(imports) {
			if isStdlib(imp) {
				continue
			}
			ok := false
			for _, a := range allowed {
				if imp == a || strings.HasPrefix(imp, a+"/") {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s imports %s", pkg, imp)
			}
		}
	}
	if pkgs < 2 {
		t.Fatalf("expected at least the game and app packages, got %d", pkgs)
	}
}

func TestCoreNeverReadsWallClockOrMathRand(t *testing.T) {
	root := serverRoot(t)
	for _, dir := range []string{"internal/game", "internal/app"} {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, banned := range []string{"time.Now(", "\"math/rand"} {
				if strings.Contains(string(src), banned) {
					t.Errorf("%s contains %s", path, banned)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
