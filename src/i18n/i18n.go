package i18n

import (
	"errors"
	"fmt"
)

var lang = "zh_cn"

// SetLang 切换界面语言 / switch interface language
func SetLang(l string) {
	if l == "en_us" {
		lang = "en_us"
		return
	}
	lang = "zh_cn"
}

// Lang 当前语言 / current language
func Lang() string { return lang }

var messages = map[string][2]string{
	// key: {zh_cn, en_us}
	"banner.bmcl": {
		"本工具的下载功能由 BMCLAPI (https://bmclapi2.bangbang93.com/) 提供加速支持。",
		"This tool's downloads are accelerated by BMCLAPI (https://bmclapi2.bangbang93.com/).",
	},
	"prompt": {" >>>  ", " >>>  "},
	"msg.bye": {
		"再见。",
		"Goodbye.",
	},
	"err.badParam": {
		"参数无效",
		"Invalid parameter",
	},
	"err.versionMissing": {
		"没有这个游戏版本",
		"No such game version",
	},
	"err.loaderMissing": {
		"参数无效，这个游戏版本没有这个模组加载器",
		"Invalid parameter: this game version has no such mod loader",
	},
	"err.network": {
		"网络请求失败：%s",
		"Network error: %s",
	},
	"msg.stage0": {
		"阶段 0 骨架已就绪，参数已接收：%s",
		"Phase 0 skeleton ready, arguments accepted: %s",
	},
	"msg.mcVersion": {
		"游戏版本：%s（类型 %s）",
		"Game version: %s (type %s)",
	},
	"msg.loader": {
		"模组加载器：%s %s",
		"Mod loader: %s %s",
	},
	"msg.instance": {
		"实例目录：%s",
		"Instance directory: %s",
	},
	"msg.clientURL": {
		"客户端下载：%s",
		"Client download: %s",
	},
	"msg.assetIndex": {
		"资源索引：%s",
		"Asset index: %s",
	},
	"msg.stage1Only": {
		"（阶段 1：元数据解析与地址替换完成，实际下载在阶段 2 实现）",
		"(Phase 1: metadata resolved and URLs rewritten; downloading arrives in phase 2)",
	},
	"msg.dlGameFiles": {
		"下载游戏文件",
		"Downloading game files",
	},
	"msg.dlAssets": {
		"下载资源文件",
		"Downloading assets",
	},
	"msg.extractNatives": {
		"解压 Natives",
		"Extracting natives",
	},
	"msg.dlInstaller": {
		"下载 %s 安装器",
		"Downloading %s installer",
	},
	"msg.dlLoaderLibs": {
		"补全加载器依赖库",
		"Completing loader libraries",
	},
	"err.fileMissing": {
		"文件不存在",
		"File not found",
	},
	"err.packUnsupported": {
		"不支持的整合包格式",
		"Unsupported modpack format",
	},
	"msg.modpackFiles": {
		"下载整合包文件",
		"Downloading modpack files",
	},
	"msg.packUnresolved": {
		"有 %d 个模组无法解析（缺少下载地址或 CurseForge API Key）",
		"%d mods could not be resolved (missing download URL or CurseForge API key)",
	},
	"err.javaMissing": {
		"没有找到可用的 Java 运行时",
		"No usable Java runtime found",
	},
	"err.installerTimeout": {
		"加载器安装超时",
		"Loader installation timed out",
	},
	"err.loaderInstall": {
		"加载器安装失败：%v",
		"Loader installation failed: %v",
	},
	"err.loaderProfile": {
		"获取 %s 版本信息失败：%v",
		"Failed to fetch the %s profile: %v",
	},
	"err.installerNoJSON": {
		"安装器内没有 version.json",
		"Installer jar contains no version.json",
	},
	"err.packNoVersion": {
		"整合包没有声明游戏版本",
		"Modpack does not declare a game version",
	},
	"err.zipOpen": {
		"无法打开整合包：%v",
		"Cannot open modpack: %v",
	},
	"err.parseManifest": {
		"解析整合包清单失败：%v",
		"Failed to parse modpack manifest: %v",
	},
	"err.readIndex": {
		"读取资源索引失败：%v",
		"Failed to read asset index: %v",
	},
	"err.parseIndex": {
		"解析资源索引失败：%v",
		"Failed to parse asset index: %v",
	},
	"msg.interrupted": {
		"已中断，未完成的文件保留为 .part",
		"Interrupted; unfinished files are kept as .part",
	},
	"msg.hint.mirror": {
		"提示：可以更换下载源（-<下载源>）或检查网络后重试",
		"Hint: try another download source (-<source>) or check your network and retry",
	},
	"err.diskFull": {
		"磁盘空间不足",
		"Not enough disk space",
	},
	"err.permission": {
		"没有写入权限，请换一个目录或以管理员身份运行",
		"Write permission denied; choose another directory or run as administrator",
	},
	"msg.lsEmpty": {
		"没有符合条件的游戏版本",
		"No game versions matched",
	},
	"err.loaderIncomplete": {
		"加载器安装不完整，请重试",
		"Loader installation was incomplete, please retry",
	},
	"msg.javaMissingAsk": {
		"没有找到可用的 Java 运行时（需要 Java %d），是否自动下载安装？(y/n)",
		"No usable Java runtime found (Java %d required). Download and install one? (y/n)",
	},
	"msg.yesNo": {
		"请输入 y 或 n：",
		"Please answer y or n: ",
	},
	"msg.javaInstalling": {
		"将在独立窗口安装 Java %s（%s）",
		"Installing Java %s (%s) in a separate window",
	},
	"msg.javaDownloading": {
		"下载 Java 运行时 %s（%s）",
		"Downloading Java runtime %s (%s)",
	},
	"msg.javaFiles": {
		"下载 Java 文件",
		"Java files",
	},
	"msg.javaInstalled": {
		"Java 安装完成：%s",
		"Java installed: %s",
	},
	"err.javaNoComponent": {
		"没有找到满足 Java %d 的运行时",
		"No runtime satisfies Java %d",
	},
	"msg.javaStateInstalled": {
		"已安装",
		"installed",
	},
	"msg.done": {"安装完成", "Installation complete"},
	"msg.installing": {
		"安装 %s",
		"Installing %s",
	},
}

// T 取文案，缺失时回退 key 本身 / lookup with key fallback
func T(key string) string {
	m, ok := messages[key]
	if !ok {
		return key
	}
	if lang == "en_us" {
		return m[1]
	}
	return m[0]
}

// Tf 带参数的文案 / formatted message
func Tf(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

// E 生成带文案的 error / a message-backed error
func E(key string, args ...any) error {
	return errors.New(Tf(key, args...))
}

// colorEnabled 控制是否输出颜色 / whether ANSI colors are emitted
var colorEnabled = true

// SetColorEnabled 关闭颜色时退回纯文本 / fall back to plain text when colors are unsupported
func SetColorEnabled(on bool) { colorEnabled = on }

// Sky 天蓝色（#87CEEB）文本 / sky-blue text
func Sky(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[38;2;135;206;235m" + s + "\x1b[0m"
}
