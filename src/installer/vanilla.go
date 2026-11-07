// Package installer 原版实例安装流水线
// Package installer builds a vanilla instance directory
package installer

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"amcd/downloader"
	"amcd/i18n"
	"amcd/metadata"
	"amcd/mirror"
)

// Vanilla 原版安装 / vanilla installation
type Vanilla struct {
	Client       *mirror.Client
	InstanceDir  string
	InstanceName string
	Version      *metadata.VersionJSON
	Workers      int
	// RootDir 共享 libraries/assets 的启动根目录（与 PCL/HMCL 一致）；空表示放在实例目录内
	RootDir string
}

func (v *Vanilla) workers() int {
	if v.Workers > 0 {
		return v.Workers
	}
	return 8
}

// Run 执行安装 / run the installation
func (v *Vanilla) Run(ctx context.Context) error {
	if err := os.MkdirAll(v.InstanceDir, 0o755); err != nil {
		return friendlyFS(err)
	}

	// 1) 客户端 + 依赖库 + 资源索引
	if err := v.download(ctx, v.gameFiles(), i18n.T("msg.dlGameFiles")); err != nil {
		return err
	}

	// 2) 资源对象
	assetFiles, err := v.assetFiles()
	if err != nil {
		return err
	}
	if err := v.download(ctx, assetFiles, i18n.T("msg.dlAssets")); err != nil {
		return err
	}

	// 3) 早期版本的 natives（classifier 形态）
	if err := v.extractNatives(); err != nil {
		return err
	}

	// 4) 版本 JSON
	return v.writeVersionJSON()
}

// download 下载并自动重试失败文件
func (v *Vanilla) download(ctx context.Context, files []downloader.File, desc string) error {
	return DownloadBatch(ctx, files, desc, v.workers())
}

// friendlyFS 把文件系统错误翻译成可读提示 / translate filesystem errors
func friendlyFS(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, fs.ErrPermission) || strings.Contains(strings.ToLower(err.Error()), "access is denied") {
		return i18n.E("err.permission")
	}
	return err
}

// JarPath 客户端主程序路径 / path of the client jar
func (v *Vanilla) JarPath() string {
	return filepath.Join(v.InstanceDir, v.InstanceName+".jar")
}

// JSONPath 版本 JSON 路径 / path of the version json
func (v *Vanilla) JSONPath() string {
	return filepath.Join(v.InstanceDir, v.InstanceName+".json")
}

func (v *Vanilla) assetsDir() string { return filepath.Join(v.shared(), "assets") }
func (v *Vanilla) libsDir() string   { return filepath.Join(v.shared(), "libraries") }

// shared 返回 libraries/assets 的归属目录 / directory holding libraries and assets
func (v *Vanilla) shared() string {
	if v.RootDir != "" {
		return v.RootDir
	}
	return v.InstanceDir
}
func (v *Vanilla) nativesDir() string {
	return filepath.Join(v.InstanceDir, "natives")
}

func (v *Vanilla) gameFiles() []downloader.File {
	var files []downloader.File

	// 客户端
	if u := v.Version.Downloads.Client.URL; u != "" {
		files = append(files, downloader.File{
			URL:         v.Client.Rewrite(u),
			FallbackURL: u,
			Dest:        v.JarPath(),
			SHA1:        v.Version.Downloads.Client.SHA1,
			Size:        v.Version.Downloads.Client.Size,
		})
	}

	// 资源索引
	if u := v.Version.AssetIndex.URL; u != "" {
		files = append(files, downloader.File{
			URL:         v.Client.Rewrite(u),
			FallbackURL: u,
			Dest:        filepath.Join(v.assetsDir(), "indexes", v.Version.AssetIndex.ID+".json"),
			SHA1:        v.Version.AssetIndex.SHA1,
			Size:        v.Version.AssetIndex.Size,
		})
	}

	// 依赖库 + natives jar
	for _, lib := range v.Version.Libraries {
		if !allowed(lib) {
			continue
		}
		if a := lib.Downloads.Artifact; a.URL != "" {
			path := a.Path
			if path == "" {
				path = mavenPath(lib.Name, "")
			}
			files = append(files, downloader.File{
				URL:         v.Client.Rewrite(a.URL),
				FallbackURL: a.URL,
				Dest:        filepath.Join(v.libsDir(), filepath.FromSlash(path)),
				SHA1:        a.SHA1,
				Size:        a.Size,
			})
		}
		if classifier, ok := nativeClassifier(lib); ok {
			if a, ok := lib.Downloads.Classifiers[classifier]; ok && a.URL != "" {
				path := a.Path
				if path == "" {
					path = mavenPath(lib.Name, classifier)
				}
				files = append(files, downloader.File{
					URL:         v.Client.Rewrite(a.URL),
					FallbackURL: a.URL,
					Dest:        filepath.Join(v.libsDir(), filepath.FromSlash(path)),
					SHA1:        a.SHA1,
					Size:        a.Size,
				})
			}
		}
	}
	return files
}

