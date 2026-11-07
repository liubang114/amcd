# AL-1S Minecraft Downloader (amcd)

一个用 Go 写的 Minecraft 下载器，单文件 exe、无运行时依赖，支持中文/英文界面。
A single-binary Minecraft downloader written in Go, with Chinese/English UI and zero runtime dependencies.

```
>>> dl -1.20.1 -fabric
>>> ins 整合包.zip
>>> ls release
```

## 功能 / Features

- **原版下载**：正式版、快照、愚人节版本（如 `24w14potato`）全覆盖。
  Vanilla downloads for releases, snapshots and April Fools versions.
- **模组加载器**：Forge / NeoForge / Fabric / Quilt，不指定版本时自动取该 MC 版本下的最新版。
  Loaders with automatic "latest for this MC version" resolution.
- **整合包**：Modrinth（`.mrpack`）与 CurseForge（`.zip`），支持 overrides / client-overrides。
  Modrinth and CurseForge modpacks, including overlay directories.
- **BMCLAPI 加速**：域名自动改写，镜像节点失效时回退官方源，403/429 限流退避重试。
  BMCLAPI acceleration with automatic official-source fallback and rate-limit backoff.
- **可重复执行**：已存在的文件按 SHA-1 校验后跳过，中断后可继续。
  Idempotent: existing files are SHA-1 verified and skipped.
- **PCL / HMCL 兼容布局**：`libraries/`、`assets/` 放在安装根目录，版本目录只放 json、jar、natives。
  PCL/HMCL-compatible layout.

## 使用 / Usage

双击 `amcd.exe` 进入交互模式，或在终端里直接用 `amcd <命令>`。

| 命令 / Command | 说明 / Description |
| --- | --- |
| `dl` | 下载游戏 / download a game instance |
| `ins` | 安装整合包 / install a modpack |
| `ls [type]` | 按时间升序列出可下载版本 / list downloadable versions, oldest first |
| `java ls` | 列出可安装的 Java 运行时 / list installable Java runtimes |
| `java install <组件\|版本>` | 安装指定 Java 运行时 / install a specific runtime |

```bash
amcd dl -1.20.1                                        # 原版 / vanilla
amcd dl -1.20.1 -forge -47.4.28 -E:\mc                 # Forge 指定版本
amcd dl -1.21.8 -neoforge -"E:\my games\mc"            # 路径含空格请加引号
amcd ins 整合包.mrpack                                  # 安装 modpacks/ 下的整合包
amcd ls release                                        # 只看正式版（升序）
amcd java install 17                                   # 手动安装 Java 17
amcd java ls                                           # 查看可用 Java 及安装状态
```

启动时会自动在工作区生成 `config.toml`、`使用说明.txt`、`version/`、`modpacks/`、`logs/`。

## 配置文件 / Configuration

`config.toml`（TOML，含中英双语注释）：

```toml
default_path       = "./version"                          # 默认安装路径 / install root
default_source     = "https://bmclapi2.bangbang93.com/"   # 下载源 / mirror root
language           = "zh_cn"                              # en_us 或 zh_cn
curseforge_api_key = ""                                   # 可选，CurseForge 整合包解析
```

## 构建 / Build

需要 Go 1.25 或更新版本。

```bash
cd src
go build -ldflags="-s -w" -o ../amcd.exe .
```

可选：`src/build.ps1` 是 Windows 一键构建脚本；签名用 `src/sign.ps1`（需要代码签名证书）。

## 项目结构 / Layout

```
src/
├── main.go            入口 / entry point
├── assets/            内嵌的使用说明 / embedded usage guide
├── cmd/               dl、ins、ls 子命令 / CLI commands
├── config/            config.toml 读写
├── downloader/        并发下载、SHA-1、重试 / download engine
├── installer/         原版安装流水线 / vanilla install pipeline
├── loader/            Forge/NeoForge/Fabric/Quilt 安装
├── modpack/           Modrinth、CurseForge 解析与安装
├── mirror/            BMCLAPI 映射与元数据 / mirror mapping
├── metadata/          版本 JSON 结构
├── progress/          天蓝色单行进度条
├── javaruntime/       Java 探测、版本匹配与运行时下载
├── i18n/              中英文文案
└── logx/              运行日志
```

## 已知限制 / Limitations

- CurseForge 整合包若只带 `projectID/fileID`，必须填 `curseforge_api_key` 才能解析；否则这些文件会被跳过并提示数量。
- Forge / NeoForge 安装需要 Java；缺失时会询问是否自动安装（Mojang 官方运行时清单，BMCLAPI 加速，装在 `runtime/<组件>/`）。Fabric / Quilt 与原版下载不需要 Java。
- 只构建 Windows amd64；其他平台可自行交叉编译。

## 许可 / License

[MIT](LICENSE)
