# 构建脚本 / Build script
# 用法 / usage:  pwsh -File src\build.ps1
# 工具链位置 / toolchain location: E:\dsh工作目录\amcd-工具链\tools\go\bin\go.exe

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

$env:Path = 'E:\dsh工作目录\amcd-工具链\tools\go\bin;' + $env:Path
$env:GOPATH = 'E:\dsh工作目录\amcd-工具链\tools\gopath'
$env:GOCACHE = 'E:\dsh工作目录\amcd-工具链\tools\gocache'
$env:GOTMPDIR = 'E:\dsh工作目录\amcd-工具链\tools\tmp'
$env:GOTOOLCHAIN = 'local'
$env:GOPROXY = 'https://goproxy.cn,direct'

Set-Location (Join-Path $root 'src')
go vet ./...
if ($LASTEXITCODE -ne 0) { throw "go vet failed" }
go build -ldflags="-s -w" -o (Join-Path $root 'amcd.exe') .
if ($LASTEXITCODE -ne 0) { throw "go build failed" }
Write-Host "built: $(Join-Path $root 'amcd.exe')"
