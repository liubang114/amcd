package loader

// installerArgs 各加载器的无头安装参数；needsProfile 表示需要 launcher_profiles.json
// installerArgs returns the headless arguments and whether a launcher profile is required
func installerArgs(s *Spec, dir string) (args []string, needsProfile bool) {
	switch s.Loader {
	case "forge":
		return forgeArgs(dir), true
	case "neoforge":
		return neoForgeArgs(dir), true
	case "fabric":
		return fabricArgs(s.MCVersion, s.LoaderVersion, dir), false
	case "quilt":
		return quiltArgs(s.MCVersion, s.LoaderVersion, dir), false
	}
	return nil, false
}
