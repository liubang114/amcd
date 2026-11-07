package mirror

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"amcd/metadata"
)

// LoaderProfile 获取 Fabric / Quilt 的 profile JSON（无需运行 Java）
// LoaderProfile fetches a loader profile json without running Java
func (c *Client) LoaderProfile(ctx context.Context, loader, mcVersion, loaderVersion string) (*metadata.VersionJSON, error) {
	mc := url.PathEscape(mcVersion)
	lv := url.PathEscape(loaderVersion)

	var candidates []string
	switch loader {
	case "fabric":
		candidates = []string{
			c.base + "fabric-meta/v2/versions/loader/" + mc + "/" + lv + "/profile/json",
			"https://meta.fabricmc.net/v2/versions/loader/" + mc + "/" + lv + "/profile/json",
		}
	case "quilt":
		candidates = []string{
			c.base + "quilt-meta/v3/versions/loader/" + mc + "/" + lv + "/profile/json",
			"https://meta.quiltmc.org/v3/versions/loader/" + mc + "/" + lv + "/profile/json",
		}
	default:
		return nil, fmt.Errorf("加载器 %s 不支持 profile JSON", loader)
	}

	var lastErr error
	for _, u := range candidates {
		data, err := c.get(ctx, u)
		if err != nil {
			lastErr = err
			continue
		}
		var vj metadata.VersionJSON
		if err := json.Unmarshal(data, &vj); err != nil {
			lastErr = fmt.Errorf("解析 %s 的 profile JSON 失败: %w", loader, err)
			continue
		}
		return &vj, nil
	}
	return nil, lastErr
}
