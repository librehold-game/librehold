// Package gen produces the files the browser and its tests read from the Go rules: the balance tables, the
// expected answers of the display helpers the browser keeps, and the UI tests' world fixtures. cmd/gen writes
// them; TestGeneratedFilesAreCurrent fails while a committed copy is stale.
package gen

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// File is one generated file, Path relative to the repository root.
type File struct {
	Path  string
	Bytes func() ([]byte, error)
}

// Files lists every generated file.
func Files() []File {
	files := []File{
		{"src/data/balance.json", BalanceJSON},
		{"tests/fixtures/rules.json", RulesJSON},
	}
	for _, kind := range fixtureKinds {
		files = append(files, File{"tests/fixtures/world-" + kind + ".json", func() ([]byte, error) { return WorldFixture(kind) }})
	}
	return files
}

// RepoRoot walks up from the working directory to the one holding package.json.
func RepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no package.json above the working directory")
		}
		dir = parent
	}
}

// canonical is v as indented JSON with sorted keys and a trailing newline.
func canonical(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var plain any
	if err := json.Unmarshal(raw, &plain); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(plain, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
