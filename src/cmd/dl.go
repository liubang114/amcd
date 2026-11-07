package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"amcd/i18n"
	"amcd/installer"
	"amcd/loader"
	"amcd/logx"
	"amcd/mirror"
)

// newDLCmd dl 子命令
func newDLCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "dl [-<版本>] [-forge|-neoforge|-fabric|-quilt] [-<加载器版本>] [-<路径>] [-<下载源>]",
		Short:              "下载原版或带模组加载器的游戏 / download vanilla or modded game",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true, // 版本号形如 -1.20.1，必须原样接收
		RunE: func(cmd *cobra.Command, args []string) error {
			if wantsHelp(args) {
				return cmd.Help()
			}
			if len(args) == 0 {
				logx.Error("dl: empty args")
				return errors.New(i18n.T("err.badParam"))
			}
			logx.Info("dl args=%q", args)

			opts, err := parseDL(args)
			if err != nil {
				logx.Error("dl parse: %v", err)
				return err
			}
			return runDL(opts)
		},
	}
}

func runDL(opts *dlArgs) error {
	base := opts.Source
	if base == "" {
		base = appCfg.DefaultSource
	}
	client := mirror.New(base)
	logx.Info("dl source=%s loader=%q loaderVersion=%q path=%q candidates=%q", base, opts.Loader, opts.LoaderVersion, opts.Path, opts.Candidates)

	// 元数据阶段用短超时，安装阶段靠单文件停顿看门狗，不设总超时
	// metadata uses a short deadline; installation relies on the per-file stall watchdog
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	manifest, err := client.FetchManifest(ctx)
	if err != nil {
		logx.Error("manifest: %v", err)
		fmt.Println(i18n.Sky(i18n.T("msg.hint.mirror")))
		return i18n.E("err.network", err)
	}

	mcVersion, loaderVer, err := splitVersions(opts.Candidates, func(id string) bool {
		_, ok := manifest.Find(id)
		return ok
	})
	if err != nil {
		logx.Error("dl split versions: %v", err)
		return err
	}
	if mcVersion == "" {
		logx.Error("dl: version not in manifest candidates=%q", opts.Candidates)
		return errors.New(i18n.T("err.versionMissing"))
	}

	entry, _ := manifest.Find(mcVersion)
	if !entry.IsRelease() && opts.Loader != "" {
		// 测试版 / 愚人节版本没有对应加载器
		logx.Error("dl: loader on non-release version %s (%s)", mcVersion, entry.Type)
		return errors.New(i18n.T("err.loaderMissing"))
	}

	if opts.Loader != "" {
		versions, lerr := client.LoaderVersions(ctx, opts.Loader, mcVersion)
		if lerr != nil {
			logx.Error("loader list: %v", lerr)
			return i18n.E("err.network", lerr)
		}
		if loaderVer == "" {
			loaderVer = mirror.LatestLoaderVersion(opts.Loader, versions)
		} else if !contains(versions, loaderVer) {
			logx.Error("dl: loader %s %s not available for %s", opts.Loader, loaderVer, mcVersion)
			return errors.New(i18n.T("err.loaderMissing"))
		}
		if loaderVer == "" {
			logx.Error("dl: empty loader version list for %s %s", opts.Loader, mcVersion)
			return errors.New(i18n.T("err.loaderMissing"))
		}
	} else if loaderVer != "" {
		// 没有加载器参数却给出了加载器版本
		return errors.New(i18n.T("err.badParam"))
	}

	vj, err := client.FetchVersionJSON(ctx, entry)
	if err != nil {
		logx.Error("version json: %v", err)
		return i18n.E("err.network", err)
	}

	installRoot := appCfg.InstallRoot()
	if opts.Path != "" {
		installRoot = opts.Path
	}
	instanceName := mcVersion
	if opts.Loader != "" {
		instanceName = mcVersion + "-" + opts.Loader
	}
	instanceDir := filepath.Join(installRoot, instanceName)

	fmt.Println(i18n.Tf("msg.mcVersion", mcVersion, entry.Type))
	if opts.Loader != "" {
		fmt.Println(i18n.Tf("msg.loader", opts.Loader, loaderVer))
	}
	fmt.Println(i18n.Tf("msg.instance", instanceDir))
	logx.Info("dl resolved instance=%s client=%s", instanceDir, client.Rewrite(vj.Downloads.Client.URL))

	inst := &installer.Vanilla{
		Client:       client,
		InstanceDir:  instanceDir,
		InstanceName: instanceName,
		Version:      vj,
		RootDir:      installRoot,
	}
	cancel() // 元数据完成，安装阶段不再受 180 秒限制
	installCtx := AppContext()
	if err := inst.Run(installCtx); err != nil {
		logx.Error("install failed: %v", err)
		if installCtx.Err() != nil {
			fmt.Println(i18n.Sky(i18n.T("msg.interrupted")))
			return err
		}
		fmt.Println(i18n.Sky(i18n.T("msg.hint.mirror")))
		return err
	}

	// 加载器：原版安装完成后再显示“安装 xxx”并静默安装
	if opts.Loader != "" {
		fmt.Println(i18n.Sky(i18n.Tf("msg.installing", opts.Loader)))
		spec := &loader.Spec{
			Loader:        opts.Loader,
			LoaderVersion: loaderVer,
			MCVersion:     mcVersion,
			InstanceDir:   instanceDir,
			InstanceName:  instanceName,
			RootDir:       installRoot,
			LogDir:        filepath.Join(appCfg.BaseDir(), "logs"),
			EnsureJava: func(required int) (string, int, error) {
				return ensureJava(AppContext(), required)
			},
		}
		if err := loader.Install(installCtx, client, spec); err != nil {
			logx.Error("loader install failed: %v", err)
			if installCtx.Err() != nil {
				fmt.Println(i18n.Sky(i18n.T("msg.interrupted")))
				return err
			}
			fmt.Println(i18n.Sky(i18n.T("msg.hint.mirror")))
			return err
		}
	}

	fmt.Println(i18n.Sky(i18n.Tf("msg.done")))
	logx.Info("install done instance=%s", instanceDir)
	return nil
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
