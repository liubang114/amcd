package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"amcd/i18n"
	"amcd/logx"
	"amcd/mirror"
)

// 版本类型过滤值 / version type filters
var versionTypes = []string{"release", "snapshot", "old_beta", "old_alpha"}

// newLSCmd ls 子命令：列出可下载的游戏版本
// ls lists the game versions available for download
func newLSCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "ls [release|snapshot|old_beta|old_alpha]",
		Short:              "列出可下载的游戏版本 / list downloadable game versions",
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if wantsHelp(args) {
				return cmd.Help()
			}
			if len(args) > 1 {
				return i18n.E("err.badParam")
			}
			filter := ""
			if len(args) == 1 {
				filter = strings.ToLower(strings.TrimPrefix(strings.Trim(args[0], `"'`), "-"))
				if filter != "all" && !isVersionType(filter) {
					return i18n.E("err.badParam")
				}
			}
			logx.Info("ls filter=%q", filter)
			return runLS(filter)
		},
	}
}

func isVersionType(t string) bool {
	for _, v := range versionTypes {
		if v == t {
			return true
		}
	}
	return false
}

func runLS(filter string) error {
	client := mirror.New(appCfg.DefaultSource)
	ctx, cancel := context.WithTimeout(AppContext(), 90*time.Second)
	defer cancel()

	manifest, err := client.FetchManifest(ctx)
	if err != nil {
		logx.Error("ls manifest: %v", err)
		fmt.Println(i18n.Sky(i18n.T("msg.hint.mirror")))
		return i18n.E("err.network", err)
	}

	count := 0
	// 升序（旧 → 新），清单本身是最新在前 / ascending: oldest first
	for i := len(manifest.Versions) - 1; i >= 0; i-- {
		v := manifest.Versions[i]
		typ := v.Type
		if typ == "" {
			typ = "release"
		}
		if filter != "" && filter != "all" && typ != filter {
			continue
		}
		fmt.Println(i18n.Sky(fmt.Sprintf("%-24s %s", "-"+v.ID, typ)))
		count++
	}
	if count == 0 {
		fmt.Println(i18n.Sky(i18n.T("msg.lsEmpty")))
	}
	return nil
}
