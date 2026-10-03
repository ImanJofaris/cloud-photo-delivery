[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $root

$localDir = Join-Path $root '.local'
$runDir = Join-Path $localDir 'run'
$logDir = Join-Path $localDir 'logs'
New-Item -ItemType Directory -Force -Path $runDir, $logDir | Out-Null

function Test-Docker {
    docker info --format '{{.ServerVersion}}' 2>$null | Out-Null
    return $LASTEXITCODE -eq 0
}

function Wait-Docker {
    $deadline = (Get-Date).AddSeconds(180)
    while ((Get-Date) -lt $deadline) {
        if (Test-Docker) { return }
        Start-Sleep -Seconds 5
    }
    throw 'Docker did not become ready within 180 seconds.'
}

$envFile = Join-Path $root '.env'
if (-not (Test-Path -LiteralPath $envFile)) {
    throw '.env is missing. Create it first: copy .env.example .env'
}

if (-not (Test-Docker)) {
    $desktop = 'C:\Program Files\Docker\Docker\Docker Desktop.exe'
    if (-not (Test-Path -LiteralPath $desktop)) {
        throw 'Docker is not running and Docker Desktop was not found.'
    }
    Write-Host 'Starting Docker Desktop...'
    Start-Process -FilePath $desktop
    Wait-Docker
}

Write-Host 'Starting Postgres and MinIO...'
docker compose up -d
if ($LASTEXITCODE -ne 0) { throw 'docker compose up failed.' }

Write-Host 'Waiting for the MinIO bucket...'
$deadline = (Get-Date).AddSeconds(90)
$ready = $false
while (-not $ready -and (Get-Date) -lt $deadline) {
    Start-Sleep -Seconds 3
    $logs = docker compose logs createbucket 2>&1 | Out-String
    $ready = $logs -match 'bucket ready'
}
if (-not $ready) { throw "The MinIO bucket was not created. Check 'docker compose logs createbucket'." }

if (-not (Get-Command goose -ErrorAction SilentlyContinue)) {
    throw 'goose is not on PATH. Install: go install github.com/pressly/goose/v3/cmd/goose@latest'
}

$config = @{}
Get-Content -LiteralPath $envFile | Where-Object { $_ -match '^\s*[^#\s][^=]*=' } | ForEach-Object {
    $name, $value = $_ -split '=', 2
    $config[$name.Trim()] = $value.Trim()
}

Write-Host 'Applying migrations...'
& goose -dir migrations postgres $config['DATABASE_URL'] up
if ($LASTEXITCODE -ne 0) { throw 'goose migrations failed.' }

function Start-LocalProcess {
    param([string]$Name, [string]$Body)
    $pidFile = Join-Path $localDir "$Name.pid"
    if (Test-Path -LiteralPath $pidFile) {
        $existing = Get-Content -LiteralPath $pidFile | Select-Object -First 1
        if ($existing -match '^\d+$' -and (Get-Process -Id $existing -ErrorAction SilentlyContinue)) {
            Write-Host "$Name is already running (pid $existing)."
            return
        }
        Remove-Item -LiteralPath $pidFile -Force
    }
    $cmdFile = Join-Path $runDir "$Name.cmd"
    Set-Content -LiteralPath $cmdFile -Value ($Body -split "`r?`n")
    $result = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = "cmd.exe /c `"$cmdFile`"" }
    if ($result.ReturnValue -ne 0) { throw "Failed to start $Name (WMI return value $($result.ReturnValue))." }
    Set-Content -LiteralPath $pidFile -Value $result.ProcessId
    Write-Host "Started $Name (pid $($result.ProcessId))."
}

$apiBody = @"
@echo off
cd /d "$root"
for /f "usebackq tokens=1,* delims==" %%a in (".env") do set "%%a=%%b"
go run ./cmd/api > "$logDir\api.log" 2>&1
"@
$workerBody = @"
@echo off
cd /d "$root"
for /f "usebackq tokens=1,* delims==" %%a in (".env") do set "%%a=%%b"
go run ./cmd/worker > "$logDir\worker.log" 2>&1
"@
$webBody = @"
@echo off
cd /d "$root"
pnpm dev > "$logDir\web.log" 2>&1
"@

Start-LocalProcess -Name 'api' -Body $apiBody
Start-LocalProcess -Name 'worker' -Body $workerBody
Start-LocalProcess -Name 'web' -Body $webBody

$port = ([string]$config['HTTP_ADDR'] -split ':')[-1]
if (-not $port) { $port = '18080' }
$apiUrl = "http://localhost:$port"

Write-Host 'Waiting for the API...'
$deadline = (Get-Date).AddSeconds(150)
$apiReady = $false
while (-not $apiReady -and (Get-Date) -lt $deadline) {
    Start-Sleep -Seconds 3
    $health = curl.exe -s --max-time 3 "$apiUrl/healthz" 2>$null
    $apiReady = $health -match '"status":"ok"'
}

Write-Host 'Waiting for the dashboard...'
$deadline = (Get-Date).AddSeconds(120)
$webReady = $false
while (-not $webReady -and (Get-Date) -lt $deadline) {
    Start-Sleep -Seconds 3
    $code = curl.exe -s -o NUL -w '%{http_code}' --max-time 3 http://localhost:3000/ 2>$null
    $webReady = $code -match '^\d{3}$' -and $code -ne '000'
}

Write-Host ''
if ($apiReady) {
    Write-Host "API ready: $apiUrl"
} else {
    Write-Warning "The API did not respond at $apiUrl within 150 seconds. See $logDir\api.log"
}
if ($webReady) {
    Write-Host 'Dashboard: http://localhost:3000'
} else {
    Write-Warning "The dashboard did not respond at http://localhost:3000 within 120 seconds. See $logDir\web.log"
}
Write-Host 'MinIO console: http://localhost:9001 (minioadmin / minioadmin)'
Write-Host ''
Write-Host "Logs: $logDir"
Write-Host 'Stop everything: powershell -ExecutionPolicy Bypass -File scripts\stop-local.ps1'
