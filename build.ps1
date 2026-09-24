# Builds dist\Bonfire.exe: one self-contained file with icon and version info.
#   .\build.ps1            # version 2.0.0
#   .\build.ps1 -Version 2.1.0
param([string]$Version = "2.0.0")

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go run ./tools/genres -version $Version
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w -X main.version=$Version" -o dist\Bonfire.exe .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$size = [math]::Round((Get-Item dist\Bonfire.exe).Length / 1MB, 1)
Write-Host "Built dist\Bonfire.exe ($size MB, v$Version)"
