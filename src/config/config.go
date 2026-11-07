package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// 默认配置文件内容（首次运行自动生成）/ Default config file content (written on first run)
const defaultFile = `# 默认下载目录 / Default download directory
# 游戏与整合包将安装到此路径下的 <实例名>/ 子目录
# Games and modpacks are installed into <instance>/ under this path
default_path = "./version"

# 默认下载源 / Default download source
# BMCLAPI 镜像根地址 / BMCLAPI mirror root URL
default_source = "https://bmclapi2.bangbang93.com/"

# 界面语言 / Interface language
# 可选值 / Available: en_us, zh_cn
language = "zh_cn"

# CurseForge API Key（可留空）/ CurseForge API key (optional)
# 整合包的 CurseForge 文件解析需要 Key，留空时只能使用整合包自带的直链
# CurseForge file resolution needs a key; leave empty to use only inline download URLs
curseforge_api_key = ""
`

// Config 对应 config 文件 / mirrors the config file
type Config struct {
	DefaultPath   string `toml:"default_path"`
	DefaultSource string `toml:"default_source"`
	Language      string `toml:"language"`
	CurseForgeKey string `toml:"curseforge_api_key"`

	baseDir    string
	configFile string
}

// Default 返回内置默认值 / built-in defaults
func Default() *Config {
	return &Config{
		DefaultPath:   "./version",
		DefaultSource: "https://bmclapi2.bangbang93.com/",
		Language:      "zh_cn",
	}
}

// BaseDir 返回程序根目录（exe 所在目录，AMCD_HOME 可覆盖）
// BaseDir returns the program root (directory of the exe; AMCD_HOME overrides)
func BaseDir() string {
	if home := os.Getenv("AMCD_HOME"); home != "" {
		if abs, err := filepath.Abs(home); err == nil {
			return abs
		}
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		return filepath.Dir(exe)
	}
	wd, _ := os.Getwd()
	return wd
}

// 配置文件：默认写入 config.toml，同时兼容无后缀的旧文件名 config
// config file: written as config.toml, legacy extension-less "config" still read
const (
	fileName       = "config.toml"
	legacyFileName = "config"
)

// Load 读取配置文件，缺失时写入默认文件 / read config, creating the default when missing
func Load(baseDir string) (*Config, error) {
	cfg := Default()
	cfg.baseDir = baseDir
	cfg.configFile = filepath.Join(baseDir, fileName)

	// 兼容旧版无后缀配置 / fall back to the legacy file name
	if _, err := os.Stat(cfg.configFile); errors.Is(err, os.ErrNotExist) {
		legacy := filepath.Join(baseDir, legacyFileName)
		if _, lerr := os.Stat(legacy); lerr == nil {
			cfg.configFile = legacy
		}
	}

	data, err := os.ReadFile(cfg.configFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return cfg, err
		}
		cfg.configFile = filepath.Join(baseDir, fileName)
		if werr := os.WriteFile(cfg.configFile, []byte(defaultFile), 0o644); werr != nil {
			return cfg, werr
		}
		return cfg, nil
	}

	if _, derr := toml.Decode(string(data), cfg); derr != nil {
		return cfg, derr
	}
	cfg.baseDir = baseDir
	cfg.normalize()
	return cfg, nil
}

func (c *Config) normalize() {
	if c.DefaultPath == "" {
		c.DefaultPath = "./version"
	}
	if c.DefaultSource == "" {
		c.DefaultSource = "https://bmclapi2.bangbang93.com/"
	}
	if c.DefaultSource[len(c.DefaultSource)-1] != '/' {
		c.DefaultSource += "/"
	}
	if c.Language != "en_us" && c.Language != "zh_cn" {
		c.Language = "zh_cn"
	}
}

// BaseDir 返回配置文件所在根目录 / returns the root directory
func (c *Config) BaseDir() string { return c.baseDir }

// ConfigFile 返回 config 完整路径 / full path of the config file
func (c *Config) ConfigFile() string { return c.configFile }

// InstallRoot 返回默认安装根目录（相对路径基于程序根目录）
// InstallRoot resolves default_path (relative paths are anchored at the root)
func (c *Config) InstallRoot() string {
	if filepath.IsAbs(c.DefaultPath) {
		return filepath.Clean(c.DefaultPath)
	}
	return filepath.Join(c.baseDir, filepath.FromSlash(c.DefaultPath))
}

// EnsureDirs 创建运行所需目录 / create the runtime directories
func (c *Config) EnsureDirs() error {
	for _, dir := range []string{c.InstallRoot(), filepath.Join(c.baseDir, "modpacks"), filepath.Join(c.baseDir, "logs")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
