package modpack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"amcd/i18n"
)

// curseManifest CurseForge 的 manifest.json
type curseManifest struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Overrides string `json:"overrides"`
	Minecraft struct {
		Version    string `json:"version"`
		ModLoaders []struct {
			ID      string `json:"id"`
			Primary bool   `json:"primary"`
		} `json:"modLoaders"`
	} `json:"minecraft"`
	Files []struct {
		ProjectID int  `json:"projectID"`
		FileID    int  `json:"fileID"`
		Required  bool `json:"required"`
		// 非标准字段，部分整合包会附带 / non-standard, provided by some packs
		DownloadURL string `json:"downloadUrl"`
		FileName    string `json:"fileName"`
	} `json:"files"`
}

func parseCurseForge(zipPath string, pack *Pack, apiKey string) error {
	data, err := readZipEntry(zipPath, "manifest.json")
	if err != nil {
		return err
	}
	var m curseManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return i18n.E("err.parseManifest", err)
	}

	pack.Name = m.Name
	pack.MCVersion = m.Minecraft.Version

	// 形如 forge-47.4.20 / fabric-0.15.0 / neoforge-20.4.237
	for _, l := range m.Minecraft.ModLoaders {
		id := l.ID
		if id == "" {
			continue
		}
		if i := strings.Index(id, "-"); i > 0 {
			pack.Loader = id[:i]
			pack.LoaderVersion = id[i+1:]
			if l.Primary {
				break
			}
		}
	}

	overlay := strings.Trim(strings.ReplaceAll(m.Overrides, "\\", "/"), "/")
	if overlay == "" {
		overlay = "overrides"
	}
	pack.Overlays = []string{overlay + "/"}

	var direct []RemoteFile
	var pending []int

	for _, f := range m.Files {
		if f.DownloadURL != "" {
			name := f.FileName
			if name == "" {
				name = path.Base(strings.TrimPrefix(f.DownloadURL, "https://"))
			}
			direct = append(direct, RemoteFile{Path: path.Join("mods", name), URL: f.DownloadURL})
			continue
		}
		pending = append(pending, f.FileID)
	}

	// 有 API Key 时通过官方接口解析 fileID → 下载地址
	if len(pending) > 0 && apiKey != "" {
		if resolved, rerr := resolveCurseForgeFiles(apiKey, pending); rerr == nil {
			direct = append(direct, resolved...)
			pending = nil
		}
	}

	for _, id := range pending {
		pack.Unresolved = append(pack.Unresolved, fmt.Sprintf("fileID=%d", id))
	}
	pack.RemoteFiles = direct
	return nil
}

type cfFile struct {
	ID          int    `json:"id"`
	FileName    string `json:"fileName"`
	DownloadURL string `json:"downloadUrl"`
	FileLength  int64  `json:"fileLength"`
	Hashes      []struct {
		Value string `json:"value"`
		Algo  int    `json:"algo"` // 1 = sha1
	} `json:"hashes"`
}

func resolveCurseForgeFiles(apiKey string, fileIDs []int) ([]RemoteFile, error) {
	body, err := json.Marshal(map[string]any{"fileIds": fileIDs})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.curseforge.com/v1/mods/files", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("User-Agent", "amcd/0.1 (AL-1S Minecraft Downloader)")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CurseForge API HTTP %d", resp.StatusCode)
	}
	var out struct {
		Data []cfFile `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	files := make([]RemoteFile, 0, len(out.Data))
	for _, f := range out.Data {
		if f.DownloadURL == "" || f.FileName == "" {
			continue
		}
		sha1 := ""
		for _, h := range f.Hashes {
			if h.Algo == 1 {
				sha1 = h.Value
			}
		}
		files = append(files, RemoteFile{
			Path: path.Join("mods", f.FileName),
			URL:  f.DownloadURL,
			SHA1: sha1,
			Size: f.FileLength,
		})
	}
	return files, nil
}
