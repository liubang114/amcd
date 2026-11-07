// Package loader 模组加载器无头安装 / headless mod loader installation
package loader

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"amcd/downloader"
	"amcd/i18n"
	"amcd/installer"
	"amcd/javaruntime"
	"amcd/logx"
	"amcd/metadata"
	"amcd/mirror"
	"amcd/winconsole"
)

// Spec 加载器安装参数 / loader installation parameters
type Spec struct {
	Loader        string
	LoaderVersion string
	MCVersion     string
	InstanceDir   string
	InstanceName  string
	Workers       int
	LogDir        string
	RootDir       string
	// EnsureJava 在缺少 Java 时由调用方询问并安装 / caller-provided Java provisioning
	EnsureJava func(required int) (string, int, error)
}

// Install 安装加载器：Fabric/Quilt 直接取 profile JSON，Forge/NeoForge 无头运行安装器
// Install handles loaders without any GUI: profile JSON for Fabric/Quilt, headless installer otherwise
func Install(ctx context.Context, client *mirror.Client, s *Spec) error {
	workDir := filepath.Join(s.InstanceDir, ".amcd", "install-"+s.Loader)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Join(s.InstanceDir, ".amcd"))

	var child *metadata.VersionJSON
	var err error
	switch s.Loader {
	case "fabric", "quilt":
		child, err = client.LoaderProfile(ctx, s.Loader, s.MCVersion, s.LoaderVersion)
		if err != nil {
			return i18n.E("err.loaderProfile", s.Loader, err)
		}
		logx.Info("loader %s %s: profile json id=%s libs=%d", s.Loader, s.LoaderVersion, child.ID, len(child.Libraries))
	default:
		child, err = runJavaInstaller(ctx, client, s, workDir)
		if err != nil {
			return err
		}
	}

	instanceJSON := filepath.Join(s.InstanceDir, s.InstanceName+".json")
	parent, err := readVersionJSON(instanceJSON)
	if err != nil {
		return err
	}
	merged := installer.Merge(parent, child)
	merged.ID = s.InstanceName
	if err := installer.WriteVersionJSON(instanceJSON, merged); err != nil {
		return err
	}
	logx.Info("loader %s %s merged: id=%s libs=%d", s.Loader, s.LoaderVersion, merged.ID, len(merged.Libraries))

	workers := s.Workers
	if workers <= 0 {
		workers = 8
	}
	libsDir := ""
	if s.RootDir != "" {
		libsDir = filepath.Join(s.RootDir, "libraries")
	}
	return installer.CompleteLibraries(ctx, client, merged, s.InstanceDir, libsDir, workers)
}

// runJavaInstaller 下载安装器并无头执行，失败时回退到安装器内置的 version.json
func runJavaInstaller(ctx context.Context, client *mirror.Client, s *Spec, workDir string) (*metadata.VersionJSON, error) {
	urls, err := client.InstallerCandidates(ctx, s.Loader, s.MCVersion, s.LoaderVersion)
	if err != nil {
		return nil, err
	}
	jarPath := filepath.Join(workDir, s.Loader+"-installer.jar")
	var dlErr error
	for _, u := range urls {
		files := []downloader.File{{URL: u, Dest: jarPath}}
		if err := installer.DownloadBatch(ctx, files, i18n.Tf("msg.dlInstaller", s.Loader), 4); err != nil {
			dlErr = err
			continue
		}
		dlErr = nil
		break
	}
	if dlErr != nil {
		return nil, dlErr
	}

	args, needsProfile := installerArgs(s, workDir)
	if needsProfile {
		profile := `{"profiles":{},"settings":{"language":"zh_cn","locale":"zh_cn"},"version":3}`
		if err := os.WriteFile(filepath.Join(workDir, "launcher_profiles.json"), []byte(profile), 0o644); err != nil {
			return nil, err
		}
	}

	required := javaruntime.Required(s.MCVersion)
	javaExe, major := javaruntime.Find(required)
	if javaExe == "" {
		if s.EnsureJava == nil {
			return nil, i18n.E("err.javaMissing")
		}
		var jerr error
		javaExe, major, jerr = s.EnsureJava(required)
		if jerr != nil {
			return nil, jerr
		}
	}
	logx.Info("loader %s %s: java=%s(%d) args=%q", s.Loader, s.LoaderVersion, javaExe, major, args)

	if err := runInstaller(ctx, javaExe, jarPath, args, workDir, s); err == nil {
		if path, ferr := findInstalledJSON(workDir, s.Loader); ferr == nil {
			if vj, rerr := readVersionJSON(path); rerr == nil {
				return vj, nil
			}
		}
	}

	// 回退：从安装器 jar 内取出版本 JSON，自行合并与补全
	logx.Warn("loader %s: 安装器不可用，改用 jar 内置版本 JSON", s.Loader)
	return versionFromInstallerJar(jarPath)
}

