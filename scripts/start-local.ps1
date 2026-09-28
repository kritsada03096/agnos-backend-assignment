param([switch]$Verify)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $projectRoot
try {
    if (-not (Test-Path -LiteralPath '.env')) { throw 'Create .env from .env.example first.' }
    $settings = @{}
    Get-Content -LiteralPath '.env' | ForEach-Object {
        if ($_ -match '^([A-Z_]+)=(.*)$') { $settings[$matches[1]] = $matches[2].Trim() }
    }

    $docker = Get-Command docker -ErrorAction SilentlyContinue
    $desktopRoot = Join-Path $env:LOCALAPPDATA 'Programs\DockerDesktop'
    if (-not (Test-Path -LiteralPath $desktopRoot)) { $desktopRoot = 'C:\Program Files\Docker\Docker' }
    $dockerPath = if ($docker) { $docker.Source } else { Join-Path $desktopRoot 'resources\bin\docker.exe' }
    if (-not (Test-Path -LiteralPath $dockerPath)) { throw 'Docker Desktop is not installed.' }

    if (-not (Get-Process -Name 'com.docker.backend' -ErrorAction SilentlyContinue)) {
        Start-Process -FilePath (Join-Path $desktopRoot 'Docker Desktop.exe') -WindowStyle Hidden
    }
    Write-Output 'Waiting for Docker Engine (restart Windows first if WSL was just installed)...'
    $ready = $false
    $deadline = (Get-Date).AddMinutes(3)
    while ((Get-Date) -lt $deadline) {
        # Native stderr is suppressed during engine startup, without hiding subsequent errors.
        $previousPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        & $dockerPath info --format '{{.ServerVersion}}' 2>$null | Out-Null
        $engineExitCode = $LASTEXITCODE
        $ErrorActionPreference = $previousPreference
        if ($engineExitCode -eq 0) { $ready = $true; break }
        Start-Sleep -Seconds 5
    }
    if (-not $ready) { throw 'Docker Engine is not ready. Open Docker Desktop and resolve its startup message; a Windows restart may be required.' }

    & $dockerPath compose --profile demo config --quiet
    if ($LASTEXITCODE -ne 0) { throw 'Docker Compose configuration failed.' }
    & $dockerPath compose --profile demo up --build -d --wait --wait-timeout 180
    if ($LASTEXITCODE -ne 0) { throw 'Stack startup failed. Inspect docker compose --profile demo logs.' }

    $port = if ($settings.HTTP_PORT) { $settings.HTTP_PORT } else { '8080' }
    $baseUrl = "http://localhost:$port"
    $health = Invoke-RestMethod -Uri "$baseUrl/health/ready" -TimeoutSec 10
    if (-not $health.success) { throw 'API database readiness check failed.' }
    & (Join-Path $PSScriptRoot 'smoke.ps1') -BaseUrl $baseUrl -ProvisioningKey $settings.STAFF_PROVISIONING_KEY

    if ($Verify) {
        $previousTestURL = $env:TEST_DATABASE_URL
        try {
            $env:TEST_DATABASE_URL = $settings.DATABASE_URL
            & go test ./...
            if ($LASTEXITCODE -ne 0) { throw 'Go unit/integration tests failed.' }
        } finally { $env:TEST_DATABASE_URL = $previousTestURL }
    }

    $dbeaver = 'C:\Program Files\DBeaver\dbeaver.exe'
    if (Test-Path -LiteralPath $dbeaver) {
        $configPath = Join-Path $env:APPDATA 'DBeaverData\workspace6\General\.dbeaver\data-sources.json'
        $connectionExists = $false
        if (Test-Path -LiteralPath $configPath) {
            $config = Get-Content -LiteralPath $configPath -Raw -Encoding UTF8 | ConvertFrom-Json
            $connectionExists = @($config.connections.PSObject.Properties | Where-Object { $_.Value.name -eq 'Agnos-Local' }).Count -gt 0
        }
        if ($connectionExists) {
            $connection = 'name=Agnos-Local|create=false|connect=true'
        } else {
            $connection = 'driver=postgresql|name=Agnos-Local|host=localhost|port=' + $settings.DB_PORT + '|database=' + $settings.DB_NAME + '|user=' + $settings.DB_USER + '|password=' + $settings.DB_PASSWORD + '|savePassword=true|save=true|connect=true|create=true'
        }
        Start-Process -FilePath $dbeaver -ArgumentList '-nosplash','-con',$connection -WindowStyle Hidden
        Write-Output 'Requested DBeaver connection: Agnos-Local.'
    }
    Write-Output "API ready: $baseUrl; PostgreSQL: localhost:$($settings.DB_PORT)/$($settings.DB_NAME)"
} finally { Pop-Location }
