package mirror

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// metaInstaller 元数据里的安装器条目 / an installer entry from loader meta
type metaInstaller struct {
	URL     string `json:"url"`
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

// InstallerCandidates 返回安装器 jar 的候选地址（镜像优先，官方兜底）
// InstallerCandidates returns installer jar URLs, mirror first
func (c *Client) InstallerCandidates(ctx context.Context, loader, mcVersion, loaderVersion string) ([]string, error) {
	switch loader {
	case "fabric":
		return c.metaInstallerURL(ctx, "fabric-meta/v2/versions/installer")
	case "quilt":
		return c.metaInstallerURL(ctx, "quilt-meta/v3/versions/installer")
	case "forge":
		// 1.8 之前的 Forge 依赖坐标是 <mc>-<ver>-<mc>
		coords := []string{fmt.Sprintf("%s-%s", mcVersion, loaderVersion)}
		if legacyForgeNaming(mcVersion) {
			coords = []string{fmt.Sprintf("%s-%s-%s", mcVersion, loaderVersion, mcVersion), coords[0]}
		} else {
			coords = append(coords, fmt.Sprintf("%s-%s-%s", mcVersion, loaderVersion, mcVersion))
		}
		var out []string
		for _, coord := range coords {
			rel := fmt.Sprintf("net/minecraftforge/forge/%s/forge-%s-installer.jar", coord, coord)
			out = append(out, c.base+"maven/"+rel, "https://maven.minecraftforge.net/"+rel)
		}
		return out, nil
	case "neoforge":
		if isLegacyNeoForge(mcVersion) {
			rel := fmt.Sprintf("net/neoforged/forge/%s-%s/forge-%s-%s-installer.jar", mcVersion, loaderVersion, mcVersion, loaderVersion)
			return []string{c.base + "maven/" + rel, "https://maven.neoforged.net/releases/" + rel}, nil
		}
		rel := fmt.Sprintf("net/neoforged/neoforge/%s/neoforge-%s-installer.jar", loaderVersion, loaderVersion)
		return []string{c.base + "maven/" + rel, "https://maven.neoforged.net/releases/" + rel}, nil
	}
	return nil, fmt.Errorf("未知加载器: %s", loader)
}

// legacyForgeNaming 1.8 之前的 Forge 使用 <mc>-<ver>-<mc> 坐标
func legacyForgeNaming(mcVersion string) bool {
	parts := strings.Split(mcVersion, ".")
	if len(parts) < 2 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return major < 2 && minor < 8
}

// isLegacyNeoForge 1.20.1 及更早使用 net.neoforged:forge
func isLegacyNeoForge(mcVersion string) bool {
	return mcVersion <= "1.20.1"
}

func (c *Client) metaInstallerURL(ctx context.Context, path string) ([]string, error) {
	data, err := c.get(ctx, c.base+path)
	if err != nil {
		return nil, err
	}
	var list []metaInstaller
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("安装器版本列表解析失败: %w", err)
	}
	var out []string
	for _, e := range list {
		if e.URL == "" || !strings.HasSuffix(e.URL, ".jar") {
			continue
		}
		if e.Stable {
			return append([]string{e.URL}, out...), nil
		}
		out = append(out, e.URL)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("没有可用的安装器版本")
	}
	return out, nil
}

// NeoForgeInstallerPath 旧版 NeoForge 列表里的安装器相对路径
func (c *Client) NeoForgeInstallerPath(ctx context.Context, mcVersion, version string) (string, error) {
	list, err := c.neoForgeList(ctx, mcVersion)
	if err != nil {
		return "", err
	}
	for _, e := range list {
		v := strings.TrimPrefix(e.Version, mcVersion+"-")
		if v == version && e.InstallerPath != "" {
			return e.InstallerPath, nil
		}
	}
	return "", fmt.Errorf("没有找到对应的安装器")
}
