// Command gen writes the files generated from the Go rules (see package gen).
//
//	go run ./cmd/gen            # from server/, or `npm run gen` from the repository root
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"librehold/internal/gen"
)

func main() {
	root := flag.String("root", "", "repository root (default: the directory holding package.json)")
	flag.Parse()
	if *root == "" {
		r, err := gen.RepoRoot()
		if err != nil {
			fail(err)
		}
		*root = r
	}
	for _, f := range gen.Files() {
		b, err := f.Bytes()
		if err != nil {
			fail(fmt.Errorf("%s: %w", f.Path, err))
		}
		path := filepath.Join(*root, f.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fail(err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			fail(err)
		}
		fmt.Println(f.Path)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
