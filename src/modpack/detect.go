// Package modpack 整合包识别与安装 / modpack detection and installation
package modpack

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"amcd/i18n"
)

// 支持的格式 / supported formats
const (
	FormatModrinth   = "modrinth"
	FormatCurseForge = "curseforge"
)

// RemoteFile 需要下载的文件 / a file to download
type RemoteFile struct {
	Path string
	URL  string
	SHA1 string
	Size int64
}

// Pack 归一化后的整合包描述 / normalized modpack description
type Pack struct {
	Name          string
	Format        string
	MCVersion     string
	Loader        string
	LoaderVersion string
	RemoteFiles   []RemoteFile
	Overlays      []string // 覆盖层目录，按顺序应用 / overlay dirs applied in order
	Unresolved    []string // 无法解析的文件描述 / files that could not be resolved
	ZipPath       string
}

// Detect 识别整合包格式 / detect the modpack format
func Detect(zipPath string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", i18n.E("err.zipOpen", err)
	}
	defer zr.Close()

	has := map[string]bool{}
	for _, f := range zr.File {
		has[zipEntryName(f.Name)] = true
	}
	if has["modrinth.index.json"] {
		return FormatModrinth, nil
	}
	if has["manifest.json"] {
		return FormatCurseForge, nil
	}
	return "", ErrUnsupported
}

// ErrUnsupported 不支持的格式 / unsupported format
var ErrUnsupported = fmt.Errorf("unsupported modpack format")

// Parse 读取整合包清单 / read the modpack manifest
func Parse(zipPath string, curseForgeKey string) (*Pack, error) {
	format, err := Detect(zipPath)
	if err != nil {
		return nil, err
	}
	pack := &Pack{Format: format, ZipPath: zipPath}
	switch format {
	case FormatModrinth:
		err = parseModrinth(zipPath, pack)
	case FormatCurseForge:
		err = parseCurseForge(zipPath, pack, curseForgeKey)
	}
	if err != nil {
		return nil, err
	}
	if pack.Name == "" {
		pack.Name = strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
	}
	pack.Name = sanitizeName(pack.Name)
	return pack, nil
}

// readZipEntry 读取 zip 内的单个文件 / read one entry from a zip
func readZipEntry(zipPath, name string) ([]byte, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if zipEntryName(f.Name) != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("找不到 %s", name)
}

// extractSubtree 把 zip 内某个前缀下的文件解压到目标目录
// extractSubtree extracts entries under prefix into dest
func extractSubtree(zipPath, prefix, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		entry := zipEntryName(f.Name)
		if !strings.HasPrefix(entry, prefix) {
			continue
		}
		rel := strings.TrimPrefix(entry, prefix)
		if rel == "" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if strings.HasSuffix(entry, "/") {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
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

// zipEntryName 统一 zip 内路径分隔符（.NET 会写反斜杠）
// zipEntryName normalizes zip entry separators
func zipEntryName(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
}

// sanitizeName 去掉非法文件名字符 / strip characters invalid in paths
func sanitizeName(name string) string {
	replacer := strings.NewReplacer("\\", "-", "/", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	out := strings.TrimSpace(replacer.Replace(name))
	if out == "" {
		return "modpack"
	}
	return out
}
