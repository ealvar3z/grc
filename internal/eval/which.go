package eval

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func resolvePath(name string, env *Env, verbose bool, stderr io.Writer) (string, bool) {
	if name == "" {
		return "", false
	}
	if strings.ContainsRune(name, '/') {
		if rcAccess(name) {
			return name, true
		}
		if verbose && stderr != nil {
			fmt.Fprintf(stderr, "rc: cannot find `%s`\n", name)
		}
		return "", false
	}
	for _, dir := range pathList(env) {
		if dir == "" {
			dir = "."
		}
		full := filepath.Join(dir, name)
		if rcAccess(full) {
			return full, true
		}
	}
	if verbose && stderr != nil {
		fmt.Fprintf(stderr, "rc: cannot find `%s`\n", name)
	}
	return "", false
}

func pathList(env *Env) []string {
	if env != nil {
		if vals := env.Get("path"); len(vals) > 0 {
			return vals
		}
	}
	if p := os.Getenv("PATH"); p != "" {
		return strings.Split(p, string(os.PathListSeparator))
	}
	return []string{""}
}

func rcAccess(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !info.Mode().IsRegular() {
		return false
	}
	mode := info.Mode().Perm()
	uid := os.Geteuid()
	gid := os.Getegid()
	if uid == 0 {
		return mode&0o111 != 0
	}
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return false
	}
	switch {
	case int(st.Uid) == uid:
		return mode&0o100 != 0
	case int(st.Gid) == gid || inGroup(int(st.Gid)):
		return mode&0o010 != 0
	default:
		return mode&0o001 != 0
	}
}

func inGroup(gid int) bool {
	groups, err := os.Getgroups()
	if err != nil {
		return false
	}
	for _, g := range groups {
		if g == gid {
			return true
		}
	}
	return false
}
