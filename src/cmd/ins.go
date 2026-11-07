package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"amcd/i18n"
	"amcd/logx"
	"amcd/mirror"
	"amcd/modpack"
)

// newInsCmd ins 子命令：安装整合包 / install a modpack
func newInsCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "ins <文件名.zip|文件名.mrpack> [-<路径>]",
		Short:              "安装整合包 / install a modpack",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true, // 路径参数形如 -E:\mc，必须原样接收
		RunE: func(cmd *cobra.Command, args []string) error {
			if wantsHelp(args) {
				return cmd.Help()
			}
			if len(args) == 0 {
				logx.Error("ins: empty args")
				return errors.New(i18n.T("err.badParam"))
			}
			logx.Info("ins args=%q", args)
			return runIns(args)
		},
	}
}

func runIns(args []string) error {
	zipArg := ""
	pathArg := ""
	for _, a := range args {
		a = strings.Trim(a, `"'`)
		if strings.HasPrefix(a, "-") && drivePath.MatchString(a[1:]) {
			if pathArg != "" {
				return errors.New(i18n.T("err.badParam"))
			}
			pathArg = a[1:]
			continue
		}
		if zipArg != "" {
			return errors.New(i18n.T("err.badParam"))
		}
		zipArg = a
	}

	lower := strings.ToLower(zipArg)
	if zipArg == "" || (!strings.HasSuffix(lower, ".zip") && !strings.HasSuffix(lower, ".mrpack")) {
		return errors.New(i18n.T("err.badParam"))
	}

	zipPath := zipArg
	if !filepath.IsAbs(zipPath) {
		if _, err := os.Stat(zipPath); err != nil {
			zipPath = filepath.Join(appCfg.BaseDir(), "modpacks", zipArg)
		}
	}
	if _, err := os.Stat(zipPath); err != nil {
		logx.Error("ins: zip not found %s", zipPath)
		return errors.New(i18n.T("err.fileMissing"))
	}

	installRoot := appCfg.InstallRoot()
	if pathArg != "" {
		installRoot = pathArg
	}

	ctx := AppContext()
	client := mirror.New(appCfg.DefaultSource)
	res, err := modpack.Install(ctx, client, &modpack.Spec{
		ZipPath:       zipPath,
		InstallRoot:   installRoot,
		Workers:       8,
		LogDir:        filepath.Join(appCfg.BaseDir(), "logs"),
		CurseForgeKey: appCfg.CurseForgeKey,
		DefaultMirror: appCfg.DefaultSource,
		EnsureJava: func(required int) (string, int, error) {
			return ensureJava(AppContext(), required)
		},
	})
	if err != nil {
		logx.Error("ins failed: %v", err)
		if errors.Is(err, modpack.ErrUnsupported) {
			return errors.New(i18n.T("err.packUnsupported"))
		}
		if errors.Is(err, context.Canceled) {
			fmt.Println(i18n.Sky(i18n.T("msg.interrupted")))
			return err
		}
		return err
	}

	fmt.Println(i18n.Tf("msg.instance", res.InstanceDir))
	if len(res.Pack.Unresolved) > 0 {
		fmt.Println(i18n.Sky(i18n.Tf("msg.packUnresolved", len(res.Pack.Unresolved))))
		logx.Warn("unresolved modpack files: %v", res.Pack.Unresolved)
	}
	fmt.Println(i18n.Sky(i18n.T("msg.done")))
	logx.Info("modpack installed %s", res.InstanceDir)
	return nil
}
