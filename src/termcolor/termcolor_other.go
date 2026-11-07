//go:build !windows

package termcolor

// Enable 非 Windows 平台默认支持 / non-Windows platforms support ANSI already
func Enable() bool { return true }