type assetIndex struct {
	Objects map[string]struct {
		Hash string `json:"hash"`
		Size int64  `json:"size"`
	} `json:"objects"`
}

func (v *Vanilla) assetFiles() ([]downloader.File, error) {
	if v.Version.AssetIndex.ID == "" {
		return nil, nil
	}
	path := filepath.Join(v.assetsDir(), "indexes", v.Version.AssetIndex.ID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, i18n.E("err.readIndex", err)
	}
	var idx assetIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, i18n.E("err.parseIndex", err)
	}

	out := make([]downloader.File, 0, len(idx.Objects))
	for _, obj := range idx.Objects {
		if len(obj.Hash) < 2 {
			continue
		}
		rel := obj.Hash[:2] + "/" + obj.Hash
		official := "https://resources.download.minecraft.net/" + rel
		out = append(out, downloader.File{
			URL:         v.Client.Rewrite(official),
			FallbackURL: official,
			Dest:        filepath.Join(v.assetsDir(), "objects", filepath.FromSlash(rel)),
			SHA1:        obj.Hash,
			Size:        obj.Size,
		})
	}
	return out, nil
}

// extractNatives 解压 classifier natives（1.18 及更早）
func (v *Vanilla) extractNatives() error {
	type entry struct {
		dir  string
		path string
	}
	var jars []entry
	for _, lib := range v.Version.Libraries {
		if !allowed(lib) {
			continue
		}
		classifier, ok := nativeClassifier(lib)
		if !ok {
			continue
		}
		a, ok := lib.Downloads.Classifiers[classifier]
		if !ok {
			continue
		}
		path := a.Path
		if path == "" {
			path = mavenPath(lib.Name, classifier)
		}
		jars = append(jars, entry{dir: v.nativesDir(), path: filepath.Join(v.libsDir(), filepath.FromSlash(path))})
	}
	if len(jars) == 0 {
		return nil
	}
	fmt.Println(i18n.T("msg.extractNatives"))
	for _, j := range jars {
		if err := unzipFlat(j.path, j.dir); err != nil {
			return err
		}
	}
	return nil
}

func unzipFlat(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, f := range r.File {
		name := f.Name
		if strings.HasPrefix(name, "META-INF/") || strings.HasSuffix(name, "/") {
			continue
		}
		out, err := os.Create(filepath.Join(dst, filepath.Base(name)))
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}

func (v *Vanilla) writeVersionJSON() error {
	out := *v.Version
	out.ID = v.InstanceName
	data, err := json.MarshalIndent(&out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(v.JSONPath(), data, 0o644)
}

// allowed 规则求值（Windows / amd64）/ evaluate library rules
func allowed(lib metadata.Library) bool {
	if len(lib.Rules) == 0 {
		return true
	}
	allow := false
	for _, r := range lib.Rules {
		if ruleMatches(r) {
			allow = r.Action == "allow"
		}
	}
	return allow
}

func ruleMatches(r metadata.Rule) bool {
	if r.OS.Name != "" && r.OS.Name != runtime.GOOS {
		return false
	}
	if r.OS.Arch != "" && r.OS.Arch != nativeArch() {
		return false
	}
	return true
}

func nativeArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "386":
		return "x86"
	case "arm64":
		return "arm64"
	}
	return runtime.GOARCH
}

// nativeClassifier 返回当前平台的 natives classifier 名
func nativeClassifier(lib metadata.Library) (string, bool) {
	if len(lib.Natives) == 0 {
		return "", false
	}
	key, ok := lib.Natives[runtime.GOOS]
	if !ok || key == "" {
		return "", false
	}
	arch := "32"
	if runtime.GOARCH == "amd64" {
		arch = "64"
	}
	key = strings.ReplaceAll(key, "${arch}", arch)
	return key, true
}

// mavenPath 由 group:artifact:version[:classifier] 生成仓库相对路径
func mavenPath(name, classifier string) string {
	parts := strings.Split(name, ":")
	if len(parts) < 3 {
		return strings.ReplaceAll(name, ":", "/") + ".jar"
	}
	group, artifact, version := parts[0], parts[1], parts[2]
	if len(parts) >= 4 && classifier == "" {
		classifier = parts[3]
	}
	file := artifact + "-" + version
	if classifier != "" {
		file += "-" + classifier
	}
	return strings.ReplaceAll(group, ".", "/") + "/" + artifact + "/" + version + "/" + file + ".jar"
}

func sumSize(files []downloader.File) int64 {
	var total int64
	for _, f := range files {
		total += f.Size
	}
	return total
}
