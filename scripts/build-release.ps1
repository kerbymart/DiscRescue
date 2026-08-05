param(
  [string]$Version = "dev",
  [string]$Commit = "unknown",
  [string]$BuildDate = "unknown",
  [string]$Output = "dist/discrescue"
)

$ErrorActionPreference = "Stop"

$outputDir = Split-Path -Parent $Output
if ($outputDir) {
  New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
}

$ldflags = @(
  "-X", "discrescue/internal/buildinfo.Version=$Version",
  "-X", "discrescue/internal/buildinfo.Commit=$Commit",
  "-X", "discrescue/internal/buildinfo.BuildDate=$BuildDate"
) -join " "

go build -trimpath -ldflags $ldflags -o $Output ./cmd/discrescue
