param(
  [string]$Version = $(git describe --tags --always --dirty 2>$null),
  [string]$DistDir = "dist",
  [string[]]$Platforms = @("linux/amd64", "linux/arm64")
)

if (-not $Version) { $Version = "dev" }

$RootDir = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $RootDir

if (Test-Path $DistDir) { Remove-Item -Recurse -Force $DistDir }
New-Item -ItemType Directory -Force -Path $DistDir | Out-Null

npm --prefix web ci
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
npm --prefix web run build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

foreach ($Platform in $Platforms) {
  $Parts = $Platform.Split("/")
  $OS = $Parts[0]
  $Arch = $Parts[1]
  $Name = "gsm-panel-$Version-$OS-$Arch"
  $OutDir = Join-Path $DistDir $Name
  New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
  $env:CGO_ENABLED = "0"
  $env:GOOS = $OS
  $env:GOARCH = $Arch
  go build -ldflags "-s -w" -o (Join-Path $OutDir "gsm-panel") ./cmd/gsm-panel
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  Copy-Item README.md (Join-Path $OutDir "README.md")
  Copy-Item scripts/gsm-panel-install.sh (Join-Path $OutDir "gsm-panel-install.sh")
  Copy-Item -Recurse data (Join-Path $OutDir "data")
  $env:RELEASE_TAR_SOURCE = (Resolve-Path $OutDir).Path
  $env:RELEASE_TAR_OUTPUT = (Join-Path (Resolve-Path $DistDir).Path "$Name.tar.gz")
  $env:RELEASE_TAR_ROOT = $Name
  @'
import os
import tarfile

source = os.environ["RELEASE_TAR_SOURCE"]
output = os.environ["RELEASE_TAR_OUTPUT"]
root = os.environ["RELEASE_TAR_ROOT"]

with tarfile.open(output, "w:gz") as tar:
    for dirpath, dirnames, filenames in os.walk(source):
        entries = [(name, True) for name in dirnames] + [(name, False) for name in filenames]
        for name, is_dir in entries:
            path = os.path.join(dirpath, name)
            rel = os.path.relpath(path, source)
            arcname = os.path.join(root, rel).replace(os.sep, "/")
            info = tar.gettarinfo(path, arcname)
            if not is_dir and name in {"gsm-panel", "gsm-panel-install.sh"}:
                info.mode = 0o755
            if is_dir:
                tar.addfile(info)
            else:
                with open(path, "rb") as f:
                    tar.addfile(info, f)
'@ | python -
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  $env:RELEASE_TAR_SOURCE = $null
  $env:RELEASE_TAR_OUTPUT = $null
  $env:RELEASE_TAR_ROOT = $null
  Remove-Item -Recurse -Force $OutDir
}

$env:CGO_ENABLED = $null
$env:GOOS = $null
$env:GOARCH = $null

$Lines = @()
$Archives = Get-ChildItem $DistDir -Filter *.tar.gz | Sort-Object Name
foreach ($Archive in $Archives) {
  $Checksum = (Get-FileHash $Archive.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
  $Lines += "$Checksum  $($Archive.Name)"
}
[System.IO.File]::WriteAllLines((Join-Path $DistDir "SHA256SUMS"), [string[]]$Lines, [System.Text.Encoding]::ASCII)
