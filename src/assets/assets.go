// Package assets 内嵌随程序分发的文本资源 / embedded text assets
package assets

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed usage.txt
var usage string

// UsageFileName 使用说明文件名 / file name of the generated guide
const UsageFileName = "使用说明.txt"

// EnsureUsage 首次运行时生成使用说明 / write the guide on first run
func EnsureUsage(baseDir string) error {
	path := filepath.Join(baseDir, UsageFileName)
	if _, err := os.Stat(path); err == nil {
		return nil // 已存在则保留用户副本 / keep an existing copy
	}
	return os.WriteFile(path, []byte(usage), 0o644)
}
