package loader

// forgeArgs Forge 客户端静默安装（实测：需要 launcher_profiles.json，且 --installClient 接收目录）
// forgeArgs installs the client silently; the installer requires launcher_profiles.json
func forgeArgs(dir string) []string {
	return []string{"--installClient", dir}
}
