package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestParseProfileLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey profileKey
		wantN   int
		wantErr bool
	}{
		{
			name: "count mode entry",
			line: "pkg/file.go:10.5,12.2 3 14",
			wantKey: profileKey{
				file: "pkg/file.go", startLn: 10, startCn: 5,
				endLn: 12, endCn: 2, numStmt: 3,
			},
			wantN: 14,
		},
		{
			name: "set mode entry",
			line: "pkg/file.go:1.1,2.2 1 1",
			wantKey: profileKey{
				file: "pkg/file.go", startLn: 1, startCn: 1,
				endLn: 2, endCn: 2, numStmt: 1,
			},
			wantN: 1,
		},
		{name: "no space", line: "pkg/file.go:1.1,2.21", wantErr: true},
		{name: "missing numStmt", line: "pkg/file.go:1.1,2.2 14", wantErr: true},
		{name: "bad count", line: "pkg/file.go:1.1,2.2 1 x", wantErr: true},
		{name: "bad numStmt", line: "pkg/file.go:1.1,2.2 x 1", wantErr: true},
		{name: "no comma", line: "pkg/file.go:1.1 1 1", wantErr: true},
		{name: "no colon", line: "pkg/file.go1.1,2.2 1 1", wantErr: true},
		{name: "no dot in start", line: "pkg/file.go:1,2.2 1 1", wantErr: true},
		{name: "no dot in end", line: "pkg/file.go:1.1,2 1 1", wantErr: true},
		{name: "bad line number", line: "pkg/file.go:x.1,2.2 1 1", wantErr: true},
		{name: "bad column", line: "pkg/file.go:1.x,2.2 1 1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, n, err := parseProfileLine(tt.line)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got key=%v n=%d", key, n)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key != tt.wantKey || n != tt.wantN {
				t.Errorf("got key=%v n=%d, want key=%v n=%d", key, n, tt.wantKey, tt.wantN)
			}
		})
	}
}

