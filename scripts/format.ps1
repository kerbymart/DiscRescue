$ErrorActionPreference = "Stop"

$goFiles = Get-ChildItem -Path . -Recurse -Filter *.go | ForEach-Object { $_.FullName }

if ($goFiles.Count -eq 0) {
    Write-Host "No Go files found."
    exit 0
}

gofmt -w $goFiles
