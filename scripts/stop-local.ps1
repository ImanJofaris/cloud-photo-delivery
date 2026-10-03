[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $root

$localDir = Join-Path $root '.local'

foreach ($name in @('api', 'worker', 'web')) {
    $pidFile = Join-Path $localDir "$name.pid"
    if (-not (Test-Path -LiteralPath $pidFile)) { continue }
    $procId = Get-Content -LiteralPath $pidFile | Select-Object -First 1
    if ($procId -match '^\d+$' -and (Get-Process -Id $procId -ErrorAction SilentlyContinue)) {
        taskkill /T /F /PID $procId | Out-Null
        if ($LASTEXITCODE -eq 0) { Write-Host "Stopped $name (pid $procId)." }
    }
    Remove-Item -LiteralPath $pidFile -Force
}

$repoPattern = [Regex]::Escape($root)
Get-CimInstance Win32_Process |
    Where-Object { $_.CommandLine -match $repoPattern } |
    ForEach-Object { taskkill /T /F /PID $_.ProcessId 2>$null | Out-Null }

$leftover = Get-NetTCPConnection -LocalPort 18080, 3000, 9091, 9092 -State Listen -ErrorAction SilentlyContinue
if ($leftover) {
    Write-Warning 'Some app ports are still listening:'
    $leftover | Select-Object LocalPort, OwningProcess | Format-Table -AutoSize | Out-String | Write-Host
}

Write-Host 'Stopping Postgres and MinIO...'
docker compose down
if ($LASTEXITCODE -ne 0) { throw 'docker compose down failed.' }

Write-Host 'Everything is stopped. Data in Docker volumes is preserved.'