func TestReadProfileRejectsBadHeaders(t *testing.T) {
	dir := t.TempDir()

	noHeader := writeFile(t, dir, "noheader.out", "pkg/file.go:1.1,2.2 1 1\n")
	if _, err := readProfile(noHeader); err == nil || !strings.Contains(err.Error(), "missing mode") {
		t.Errorf("missing header error = %v", err)
	}

	badMode := writeFile(t, dir, "badmode.out", "mode: atomic\npkg/file.go:1.1,2.2 1 1\n")
	if _, err := readProfile(badMode); err == nil || !strings.Contains(err.Error(), "unsupported mode") {
		t.Errorf("bad mode error = %v", err)
	}

	missing := filepath.Join(dir, "does-not-exist.out")
	if _, err := readProfile(missing); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadProfileParsesCountAndSet(t *testing.T) {
	dir := t.TempDir()

	countProfile := writeFile(t, dir, "count.out",
		"mode: count\npkg/a.go:1.1,2.2 2 3\npkg/b.go:5.1,6.2 1 0\n")
	count, err := readProfile(countProfile)
	if err != nil {
		t.Fatalf("readProfile count: %v", err)
	}
	if len(count) != 2 {
		t.Fatalf("count entries = %d, want 2", len(count))
	}
	setProfile := writeFile(t, dir, "set.out",
		"mode: set\npkg/a.go:1.1,2.2 2 1\n")
	set, err := readProfile(setProfile)
	if err != nil {
		t.Fatalf("readProfile set: %v", err)
	}
	if len(set) != 1 || set[profileKey{file: "pkg/a.go", startLn: 1, startCn: 1, endLn: 2, endCn: 2, numStmt: 2}] != 1 {
		t.Fatalf("set entries = %v", set)
	}
}

func TestMergeCmdMaxSemantics(t *testing.T) {
	dir := t.TempDir()
	base := writeFile(t, dir, "base.out",
		"mode: count\npkg/a.go:1.1,3.2 2 5\npkg/a.go:4.1,5.2 1 0\n")
	extra := writeFile(t, dir, "extra.out",
		"mode: set\npkg/a.go:1.1,3.2 2 1\npkg/a.go:4.1,5.2 1 1\npkg/b.go:1.1,2.2 1 1\n")
	out := filepath.Join(dir, "merged.out")

	if err := mergeCmd([]string{base, extra, out}); err != nil {
		t.Fatalf("mergeCmd: %v", err)
	}
	merged, err := readProfile(out)
	if err != nil {
		t.Fatalf("readProfile merged: %v", err)
	}
	a1 := profileKey{file: "pkg/a.go", startLn: 1, startCn: 1, endLn: 3, endCn: 2, numStmt: 2}
	a2 := profileKey{file: "pkg/a.go", startLn: 4, startCn: 1, endLn: 5, endCn: 2, numStmt: 1}
	b1 := profileKey{file: "pkg/b.go", startLn: 1, startCn: 1, endLn: 2, endCn: 2, numStmt: 1}
	if merged[a1] != 5 {
		t.Errorf("a1 = %d, want max(5,1)=5", merged[a1])
	}
	if merged[a2] != 1 {
		t.Errorf("a2 = %d, want max(0,1)=1", merged[a2])
	}
	if merged[b1] != 1 {
		t.Errorf("b1 = %d, want 1", merged[b1])
	}
}

func TestMergeCmdArgValidation(t *testing.T) {
	if err := mergeCmd(nil); err == nil {
		t.Error("expected error for no args")
	}
	if err := mergeCmd([]string{"a", "b"}); err == nil {
		t.Error("expected error for two args")
	}
	dir := t.TempDir()
	base := writeFile(t, dir, "base.out", "mode: count\npkg/a.go:1.1,2.2 1 1\n")
	if err := mergeCmd([]string{filepath.Join(dir, "missing.out"), base, filepath.Join(dir, "o.out")}); err == nil {
		t.Error("expected error for missing base file")
	}
	if err := mergeCmd([]string{base, filepath.Join(dir, "missing.out"), filepath.Join(dir, "o.out")}); err == nil {
		t.Error("expected error for missing extra file")
	}
}

func TestWriteProfileSortedOutput(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "p.out")
	profile := map[profileKey]int{
		{file: "pkg/b.go", startLn: 1, startCn: 1, endLn: 2, endCn: 2, numStmt: 1}: 3,
		{file: "pkg/a.go", startLn: 5, startCn: 1, endLn: 6, endCn: 2, numStmt: 2}: 4,
		{file: "pkg/a.go", startLn: 1, startCn: 1, endLn: 2, endCn: 2, numStmt: 1}: 1,
	}
	if err := writeProfile(out, profile); err != nil {
		t.Fatalf("writeProfile: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "mode: count\n") {
		t.Fatalf("header = %q", text)
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4", len(lines))
	}
	if !strings.Contains(lines[1], "pkg/a.go:1.1") || !strings.Contains(lines[2], "pkg/a.go:5.1") || !strings.Contains(lines[3], "pkg/b.go:1.1") {
		t.Errorf("unsorted output:\n%s", text)
	}
	if err := writeProfile(filepath.Join(dir, "bad", "x.out"), profile); err == nil {
		t.Error("expected error writing into missing directory")
	}
}

func TestFmtCmd(t *testing.T) {
	dir := t.TempDir()
	formatted := "package p\n\nfunc F() {}\n"
	unformatted := "package p\nfunc F() {}\n"
	writeFile(t, dir, "ok.go", formatted)
	if err := fmtCmd([]string{dir}); err != nil {
		t.Errorf("fmtCmd on formatted dir: %v", err)
	}
	writeFile(t, dir, "bad.go", unformatted)
	if err := fmtCmd([]string{dir}); err == nil {
		t.Error("expected fmtCmd error for unformatted file")
	}
	if err := fmtCmd(nil); err != nil {
		t.Errorf("fmtCmd default path: %v", err)
	}
}

func TestCleanCmd(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "artifact.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "bin")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cleanCmd([]string{target, nested}); err != nil {
		t.Fatalf("cleanCmd: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("artifact still exists")
	}
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Error("bin dir still exists")
	}
}
