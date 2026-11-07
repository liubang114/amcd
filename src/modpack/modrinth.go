package modpack

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

// modrinthIndex modrinth.index.json
type modrinthIndex struct {
	FormatVersion int               `json:"formatVersion"`
	Name          string            `json:"name"`
	VersionID     string            `json:"versionId"`
	Dependencies  map[string]string `json:"dependencies"`
	Files         []struct {
		Path   string `json:"path"`
		Hashes struct {
			SHA1 string `json:"sha1"`
		} `json:"hashes"`
		Env struct {
			Client string `json:"client"`
			Server string `json:"server"`
		} `json:"env"`
		Downloads []string `json:"downloads"`
		FileSize  int64    `json:"fileSize"`
	} `json:"files"`
}

func parseModrinth(zipPath string, pack *Pack) error {
	data, err := readZipEntry(zipPath, "modrinth.index.json")
	if err != nil {
		return err
	}
	var idx modrinthIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return fmt.Errorf("解析 modrinth.index.json 失败: %w", err)
	}

	pack.Name = idx.Name
	if pack.Name == "" {
		pack.Name = idx.VersionID
	}
	pack.MCVersion = idx.Dependencies["minecraft"]
	if v, ok := idx.Dependencies["fabric-loader"]; ok {
		pack.Loader, pack.LoaderVersion = "fabric", v
	} else if v, ok := idx.Dependencies["quilt-loader"]; ok {
		pack.Loader, pack.LoaderVersion = "quilt", v
	} else if v, ok := idx.Dependencies["forge"]; ok {
		pack.Loader, pack.LoaderVersion = "forge", v
	} else if v, ok := idx.Dependencies["neoforge"]; ok {
		pack.Loader, pack.LoaderVersion = "neoforge", v
	}

	for _, f := range idx.Files {
		// 客户端不支持的模组直接跳过 / skip files the client cannot use
		if f.Env.Client == "unsupported" {
			continue
		}
		if len(f.Downloads) == 0 {
			pack.Unresolved = append(pack.Unresolved, f.Path)
			continue
		}
		rel := strings.TrimPrefix(path.Clean(strings.ReplaceAll(f.Path, "\\", "/")), "/")
		if strings.HasPrefix(rel, "..") {
			continue // 拒绝越界路径 / refuse path traversal
		}
		pack.RemoteFiles = append(pack.RemoteFiles, RemoteFile{
			Path: rel,
			URL:  f.Downloads[0],
			SHA1: f.Hashes.SHA1,
			Size: f.FileSize,
		})
	}
	pack.Overlays = []string{"overrides/", "client-overrides/"}
	return nil
}
