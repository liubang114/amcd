package loader

// fabricArgs Fabric 安装器：client 子命令 + -noprofile 避免写启动器 profile
// fabricArgs uses the client subcommand with -noprofile so no launcher profile is written
func fabricArgs(mcVersion, loaderVersion, dir string) []string {
	return []string{
		"client",
		"-dir", dir,
		"-mcversion", mcVersion,
		"-loader", loaderVersion,
		"-noprofile",
	}
}