func runInstaller(ctx context.Context, javaExe, jarPath string, args []string, workDir string, s *Spec) error {
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	logPath := ""
	if s.LogDir != "" {
		_ = os.MkdirAll(s.LogDir, 0o755)
		logPath = filepath.Join(s.LogDir, "installer-"+s.Loader+".log")
	}

	// 独立窗口运行安装器，同时把输出写进日志（Tee-Object）
	// run the installer in its own window and tee its output into the log
	if winconsole.Available() {
		script := buildInstallerScript(javaExe, jarPath, args, workDir, logPath, s.Loader)
		scriptPath := filepath.Join(workDir, "run-installer.ps1")
		if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
			return err
		}
		err := winconsole.Run("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
		text := tailFile(logPath, 4000)
		logx.Info("installer window finished: err=%v\n%s", err, text)
		if runCtx.Err() == context.DeadlineExceeded {
			return i18n.E("err.installerTimeout")
		}
		if strings.Contains(text, "There was an error during installation") {
			return i18n.E("err.loaderIncomplete")
		}
		return nil // 失败时由调用方回退到 jar 内置 JSON / caller falls back to the bundled json
	}

	cmdArgs := append([]string{"-Djava.awt.headless=true", "-jar", jarPath}, args...)
	cmd := exec.CommandContext(runCtx, javaExe, cmdArgs...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	text := string(out)
	if logPath != "" {
		_ = os.WriteFile(logPath, out, 0o644)
	}
	tail := text
	if len(tail) > 4000 {
		tail = tail[len(tail)-4000:]
	}
	logx.Info("installer output (tail):\n%s", tail)
	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return i18n.E("err.installerTimeout")
		}
		return i18n.E("err.loaderInstall", err)
	}
	if strings.Contains(text, "There was an error during installation") {
		return i18n.E("err.loaderIncomplete")
	}
	return nil
}

// buildInstallerScript 生成新窗口运行安装器的 PowerShell 脚本
// buildInstallerScript builds the PowerShell wrapper shown in a separate window
func buildInstallerScript(javaExe, jarPath string, args []string, workDir, logPath, loaderName string) string {
	var b strings.Builder
	b.WriteString("$ErrorActionPreference = 'Continue'\r\n")
	b.WriteString("$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::new()\r\n")
	b.WriteString("[Console]::OutputEncoding = [Text.UTF8Encoding]::new()\r\n")
	b.WriteString("[Console]::Title = 'amcd - " + loaderName + " installer'\r\n")
	b.WriteString("Write-Host '运行 " + loaderName + " 安装器 / running the " + loaderName + " installer' -ForegroundColor Cyan\r\n")
	b.WriteString("Write-Host ''\r\n")
	b.WriteString("Set-Location -LiteralPath '" + escapePS(workDir) + "'\r\n")
	parts := []string{"'" + escapePS(javaExe) + "'", "'-Djava.awt.headless=true'", "'-jar'", "'" + escapePS(jarPath) + "'"}
	for _, a := range args {
		parts = append(parts, "'"+escapePS(a)+"'")
	}
	invoke := "& " + strings.Join(parts, " ") + " 2>&1"
	if logPath != "" {
		invoke += " | Tee-Object -FilePath '" + escapePS(logPath) + "'"
	}
	b.WriteString(invoke + "\r\n")
	b.WriteString("$code = $LASTEXITCODE\r\n")
	b.WriteString("Write-Host ''\r\n")
	b.WriteString("if ($code -ne 0) { Write-Host ('安装器退出码 ' + $code + '，8 秒后关闭窗口') -ForegroundColor Yellow; Start-Sleep -Seconds 8 }\r\n")
	return b.String()
}

func escapePS(s string) string { return strings.ReplaceAll(s, "'", "''") }

func tailFile(path string, limit int) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := string(data)
	if len(text) > limit {
		text = text[len(text)-limit:]
	}
	return text
}

// versionFromInstallerJar 读取安装器 jar 内的 version.json
func versionFromInstallerJar(jarPath string) (*metadata.VersionJSON, error) {
	zr, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		// 新版安装器带 version.json，1.7.10 等旧版放在 install_profile.json 的 versionInfo 中
		if f.Name != "version.json" && f.Name != "install_profile.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		if f.Name == "install_profile.json" {
			var wrapper struct {
				VersionInfo *metadata.VersionJSON `json:"versionInfo"`
			}
			if err := json.Unmarshal(data, &wrapper); err != nil || wrapper.VersionInfo == nil {
				continue
			}
			return wrapper.VersionInfo, nil
		}
		var vj metadata.VersionJSON
		if err := json.Unmarshal(data, &vj); err != nil {
			return nil, fmt.Errorf("解析安装器内置版本 JSON 失败: %w", err)
		}
		return &vj, nil
	}
	return nil, i18n.E("err.installerNoJSON")
}

func readVersionJSON(path string) (*metadata.VersionJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var vj metadata.VersionJSON
	if err := json.Unmarshal(data, &vj); err != nil {
		return nil, fmt.Errorf("解析版本 JSON 失败 %s: %w", path, err)
	}
	return &vj, nil
}

// findInstalledJSON 在安装器输出目录中找最新的版本 JSON
func findInstalledJSON(workDir, loaderName string) (string, error) {
	versionsDir := filepath.Join(workDir, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return "", fmt.Errorf("安装器没有生成版本目录: %w", err)
	}
	type cand struct {
		path string
		mod  time.Time
	}
	var found []cand
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		jsons, _ := filepath.Glob(filepath.Join(versionsDir, e.Name(), "*.json"))
		for _, j := range jsons {
			info, err := os.Stat(j)
			if err != nil {
				continue
			}
			found = append(found, cand{j, info.ModTime()})
		}
	}
	if len(found) == 0 {
		return "", fmt.Errorf("安装器没有生成版本 JSON")
	}
	sort.Slice(found, func(i, j int) bool { return found[i].mod.After(found[j].mod) })
	for _, c := range found {
		if strings.Contains(strings.ToLower(filepath.Base(c.path)), loaderName) {
			return c.path, nil
		}
	}
	return found[0].path, nil
}
