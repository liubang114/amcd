package javaruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Java 运行时清单：Mojang 官方 + BMCLAPI 镜像（与 PCL2 相同来源）
// Java runtime index from Mojang, mirrored by BMCLAPI — same source PCL2 uses
const (
	javaIndexPath = "/v1/products/java-runtime/2ec0cc96c44e5a76b9c8b7c39df7210883d12871/all.json"
	javaPlatform  = "windows-x64"
)

var javaIndexURLs = []string{
	"https://bmclapi2.bangbang93.com" + javaIndexPath,
	"https://piston-meta.mojang.com" + javaIndexPath,
}

// Component 一套可下载的 Java 运行时 / a downloadable runtime component
type Component struct {
	Name      string
	Version   int
	Manifest  string
	NiceName  string
}

type indexEntry struct {
	Manifest struct {
		URL string `json:"url"`
	} `json:"manifest"`
	Version struct {
		Name string `json:"name"`
	} `json:"version"`
}

func getJSON(ctx context.Context, urls []string, out any) error {
	client := &http.Client{Timeout: 60 * time.Second}
	var lastErr error
	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "amcd/0.1 (AL-1S Minecraft Downloader)")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d %s", resp.StatusCode, u)
			continue
		}
		if err := json.Unmarshal(data, out); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

// Components 列出 Mojang 提供的 Java 运行时 / list Mojang runtime components
func Components(ctx context.Context) ([]Component, error) {
	var raw map[string]map[string][]indexEntry
	if err := getJSON(ctx, javaIndexURLs, &raw); err != nil {
		return nil, err
	}
	entries, ok := raw[javaPlatform]
	if !ok {
		return nil, fmt.Errorf("清单里没有 %s 平台", javaPlatform)
	}

	type named struct {
		key string
		val indexEntry
	}
	var sorted []named
	for k, v := range entries {
		if k == "minecraft-java-exe" || len(v) == 0 {
			continue
		}
		sorted = append(sorted, named{k, v[0]})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].key < sorted[j].key })

	var out []Component
	for _, e := range sorted {
		name := e.val.Version.Name
		major := 0
		if i := strings.Index(name, "u"); i > 0 && !strings.Contains(name[:i], ".") && i <= 2 {
			major, _ = strconv.Atoi(name[:i])
		} else if i := strings.Index(name, "."); i > 0 {
			major, _ = strconv.Atoi(name[:i])
		}
		out = append(out, Component{
			Name:     e.key,
			Version:  major,
			Manifest: e.val.Manifest.URL,
			NiceName: name,
		})
	}
	return out, nil
}

// Pick 选择满足要求的最低版本运行时 / pick the lowest component satisfying version
func Pick(list []Component, required int) (Component, bool) {
	var candidates []Component
	for _, c := range list {
		if c.Version >= required {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		return Component{}, false
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Version < candidates[j].Version })
	return candidates[0], true
}

// FindByName 按组件名查找 / find a component by name
func FindByName(list []Component, name string) (Component, bool) {
	for _, c := range list {
		if c.Name == name {
			return c, true
		}
	}
	return Component{}, false
}

// RuntimeDir 运行时的安装目录 / install directory of a runtime
func RuntimeDir(baseDir, component string) string {
	return filepath.Join(baseDir, "runtime", component)
}

// ExePath 组件的 java.exe 路径 / java.exe of a component
func ExePath(baseDir, component string) string {
	return filepath.Join(RuntimeDir(baseDir, component), "bin", "java.exe")
}

// runtimeManifest 版本清单结构 / per-runtime file manifest
type runtimeManifest struct {
	Files map[string]struct {
		Type     string `json:"type"`
		Executable bool `json:"executable"`
		Downloads struct {
			Raw struct {
				URL  string `json:"url"`
				SHA1 string `json:"sha1"`
				Size int64  `json:"size"`
			} `json:"raw"`
		} `json:"downloads"`
	} `json:"files"`
}

// File 下载条目 / a file to download
type File struct {
	URL  string
	SHA1 string
	Size int64
	Dest string
}

// Plan 生成某组件的下载计划 / build the download plan for a component
func Plan(ctx context.Context, baseDir string, comp Component) ([]File, error) {
	raw := strings.Replace(comp.Manifest, "piston-meta.mojang.com", "bmclapi2.bangbang93.com", 1)
	var manifest runtimeManifest
	if err := getJSON(ctx, []string{raw, comp.Manifest}, &manifest); err != nil {
		return nil, err
	}

	root := RuntimeDir(baseDir, comp.Name)
	var files []File
	for name, f := range manifest.Files {
		if f.Type != "file" || f.Downloads.Raw.URL == "" {
			continue
		}
		url := strings.Replace(f.Downloads.Raw.URL, "piston-data.mojang.com", "bmclapi2.bangbang93.com", 1)
		files = append(files, File{
			URL:  url,
			SHA1: f.Downloads.Raw.SHA1,
			Size: f.Downloads.Raw.Size,
			Dest: filepath.Join(root, filepath.FromSlash(name)),
		})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("运行时清单为空")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Dest < files[j].Dest })
	return files, nil
}

// EnsureExecutable 确认 java.exe 存在 / ensure java.exe exists
func EnsureExecutable(baseDir, component string) error {
	exe := ExePath(baseDir, component)
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("运行时安装不完整: %w", err)
	}
	return nil
}
