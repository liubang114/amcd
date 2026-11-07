package loader

// neoForgeArgs NeoForge 必须显式 --install-client，否则无头环境默认装服务端
// neoForgeArgs must pass --install-client explicitly, otherwise it installs a server
func neoForgeArgs(dir string) []string {
	return []string{"--install-client", dir}
}
