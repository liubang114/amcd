package main

import (
	"fmt"

	"amcd/assets"
	"amcd/cmd"
	"amcd/config"
	"amcd/i18n"
	"amcd/logx"
	"amcd/progress"
	"amcd/termcolor"
)

func main() {
	base := config.BaseDir()

	if err := logx.Init(base); err != nil {
		fmt.Println("log init failed:", err)
	}
	// 使用说明与配置一样，首次运行自动生成 / generate the guide on first run
	if err := assets.EnsureUsage(base); err != nil {
		logx.Warn("usage guide: %v", err)
	}

	cfg, err := config.Load(base)
	if err != nil {
		fmt.Println("config load failed:", err)
		cfg = config.Default()
	}
	i18n.SetLang(cfg.Language)

	// 控制台不支持 ANSI 时退回纯文本，避免把转义符打印出来
	// fall back to plain text when the console cannot render ANSI
	if !termcolor.Enable() {
		i18n.SetColorEnabled(false)
		progress.SetColorEnabled(false)
	}

	// BMCLAPI 使用规范：启动首行（终端 + 日志）声明加速来源
	banner := i18n.T("banner.bmcl")
	logx.Info("%s", banner)
	fmt.Println(i18n.Sky(banner))

	cmd.InstallSignalHandler()
	cmd.Execute(cfg)
	logx.Close()
}
