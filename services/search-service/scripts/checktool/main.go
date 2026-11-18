package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "merge":
		err = mergeCmd(os.Args[2:])
	case "fmt":
		err = fmtCmd(os.Args[2:])
	case "clean":
		err = cleanCmd(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "checktool:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: checktool merge <base.out> <extra.out> <out.out>")
	fmt.Fprintln(os.Stderr, "       checktool fmt [path ...]")
	fmt.Fprintln(os.Stderr, "       checktool clean [file ...]")
}

func mergeCmd(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("merge requires exactly 3 arguments: base, extra, out")
	}
	base, err := readProfile(args[0])
	if err != nil {
		return err
	}
	extra, err := readProfile(args[1])
	if err != nil {
		return err
	}
	for k, v := range extra {
		if cur, ok := base[k]; !ok || v > cur {
			base[k] = v
		}
	}
	return writeProfile(args[2], base)
}

type profileKey struct {
	file    string
	startLn int
	startCn int
	endLn   int
	endCn   int
	numStmt int
}

func readProfile(path string) (map[profileKey]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	profile := make(map[profileKey]int)
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if lineNo == 1 {
			if !strings.HasPrefix(line, "mode:") {
				return nil, fmt.Errorf("%s: missing mode: header", path)
			}
			if mode := strings.TrimSpace(strings.TrimPrefix(line, "mode:")); mode != "count" && mode != "set" {
				return nil, fmt.Errorf("%s: unsupported mode %q (want count or set)", path, mode)
			}
			continue
		}
		if line == "" {
			continue
		}
		key, count, err := parseProfileLine(line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
		profile[key] = count
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return profile, nil
}

func parseProfileLine(line string) (profileKey, int, error) {
	// format: file:startLine.startCol,endLine.endCol numStmt count
	last := strings.LastIndex(line, " ")
	if last < 0 {
		return profileKey{}, 0, fmt.Errorf("malformed profile line %q", line)
	}
	count, err := strconv.Atoi(line[last+1:])
	if err != nil {
		return profileKey{}, 0, fmt.Errorf("bad count in %q", line)
	}
	rest := line[:last]
	prev := strings.LastIndex(rest, " ")
	if prev < 0 {
		return profileKey{}, 0, fmt.Errorf("malformed profile line %q", line)
	}
	numStmt, err := strconv.Atoi(rest[prev+1:])
	if err != nil {
		return profileKey{}, 0, fmt.Errorf("bad numStmt in %q", line)
	}
	loc := rest[:prev]
	comma := strings.Index(loc, ",")
	if comma < 0 {
		return profileKey{}, 0, fmt.Errorf("malformed locations %q", loc)
	}
	start, end := loc[:comma], loc[comma+1:]
	colon := strings.LastIndex(start, ":")
	if colon < 0 {
		return profileKey{}, 0, fmt.Errorf("malformed start position %q", start)
	}
	file := start[:colon]
	sl, scol, err := parsePos(start[colon+1:])
	if err != nil {
		return profileKey{}, 0, err
	}
	el, ecol, err := parsePos(end)
	if err != nil {
		return profileKey{}, 0, err
	}
	return profileKey{file, sl, scol, el, ecol, numStmt}, count, nil
}

func parsePos(pos string) (line, col int, err error) {
	dot := strings.LastIndex(pos, ".")
	if dot < 0 {
		return 0, 0, fmt.Errorf("malformed position %q", pos)
	}
	if line, err = strconv.Atoi(pos[:dot]); err != nil {
		return 0, 0, fmt.Errorf("bad line in position %q", pos)
	}
	if col, err = strconv.Atoi(pos[dot+1:]); err != nil {
		return 0, 0, fmt.Errorf("bad column in position %q", pos)
	}
	return line, col, nil
}

func writeProfile(path string, profile map[profileKey]int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, "mode: count"); err != nil {
		return err
	}
	keys := make([]profileKey, 0, len(profile))
	for k := range profile {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.file != b.file {
			return a.file < b.file
		}
		if a.startLn != b.startLn {
			return a.startLn < b.startLn
		}
		return a.startCn < b.startCn
	})
	for _, k := range keys {
		if _, err := fmt.Fprintf(f, "%s:%d.%d,%d.%d %d %d\n",
			k.file, k.startLn, k.startCn, k.endLn, k.endCn, k.numStmt, profile[k]); err != nil {
			return err
		}
	}
	return nil
}

func fmtCmd(args []string) error {
	if len(args) == 0 {
		args = []string{"."}
	}
	cmd := exec.Command("gofmt", append([]string{"-l"}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("gofmt failed: %w", err)
	}
	files := strings.TrimSpace(string(out))
	if files != "" {
		return fmt.Errorf("files need gofmt:\n%s", files)
	}
	return nil
}

func cleanCmd(args []string) error {
	if len(args) == 0 {
		args = []string{
			"bin",
			"coverprofile.out",
			"subprocess.out",
			"merged.out",
			"coverage.html",
		}
	}
	for _, p := range args {
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(abs); err != nil {
			return fmt.Errorf("remove %s: %w", p, err)
		}
		fmt.Println("removed", p)
	}
	return nil
}
