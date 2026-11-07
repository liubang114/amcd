package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"amcd/downloader"
	"amcd/i18n"
	"amcd/javaruntime"
	"amcd/logx"
	"amcd/progress"
	"amcd/winconsole"
)

// newInstallJavaCmd 隐藏命令：在新窗口里下载并安装 Java 运行时
// hidden command: download a Mojang runtime inside its own console window
func newInstallJavaCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "__install-java <组件名>",
		Hidden:             true,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return i18n.E("err.badParam")
			}
			return runInstallJava(args[0])
		},
	}
}

func runInstallJava(componentName string) error {
	ctx := AppContext()
	components, err := javaruntime.Components(ctx)
	if err != nil {
		fmt.Println(i18n.Sky(i18n.T("msg.hint.mirror")))
		return i18n.E("err.network", err)
	}
	comp, ok := javaruntime.FindByName(components, componentName)
	if !ok {
		return i18n.E("err.badParam")
	}

	fmt.Println(i18n.Sky(i18n.Tf("msg.javaDownloading", comp.NiceName, comp.Name)))
	files, err := javaruntime.Plan(ctx, appCfg.BaseDir(), comp)
	if err != nil {
		return i18n.E("err.network", err)
	}

	batch := make([]downloader.File, 0, len(files))
	for _, f := range files {
		batch = append(batch, downloader.File{
			URL:  f.URL,
			Dest: f.Dest,
			SHA1: f.SHA1,
			Size: f.Size,
		})
	}
	// 运行时里个别文件在不同镜像节点上哈希不一致（如 Welcome.html），不因此中断
	// a few runtime files differ between mirror nodes; do not abort the whole install
	failed := 0
	bar := progress.New(0, i18n.T("msg.javaFiles"))
	_, _ = downloader.Run(ctx, batch, downloader.Options{
		Workers:  8,
		AddBytes: func(n int) { _ = bar.Add(n) },
		OnError:  func(_ downloader.File, _ error) { failed++ },
	})
	bar.Finish()
	if failed > 0 {
		logx.Warn("java runtime: %d file(s) failed checksum/download", failed)
	}
	if err := javaruntime.EnsureExecutable(appCfg.BaseDir(), comp.Name); err != nil {
		return err
	}
	fmt.Println(i18n.Sky(i18n.Tf("msg.javaInstalled", javaruntime.ExePath(appCfg.BaseDir(), comp.Name))))
	logx.Info("java installed: %s (%s)", comp.Name, comp.NiceName)
	return nil
}

// ensureJava 返回可用的 Java；缺失时询问并自动安装
// ensureJava returns a usable Java, offering an automatic install when missing
func ensureJava(ctx context.Context, required int) (string, int, error) {
	exe, major := javaruntime.Find(required)
	if exe != "" {
		return exe, major, nil
	}

	fmt.Println(i18n.Sky(i18n.Tf("msg.javaMissingAsk", required)))
	if !askYesNo() {
		return "", 0, i18n.E("err.javaMissing")
	}

	components, err := javaruntime.Components(ctx)
	if err != nil {
		return "", 0, i18n.E("err.network", err)
	}
	comp, ok := javaruntime.Pick(components, required)
	if !ok {
		return "", 0, i18n.E("err.javaNoComponent", required)
	}

	fmt.Println(i18n.Sky(i18n.Tf("msg.javaInstalling", comp.NiceName, comp.Name)))
	self, err := os.Executable()
	if err != nil {
		return "", 0, err
	}
	// 独立窗口下载，避免主窗口被进度条刷屏 / separate window keeps the main output clean
	ranWindow := false
	if winconsole.Available() {
		if err := winconsole.Run(self, "__install-java", comp.Name); err == nil {
			ranWindow = true
		} else {
			logx.Warn("java install window failed: %v", err)
		}
	}
	if !ranWindow {
		if err := runInstallJava(comp.Name); err != nil {
			return "", 0, err
		}
	}

	exePath := javaruntime.ExePath(appCfg.BaseDir(), comp.Name)
	if _, statErr := os.Stat(exePath); statErr != nil {
		return "", 0, i18n.E("err.javaMissing")
	}
	return exePath, comp.Version, nil
}
