param([switch]$Verify, [switch]$SkipDBeaver)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $projectRoot
try {
    if (-not (Test-Path -LiteralPath '.env')) { throw 'Create .env before starting the native development setup.' }
    $settings = @{}
    Get-Content -LiteralPath '.env' | ForEach-Object {
        if ($_ -match '^([A-Z_]+)=(.*)$') { $settings[$matches[1]] = $matches[2].Trim() }
    }
    foreach ($name in @('DATABASE_URL','JWT_SECRET','STAFF_PROVISIONING_KEY','JWT_TTL','HIS_TIMEOUT')) {
        if ($settings.ContainsKey($name)) { [Environment]::SetEnvironmentVariable($name, $settings[$name], 'Process') }
    }
    # Docker's internal hostnames stay in .env; native child processes use loopback.
    $env:HOSPITAL_A_API_URL = 'http://127.0.0.1:8090/a'
    $env:HOSPITAL_B_API_URL = 'http://127.0.0.1:8090/b'
    $env:MOCK_HIS_ADDR = '127.0.0.1:8090'
    $port = if ($settings.HTTP_PORT) { $settings.HTTP_PORT } else { '8080' }
    $env:HTTP_ADDR = "127.0.0.1:$port"
    $env:GIN_MODE = 'release'
    $env:GOCACHE = Join-Path $projectRoot '.cache\go-build'
    $env:GOMODCACHE = Join-Path $projectRoot '.cache\gomod'
    $runtimeRoot = Join-Path $projectRoot '.cache\native'
    New-Item -ItemType Directory -Path $runtimeRoot -Force | Out-Null
    $pgBin = 'C:\Program Files\PostgreSQL\16\bin'
    if (-not (Test-Path -LiteralPath (Join-Path $pgBin 'pg_isready.exe'))) { throw 'Install PostgreSQL 16 and provision the agnos database first.' }
    & (Join-Path $pgBin 'pg_isready.exe') -h localhost -p $settings.DB_PORT -d $settings.DB_NAME
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL is not accepting connections. Check the agnos-postgresql16 Windows service.' }

    $statePath = Join-Path $runtimeRoot 'processes.json'
    $state = @{}
    if (Test-Path -LiteralPath $statePath) {
        $saved = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
        foreach ($property in $saved.PSObject.Properties) { $state[$property.Name] = $property.Value }
    }
    foreach ($name in @('mockhis','api')) {
        $executable = Join-Path $runtimeRoot "$name.exe"
        $running = $null
        if ($state.ContainsKey($name)) { $running = Get-Process -Id $state[$name] -ErrorAction SilentlyContinue }
        if ($running -and $running.Path -eq $executable) { continue }
        & go build -o $executable "./cmd/$name"
        if ($LASTEXITCODE -ne 0) { throw "Build failed: $name" }
        if ($name -eq 'api') {
            & $executable migrate
            if ($LASTEXITCODE -ne 0) { throw 'Database migration failed.' }
            & $executable seed
            if ($LASTEXITCODE -ne 0) { throw 'Hospital seed failed.' }
        }
        $process = Start-Process -FilePath $executable -WorkingDirectory $projectRoot -WindowStyle Hidden -RedirectStandardOutput (Join-Path $runtimeRoot "$name.stdout.log") -RedirectStandardError (Join-Path $runtimeRoot "$name.stderr.log") -PassThru
        $state[$name] = $process.Id
        $state | ConvertTo-Json | Set-Content -LiteralPath $statePath -Encoding UTF8
    }

    $baseUrl = "http://127.0.0.1:$port"
    $ready = $false
    for ($attempt = 0; $attempt -lt 20; $attempt++) {
        try {
            $response = Invoke-RestMethod -Uri "$baseUrl/health/ready" -TimeoutSec 2
            if ($response.success) { $ready = $true; break }
        } catch { Start-Sleep -Milliseconds 500 }
    }
    if (-not $ready) { throw "API did not become ready. Inspect $runtimeRoot\api.stderr.log" }
    if ($Verify) {
        $env:TEST_DATABASE_URL = $env:DATABASE_URL
        & go test '-coverpkg=./internal/...' '-coverprofile=coverage.out' ./...
        if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
        & (Join-Path $PSScriptRoot 'smoke.ps1') -BaseUrl $baseUrl -ProvisioningKey $settings.STAFF_PROVISIONING_KEY
    }
    if (-not $SkipDBeaver -and (Test-Path -LiteralPath 'C:\Program Files\DBeaver\dbeaver.exe')) {
        $connection = 'name=Agnos-Local|create=false|connect=true|user=' + $settings.DB_USER + '|password=' + $settings.DB_PASSWORD + '|savePassword=true'
        Start-Process -FilePath 'C:\Program Files\DBeaver\dbeaver.exe' -ArgumentList '-nosplash','-con',$connection -WindowStyle Hidden
        Write-Output 'Requested DBeaver connection: Agnos-Local.'
    }
    Write-Output "Native API ready: $baseUrl; PostgreSQL: localhost:$($settings.DB_PORT)/$($settings.DB_NAME)"
} finally { Pop-Location }
