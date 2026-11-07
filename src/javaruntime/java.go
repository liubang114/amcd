// Package javaruntime 定位可用的 Java 运行时 / locate a usable Java runtime
package javaruntime

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var majorRe = regexp.MustCompile(`version "(\d+)(?:\.(\d+))?`)

// Candidates 返回常见安装位置 / common install locations
func candidates() []string {
	var out []string
	if home := os.Getenv("JAVA_HOME"); home != "" {
		out = append(out, filepath.Join(home, "bin", "java.exe"))
	}
	if p, err := exec.LookPath("java"); err == nil {
		out = append(out, p)
	}
	globs := []string{
		`C:\Program Files\Java\*\bin\java.exe`,
		`C:\Program Files (x86)\Java\*\bin\java.exe`,
		`C:\Program Files\Eclipse Adoptium\*\bin\java.exe`,
		`C:\Program Files\Microsoft\jdk-*\bin\java.exe`,
		`C:\Program Files\Zulu\*\bin\java.exe`,
		`C:\Program Files\BellSoft\*\bin\java.exe`,
	}
	for _, g := range globs {
		matches, _ := filepath.Glob(g)
		out = append(out, matches...)
	}
	return out
}

// Major 读取 java 主版本号 / major version of a java executable
func Major(javaExe string) int {
	out, err := exec.Command(javaExe, "-version").CombinedOutput()
	if err != nil && len(out) == 0 {
		return 0
	}
	m := majorRe.FindStringSubmatch(string(out))
	if len(m) < 2 {
		return 0
	}
	major, _ := strconv.Atoi(m[1])
	if major == 1 && len(m) > 2 && m[2] != "" {
		major, _ = strconv.Atoi(m[2]) // 1.8 → 8 / legacy versioning
	}
	return major
}

// Required 按 MC 版本判断所需 Java 主版本 / required Java major for an MC version
func Required(mcVersion string) int {
	switch {
	case strings.HasPrefix(mcVersion, "1.20.5"), strings.HasPrefix(mcVersion, "1.20.6"),
		strings.HasPrefix(mcVersion, "1.21"), strings.HasPrefix(mcVersion, "1.22"),
		mcVersion >= "26.":
		return 21
	case strings.HasPrefix(mcVersion, "1.18"), strings.HasPrefix(mcVersion, "1.19"), strings.HasPrefix(mcVersion, "1.20"):
		return 17
	case strings.HasPrefix(mcVersion, "1.17"):
		return 16
	}
	return 8
}

// Find 返回满足最低版本的 java 路径 / pick the lowest java that satisfies required
func Find(required int) (string, int) {
	type cand struct {
		path  string
		major int
	}
	seen := map[string]bool{}
	var list []cand
	for _, p := range candidates() {
		if seen[strings.ToLower(p)] {
			continue
		}
		seen[strings.ToLower(p)] = true
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if m := Major(p); m > 0 {
			list = append(list, cand{p, m})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].major < list[j].major })
	for _, c := range list {
		if c.major >= required {
			return c.path, c.major
		}
	}
	if len(list) > 0 {
		last := list[len(list)-1]
		return last.path, last.major
	}
	return "", 0
}
