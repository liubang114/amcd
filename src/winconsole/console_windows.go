//go:build windows

// Package winconsole 在独立控制台窗口中运行程序 / run a program in its own console
package winconsole

import (
	"os/exec"
	"syscall"
)

// Run 新窗口运行并等待退出 / run in a new console window and wait
func Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010} // CREATE_NEW_CONSOLE
	return cmd.Run()
}

// Available 是否支持独立窗口 / whether separate consoles are supported
func Available() bool { return true }
