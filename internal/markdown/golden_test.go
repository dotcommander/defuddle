package markdown

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// update regenerates .out.md golden files instead of asserting equality.
// Usage: go test ./internal/markdown/... -update
var update = flag.Bool("update", false, "regenerate golden files under testdata/")

// TestGoldenRenderers walks internal/markdown/testdata/<tag>/*.in.html,
// converts each via ConvertHTML, and compares to the sibling <name>.out.md.
// When -update is set, it writes the actual output to <name>.out.md instead.
//
// Coverage invariant: one subdirectory per registered tag in markdown.go's
// defuddlePlugin.Init (22 registrations, 23 cases because sup covers two
// priorities).
func TestGoldenRenderers(t *testing.T) {
	t.Parallel()

	root := "testdata"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	for _, tagDir := range entries {
		if !tagDir.IsDir() {
			continue
		}
		tag := tagDir.Name()
		dir := filepath.Join(root, tag)

		inputs, err := filepath.Glob(filepath.Join(dir, "*.in.html"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		if len(inputs) == 0 {
			t.Errorf("no .in.html fixtures in %s", dir)
			continue
		}

		for _, inPath := range inputs {
			inPath := inPath
			name := tag + "/" + strings.TrimSuffix(filepath.Base(inPath), ".in.html")

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				htmlBytes, err := os.ReadFile(inPath)
				if err != nil {
					t.Fatalf("read %s: %v", inPath, err)
				}

				got, err := ConvertHTML(string(htmlBytes))
				if err != nil {
					t.Fatalf("ConvertHTML(%s): %v", inPath, err)
				}

				outPath := strings.TrimSuffix(inPath, ".in.html") + ".out.md"

				if *update {
					if err := os.WriteFile(outPath, []byte(got), 0o644); err != nil {
						t.Fatalf("write %s: %v", outPath, err)
					}
					return
				}

				wantBytes, err := os.ReadFile(outPath)
				if err != nil {
					t.Fatalf("read %s: %v (hint: run go test -update to generate)", outPath, err)
				}
				want := string(wantBytes)

				if got != want {
					t.Errorf("golden mismatch for %s\n--- want (%s) ---\n%s\n--- got ---\n%s\n--- end ---",
						name, outPath, want, got)
				}
			})
		}
	}
}
