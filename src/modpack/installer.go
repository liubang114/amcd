package modpack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"amcd/downloader"
	"amcd/i18n"
	"amcd/installer"
	"amcd/loader"
	"amcd/logx"
	"amcd/metadata"
	"amcd/mirror"
)

// Spec 整合包安装参数 / modpack installation parameters
type Spec struct {
	ZipPath       string
	InstallRoot   string
	Workers       int
	LogDir        string
	CurseForgeKey string
	DefaultMirror string
	// EnsureJava 缺少 Java 时由调用方询问安装 / Java provisioning callback
	EnsureJava func(required int) (string, int, error)
}

// Result 安装结果 / installation result
type Result struct {
	Pack        *Pack
	InstanceDir string
	Version     *metadata.VersionJSON
}

// Install 统一安装流水线：原版 → 加载器 → 模组 → 覆盖层 → 安装记录
// Install runs the shared pipeline: vanilla, loader, mod files, overlays, record
func Install(ctx context.Context, client *mirror.Client, s *Spec) (*Result, error) {
	pack, err := Parse(s.ZipPath, s.CurseForgeKey)
	if err != nil {
		return nil, err
	}
	if pack.MCVersion == "" {
		return nil, i18n.E("err.packNoVersion")
	}
	logx.Info("modpack %s format=%s mc=%s loader=%s %s files=%d unresolved=%d",
		pack.Name, pack.Format, pack.MCVersion, pack.Loader, pack.LoaderVersion, len(pack.RemoteFiles), len(pack.Unresolved))

	instanceName := pack.Name
	if pack.Loader != "" {
		instanceName = pack.Name
	}
	instanceDir := filepath.Join(s.InstallRoot, instanceName)

	// 1) 原版
	manifest, err := client.FetchManifest(ctx)
	if err != nil {
		return nil, i18n.E("err.network", err)
	}
	entry, ok := manifest.Find(pack.MCVersion)
	if !ok {
		return nil, i18n.E("err.versionMissing")
	}
	vj, err := client.FetchVersionJSON(ctx, entry)
	if err != nil {
		return nil, i18n.E("err.network", err)
	}
	vanilla := &installer.Vanilla{
		Client:       client,
		InstanceDir:  instanceDir,
		InstanceName: instanceName,
		Version:      vj,
		Workers:      s.Workers,
		RootDir:      s.InstallRoot,
	}
	if err := vanilla.Run(ctx); err != nil {
		return nil, err
	}

	// 2) 加载器
	if pack.Loader != "" {
		fmt.Println(i18n.Sky(i18n.Tf("msg.installing", pack.Loader)))
		spec := &loader.Spec{
			Loader:        pack.Loader,
			LoaderVersion: pack.LoaderVersion,
			MCVersion:     pack.MCVersion,
			InstanceDir:   instanceDir,
			InstanceName:  instanceName,
			Workers:       s.Workers,
			LogDir:        s.LogDir,
			RootDir:       s.InstallRoot,
			EnsureJava:    s.EnsureJava,
		}
		if err := loader.Install(ctx, client, spec); err != nil {
			return nil, err
		}
	}

	// 3) 模组等远端文件
	if len(pack.RemoteFiles) > 0 {
		files := make([]downloader.File, 0, len(pack.RemoteFiles))
		for _, rf := range pack.RemoteFiles {
			files = append(files, downloader.File{
				URL:  rf.URL,
				Dest: filepath.Join(instanceDir, filepath.FromSlash(rf.Path)),
				SHA1: rf.SHA1,
				Size: rf.Size,
			})
		}
		if err := installer.DownloadBatch(ctx, files, i18n.T("msg.modpackFiles"), s.Workers); err != nil {
			// 模组个别失败不中断整体安装 / a failed mod does not abort the install
			logx.Warn("modpack files: %v", err)
		}
	}

	// 4) 覆盖层（overrides → client-overrides）
	for _, overlay := range pack.Overlays {
		if err := extractSubtree(s.ZipPath, overlay, instanceDir); err != nil {
			return nil, err
		}
	}

	// 5) 安装记录
	if err := writeRecord(instanceDir, pack); err != nil {
		return nil, err
	}

	return &Result{Pack: pack, InstanceDir: instanceDir, Version: vj}, nil
}

type record struct {
	Name          string       `json:"name"`
	Format        string       `json:"format"`
	SourceZip     string       `json:"sourceZip"`
	MCVersion     string       `json:"mcVersion"`
	Loader        string       `json:"loader,omitempty"`
	LoaderVersion string       `json:"loaderVersion,omitempty"`
	Files         []RemoteFile `json:"files"`
	Unresolved    []string     `json:"unresolved,omitempty"`
}

func writeRecord(instanceDir string, pack *Pack) error {
	rec := record{
		Name:          pack.Name,
		Format:        pack.Format,
		SourceZip:     filepath.Base(pack.ZipPath),
		MCVersion:     pack.MCVersion,
		Loader:        pack.Loader,
		LoaderVersion: pack.LoaderVersion,
		Files:         pack.RemoteFiles,
		Unresolved:    pack.Unresolved,
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(instanceDir, "modpack.json"), data, 0o644)
}
