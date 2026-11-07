package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"

	"amcd/config"
	"amcd/i18n"
	"amcd/logx"
)

// init 关闭 cobra 的“从资源管理器启动”拦截：
// 本工具双击运行就是交互模式，不能用 cobra 默认的拦截行为
// disable cobra's explorer-launch guard: double-click is a supported entry point
func init() {
	cobra.MousetrapHelpText = ""
	cobra.MousetrapDisplayDuration = 0
}

var (
	appCfg      *config.Config
	interactive bool
	appCtx      = context.Background()
	appCancel   = func() {}
)

// AppContext 全局上下文，Ctrl+C 时取消 / global context cancelled on Ctrl+C
func AppContext() context.Context { return appCtx }

// InstallSignalHandler 注册 Ctrl+C 处理 / register Ctrl+C handling
func InstallSignalHandler() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	appCtx = ctx
	appCancel = stop
}

// InteractiveUsed 本次运行是否进入过交互模式 / whether the interactive mode was used
func InteractiveUsed() bool { return interactive }

// wantsHelp 子命令禁用了 flag 解析，需自行识别帮助请求
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			return true
		}
	}
	return false
}

// Execute 双模式入口 / dual-mode entry
func Execute(cfg *config.Config) {
	appCfg = cfg
	defer appCancel()
	if err := cfg.EnsureDirs(); err != nil {
		logx.Warn("ensure dirs: %v", err)
	}

	root := newRootCmd()
	if len(os.Args) <= 1 {
		runInteractive(root)
		return
	}

	logx.Info("command mode argv=%q", os.Args[1:])
	root.SetArgs(os.Args[1:])
	if err := root.Execute(); err != nil {
		fmt.Println(err.Error())
		logx.Error("%v", err)
		os.Exit(1)
	}
}

// NewRootCmd 供交互模式与外部复用 / reusable root command
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "amcd",
		Short:         "AL-1S Minecraft Downloader",
		Long:          "AL-1S Minecraft Downloader (amcd)\n交互模式直接运行 amcd；命令模式使用 amcd dl ... / amcd ins ...",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return errors.New(i18n.T("err.badParam"))
			}
			return cmd.Help()
		},
	}
	root.AddCommand(newDLCmd())
	root.AddCommand(newInsCmd())
	root.AddCommand(newLSCmd())
	root.AddCommand(newInstallJavaCmd())
	return root
}

func runInteractive(root *cobra.Command) {
	interactive = true
	logx.Info("interactive mode started")

	for {
		fmt.Print(i18n.Sky(i18n.T("prompt")))
		line, err := readLine()
		if line != "" {
			lower := strings.ToLower(line)
			if lower == "exit" || lower == "quit" || lower == "q" {
				fmt.Println(i18n.T("msg.bye"))
				return
			}
			logx.Info("interactive input=%q", line)
			root.SetArgs(splitArgs(line))
			if e := root.Execute(); e != nil {
				fmt.Println(e.Error())
				logx.Error("%v", e)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println()
				return
			}
			fmt.Println(err.Error())
			return
		}
	}
}
