package loader

// quiltArgs Quilt 安装器：install client <mc> [<loader>] --install-dir=... --no-profile
// quiltArgs installs the client into dir without writing a launcher profile
func quiltArgs(mcVersion, loaderVersion, dir string) []string {
	return []string{
		"install", "client", mcVersion, loaderVersion,
		"--install-dir=" + dir,
		"--no-profile",
	}
}
