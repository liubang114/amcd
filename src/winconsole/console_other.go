//go:build !windows

// Package winconsole 非 Windows 平台退化为当前终端 / fallback on non-Windows
package winconsole

import "os/exec"

// Run 直接运行命令 / run the command directly
func Run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

// Available 是否支持独立窗口 / whether separate consoles are supported
func Available() bool { return false }
