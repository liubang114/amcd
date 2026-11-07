package mirror

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"amcd/metadata"
)

// 官方域名 → BMCLAPI 路径 / official host → BMCLAPI path
var hostRoutes = []struct {
	from string
	to   string // 相对 base 的前缀 / prefix relative to base
}{
	{"https://launchermeta.mojang.com", ""},
	{"http://launchermeta.mojang.com", ""},
	{"https://piston-meta.mojang.com", ""},
	{"http://piston-meta.mojang.com", ""},
	{"https://piston-data.mojang.com", ""},
	{"http://piston-data.mojang.com", ""},
	{"https://libraries.minecraft.net", "maven"},
	{"http://libraries.minecraft.net", "maven"},
	{"https://maven.minecraftforge.net", "maven"},
	{"http://files.minecraftforge.net/maven", "maven"},
	{"https://files.minecraftforge.net/maven", "maven"},
	{"https://maven.neoforged.net/releases", "maven"},
	{"https://maven.neoforged.net", "maven"},
	{"https://resources.download.minecraft.net", "assets"},
	{"http://resources.download.minecraft.net", "assets"},
}

// Client BMCLAPI 访问器 / BMCLAPI accessor
type Client struct {
	base string
	http *http.Client
}

// New 创建客户端，base 需为镜像根地址 / create a client for the mirror root
func New(base string) *Client {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "https://bmclapi2.bangbang93.com/"
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return &Client{
		base: base,
		http: &http.Client{
			Timeout: 60 * time.Second,
			// BMCLAPI 大文件会 302 到 OpenBMCLAPI 节点，必须跟随重定向
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

// Base 镜像根地址 / mirror root
func (c *Client) Base() string { return c.base }

// Rewrite 把官方 URL 改写为镜像 URL / rewrite an official URL onto the mirror
func (c *Client) Rewrite(raw string) string {
	for _, r := range hostRoutes {
		if strings.HasPrefix(raw, r.from) {
			rest := strings.TrimPrefix(raw, r.from)
			rest = strings.TrimPrefix(rest, "/")
			if r.to == "" {
				return c.base + rest
			}
			return c.base + r.to + "/" + rest
		}
	}
	return raw
}

// ManifestURL 版本清单地址 / version manifest URL
func (c *Client) ManifestURL() string { return c.base + "mc/game/version_manifest.json" }

// StatusError 非 200 响应 / a non-200 response
type StatusError struct {
	Code int
	URL  string
}

func (e *StatusError) Error() string { return fmt.Sprintf("HTTP %d %s", e.Code, e.URL) }

func (c *Client) getOnce(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "amcd/0.1 (AL-1S Minecraft Downloader)")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败 %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{Code: resp.StatusCode, URL: url}
	}
	return io.ReadAll(resp.Body)
}

// get 带重试的 GET：BMCLAPI 会 302 到不稳定的镜像节点
// get retries transient failures; BMCLAPI 302s to nodes that occasionally time out
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	const attempts = 3
	var lastErr error
	for i := 0; i < attempts; i++ {
		data, err := c.getOnce(ctx, url)
		if err == nil {
			return data, nil
		}
		lastErr = err
		var se *StatusError
		if errors.As(err, &se) && se.Code < 500 {
			return nil, err // 4xx 不重试 / do not retry client errors
		}
		if ctx.Err() != nil {
			return nil, lastErr
		}
		select {
		case <-ctx.Done():
			return nil, lastErr
		case <-time.After(time.Duration(i+1) * 500 * time.Millisecond):
		}
	}
	return nil, lastErr
}

// FetchManifest 获取版本清单 / fetch version_manifest.json
func (c *Client) FetchManifest(ctx context.Context) (*metadata.Manifest, error) {
	data, err := c.get(ctx, c.ManifestURL())
	if err != nil {
		return nil, err
	}
	var m metadata.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("版本清单解析失败: %w", err)
	}
	if len(m.Versions) == 0 {
		return nil, fmt.Errorf("版本清单为空")
	}
	return &m, nil
}

// FetchVersionJSON 获取版本 JSON / fetch a version json via the mirror
func (c *Client) FetchVersionJSON(ctx context.Context, v metadata.Version) (*metadata.VersionJSON, error) {
	data, err := c.get(ctx, c.Rewrite(v.URL))
	if err != nil {
		return nil, err
	}
	var vj metadata.VersionJSON
	if err := json.Unmarshal(data, &vj); err != nil {
		return nil, fmt.Errorf("版本 JSON 解析失败: %w", err)
	}
	return &vj, nil
}

// Loaders 支持的加载器名 / supported loader names
var Loaders = []string{"forge", "neoforge", "fabric", "quilt"}

// IsLoader 判断是否为已知加载器 / whether the token names a loader
func IsLoader(name string) bool {
	for _, l := range Loaders {
		if l == name {
			return true
		}
	}
	return false
}
