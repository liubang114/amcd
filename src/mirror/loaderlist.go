package mirror

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// LoaderVersions 查询某加载器在某 MC 版本下的版本列表（顺序同接口返回）
// LoaderVersions lists loader builds for an MC version, in API order
func (c *Client) LoaderVersions(ctx context.Context, loader, mcVersion string) ([]string, error) {
	switch loader {
	case "forge":
		return c.forgeVersions(ctx, mcVersion)
	case "neoforge":
		return c.neoforgeVersions(ctx, mcVersion)
	case "fabric":
		return c.metaLoaderVersions(ctx, "fabric-meta/v2/versions/loader/", mcVersion)
	case "quilt":
		return c.quiltVersions(ctx, mcVersion)
	}
	return nil, fmt.Errorf("未知加载器: %s", loader)
}

type forgeEntry struct {
	Version   string `json:"version"`
	MCVersion string `json:"mcversion"`
	Build     int    `json:"build"`
}

func (c *Client) forgeVersions(ctx context.Context, mcVersion string) ([]string, error) {
	data, err := c.get(ctx, c.base+"forge/minecraft/"+url.PathEscape(mcVersion))
	if err != nil {
		return nil, err
	}
	var list []forgeEntry
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("Forge 版本列表解析失败: %w", err)
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if e.Version != "" {
			out = append(out, e.Version)
		}
	}
	return out, nil
}

type neoForgeEntry struct {
	Version       string `json:"version"`
	RawVersion    string `json:"rawVersion"`
	MCVersion     string `json:"mcversion"`
	InstallerPath string `json:"installerPath"`
}

func (c *Client) neoForgeList(ctx context.Context, mcVersion string) ([]neoForgeEntry, error) {
	data, err := c.get(ctx, c.base+"neoforge/list/"+url.PathEscape(mcVersion))
	if err != nil {
		return nil, err
	}
	var list []neoForgeEntry
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("NeoForge 版本列表解析失败: %w", err)
	}
	return list, nil
}

func (c *Client) neoforgeVersions(ctx context.Context, mcVersion string) ([]string, error) {
	list, err := c.neoForgeList(ctx, mcVersion)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		v := e.Version
		// 1.20.1 线的条目形如 1.20.1-47.1.105
		v = strings.TrimPrefix(v, mcVersion+"-")
		if v != "" {
			out = append(out, v)
		}
	}
	return out, nil
}

type metaLoaderEntry struct {
	Loader struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	} `json:"loader"`
}

func (c *Client) metaLoaderVersions(ctx context.Context, prefix, mcVersion string) ([]string, error) {
	data, err := c.get(ctx, c.base+prefix+url.PathEscape(mcVersion))
	if err != nil {
		return nil, err
	}
	var list []metaLoaderEntry
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("加载器版本列表解析失败: %w", err)
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if e.Loader.Version != "" {
			out = append(out, e.Loader.Version)
		}
	}
	return out, nil
}

// LatestLoaderVersion 取最新版本：优先稳定版（无 -pre 后缀），按数字逐段比较
// LatestLoaderVersion picks the highest build, preferring stable ones
func LatestLoaderVersion(loader string, versions []string) string {
	best := ""
	bestStable := ""
	for _, v := range versions {
		if v == "" {
			continue
		}
		if compareVersions(v, best) > 0 {
			best = v
		}
		if !strings.Contains(v, "-") && compareVersions(v, bestStable) > 0 {
			bestStable = v
		}
	}
	if bestStable != "" {
		return bestStable
	}
	return best
}

// compareVersions 逐段比较版本号，数字段按数值比较
// compareVersions compares version strings segment by segment
func compareVersions(a, b string) int {
	as := strings.FieldsFunc(a, func(r rune) bool { return r == '.' || r == '-' || r == '+' })
	bs := strings.FieldsFunc(b, func(r rune) bool { return r == '.' || r == '-' || r == '+' })
	for i := 0; i < len(as) || i < len(bs); i++ {
		if i >= len(as) {
			return -1
		}
		if i >= len(bs) {
			return 1
		}
		an, aerr := strconv.Atoi(as[i])
		bn, berr := strconv.Atoi(bs[i])
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				if an > bn {
					return 1
				}
				return -1
			}
		case aerr == nil && berr != nil:
			return 1 // 数字段优先于预发布标识 / numeric beats prerelease tag
		case aerr != nil && berr == nil:
			return -1
		default:
			if as[i] != bs[i] {
				if as[i] > bs[i] {
					return 1
				}
				return -1
			}
		}
	}
	return 0
}

// quiltVersions BMCLAPI 未镜像 Quilt 的按 MC 版本列表，退回官方元数据，再退回全局列表
// quiltVersions falls back from the mirror to official meta, then to the global list
func (c *Client) quiltVersions(ctx context.Context, mcVersion string) ([]string, error) {
	if list, err := c.metaLoaderVersions(ctx, "quilt-meta/v3/versions/loader/", mcVersion); err == nil && len(list) > 0 {
		return list, nil
	}
	if list, err := c.metaLoaderVersionsOfficial(ctx, "https://meta.quiltmc.org/v3/versions/loader/"+url.PathEscape(mcVersion)); err == nil && len(list) > 0 {
		return list, nil
	}

	data, err := c.get(ctx, c.base+"quilt-meta/v3/versions/loader")
	if err != nil {
		return nil, err
	}
	var list []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("Quilt 版本列表解析失败: %w", err)
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if e.Version != "" {
			out = append(out, e.Version)
		}
	}
	return out, nil
}

func (c *Client) metaLoaderVersionsOfficial(ctx context.Context, rawURL string) ([]string, error) {
	data, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var list []metaLoaderEntry
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("加载器版本列表解析失败: %w", err)
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if e.Loader.Version != "" {
			out = append(out, e.Loader.Version)
		}
	}
	return out, nil
}
