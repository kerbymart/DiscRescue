$ErrorActionPreference = "Stop"

Write-Host "[release] format"
& "$PSScriptRoot\format.ps1"

Write-Host "[release] baseline tests"
& "$PSScriptRoot\test.ps1"

Write-Host "[release] vet and build"
& "$PSScriptRoot\check.ps1"

Write-Host "[release] command audit"
go test ./internal/testdevice -run "TestRequestAudit"

Write-Host "[release] simulator integration"
go test ./internal/testdevice -run "TestScenario"

Write-Host "[release] soak and leak"
go test ./internal/testdevice -run "TestScenarioValidation(Soak|NoGoroutineLeak)"

if ($env:CGO_ENABLED -eq "1") {
  Write-Host "[release] race"
  go test -race ./...
}
else {
  Write-Warning "Skipping race gate because CGO_ENABLED is not set to 1 on this Windows environment."
}

Write-Host "[release] throughput and cpu benchmarks"
go test -run "^$" -bench "Benchmark(BuildPlan|VerifyExternal|VerifyImage)" ./internal/merge ./internal/integrity
