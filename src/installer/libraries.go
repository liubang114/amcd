package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"amcd/downloader"
	"amcd/i18n"
	"amcd/metadata"
	"amcd/mirror"
	"amcd/progress"
)

// DownloadBatch 下载一批文件并自动重试失败项（最多 3 轮）
// DownloadBatch runs a batch and retries failed files up to three passes
func DownloadBatch(ctx context.Context, files []downloader.File, desc string, workers int) error {
	if len(files) == 0 {
		return nil
	}
	var lastErr error
	for pass := 0; pass < 3 && len(files) > 0; pass++ {
		var (
			mu     sync.Mutex
			failed []downloader.File
		)
		bar := progress.New(sumSize(files), desc)
		_, err := downloader.Run(ctx, files, downloader.Options{
			Workers:  workers,
			AddBytes: func(n int) { _ = bar.Add(n) },
			OnError: func(f downloader.File, _ error) {
				mu.Lock()
				failed = append(failed, f)
				mu.Unlock()
			},
		})
		bar.Finish()
		if err == nil {
			return nil
		}
		lastErr = err
		if len(failed) == 0 || ctx.Err() != nil {
			return err
		}
		files = failed
	}
	return lastErr
}

// LibraryFiles 依赖库与 natives 的下载清单 / download list for libraries and natives
func LibraryFiles(client *mirror.Client, libs []metadata.Library, libsDir string) []downloader.File {
	var files []downloader.File

	for _, lib := range libs {
		if !allowed(lib) {
			continue
		}
		if a := lib.Downloads.Artifact; a.URL != "" {
			path := a.Path
			if path == "" {
				path = mavenPath(lib.Name, "")
			}
			files = append(files, downloader.File{
				URL:         client.Rewrite(a.URL),
				FallbackURL: a.URL,
				Dest:        filepath.Join(libsDir, filepath.FromSlash(path)),
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
					URL:         client.Rewrite(a.URL),
					FallbackURL: a.URL,
					Dest:        filepath.Join(libsDir, filepath.FromSlash(path)),
					SHA1:        a.SHA1,
					Size:        a.Size,
				})
			}
		}
		// Forge / NeoForge 使用 maven 段式描述
		if lib.Downloads.Artifact.URL == "" && lib.URL != "" && lib.Name != "" {
			path := mavenPath(lib.Name, "")
			files = append(files, downloader.File{
				URL:         client.Rewrite(strings.TrimSuffix(lib.URL, "/") + "/" + path),
				FallbackURL: strings.TrimSuffix(lib.URL, "/") + "/" + path,
				Dest:        filepath.Join(libsDir, filepath.FromSlash(path)),
			})
		}
	}
	return files
}

// ExtractNativesFromLibs 解压当前平台的 natives（1.18 及更早的 classifier 形态）
func ExtractNativesFromLibs(libs []metadata.Library, instanceDir string) error {
	libsDir := filepath.Join(instanceDir, "libraries")
	nativesDir := filepath.Join(instanceDir, "natives")
	found := false
	for _, lib := range libs {
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
		src := filepath.Join(libsDir, filepath.FromSlash(path))
		if err := unzipFlat(src, nativesDir); err != nil {
			return err
		}
		found = true
	}
	_ = found
	return nil
}

// Merge 按 inheritsFrom 语义合并父版本与加载器版本
// Merge combines a parent version json with a loader version json
func Merge(parent, child *metadata.VersionJSON) *metadata.VersionJSON {
	out := *parent
	if child.MainClass != "" {
		out.MainClass = child.MainClass
	}
	if child.Assets != "" {
		out.Assets = child.Assets
	}
	if child.AssetIndex.URL != "" {
		out.AssetIndex = child.AssetIndex
	}
	if child.JavaVersion.MajorVersion != 0 {
		out.JavaVersion = child.JavaVersion
	}
	if child.Downloads.Client.URL != "" {
		out.Downloads = child.Downloads
	}
	if child.Type != "" {
		out.Type = child.Type
	}
	// 子版本库在前，按 group:artifact 去重
	merged := make([]metadata.Library, 0, len(child.Libraries)+len(parent.Libraries))
	seen := map[string]bool{}
	for _, l := range child.Libraries {
		key := libKey(l.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, l)
	}
	for _, l := range parent.Libraries {
		key := libKey(l.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, l)
	}
	out.Libraries = merged
	out.InheritsFrom = ""
	return &out
}

func libKey(name string) string {
	parts := strings.Split(name, ":")
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return name
}

// NormalizeLibraries 补全 dependencies 的 artifact.path/url，兼容 PCL/HMCL
// NormalizeLibraries fills artifact.path/url so launchers like PCL can resolve them
func NormalizeLibraries(vj *metadata.VersionJSON) {
	for i := range vj.Libraries {
		lib := &vj.Libraries[i]
		art := &lib.Downloads.Artifact
		if art.Path == "" && lib.Name != "" {
			art.Path = mavenPath(lib.Name, "")
		}
		if art.Path == "" {
			continue
		}
		if art.URL == "" {
			base := lib.URL
			if base == "" {
				base = "https://libraries.minecraft.net/"
			}
			if !strings.HasSuffix(base, "/") {
				base += "/"
			}
			art.URL = base + art.Path
		}
	}
}

// WriteVersionJSON 写出实例版本 JSON / write the instance version json
func WriteVersionJSON(path string, vj *metadata.VersionJSON) error {
	NormalizeLibraries(vj)
	data, err := json.MarshalIndent(vj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// CompleteLibraries 补全加载器依赖库并解压 natives
// CompleteLibraries downloads missing loader libraries and extracts natives
func CompleteLibraries(ctx context.Context, client *mirror.Client, vj *metadata.VersionJSON, instanceDir, libsDir string, workers int) error {
	if libsDir == "" {
		libsDir = filepath.Join(instanceDir, "libraries")
	}
	if err := DownloadBatch(ctx, LibraryFiles(client, vj.Libraries, libsDir), i18n.T("msg.dlLoaderLibs"), workers); err != nil {
		return err
	}
	return ExtractNativesFromLibs(vj.Libraries, instanceDir)
}
