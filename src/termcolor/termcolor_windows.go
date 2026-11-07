//go:build windows

// Package termcolor 打开 Windows 控制台的 ANSI 支持 / enable ANSI escapes in Windows consoles
package termcolor

import (
	"golang.org/x/sys/windows"
)

const enableVirtualTerminalProcessing = 0x0004

// Enable 尝试启用 VT 转义序列支持 / try to enable VT sequences
func Enable() bool {
	handle := windows.Handle(windows.Stdout)
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	if err := windows.SetConsoleMode(handle, mode|enableVirtualTerminalProcessing); err != nil {
		return false
	}
	return true
}
