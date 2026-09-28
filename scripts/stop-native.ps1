$ErrorActionPreference = 'Stop'
$runtimeRoot = Join-Path (Split-Path -Parent $PSScriptRoot) '.cache\native'
$statePath = Join-Path $runtimeRoot 'processes.json'
if (-not (Test-Path -LiteralPath $statePath)) { Write-Output 'No native API process state found.'; return }
$state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
foreach ($name in @('api','mockhis')) {
    $processID = $state.$name
    if (-not $processID) { continue }
    $process = Get-Process -Id $processID -ErrorAction SilentlyContinue
    $expectedPath = Join-Path $runtimeRoot "$name.exe"
    # A reused PID must never terminate an unrelated application.
    if ($process -and $process.Path -eq $expectedPath) {
        Stop-Process -Id $process.Id
        Write-Output "Stopped $name."
    }
}
Remove-Item -LiteralPath $statePath
Write-Output 'PostgreSQL Windows service remains running; database data is preserved.'
