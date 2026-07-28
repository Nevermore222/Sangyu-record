param(
    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot 'deploy\local\compose.yaml'

function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)][string]$Command,
        [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments
    )
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Command failed with exit code $LASTEXITCODE"
    }
}

function Wait-ForAPI {
    param([string]$URL)
    $deadline = (Get-Date).AddSeconds(60)
    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-RestMethod -Uri $URL -TimeoutSec 3
            if ($response.status -eq 'ok') {
                return
            }
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    throw "API did not become healthy at $URL"
}

Push-Location $repoRoot
try {
    & (Join-Path $PSScriptRoot 'verify-prerequisites.ps1')
    if ($LASTEXITCODE -ne 0) {
        throw 'Prerequisite verification failed.'
    }

    $composeArgs = @('compose', '-f', $composeFile, 'up', '-d', '--wait')
    if (-not $SkipBuild) {
        $composeArgs += '--build'
    }
    Invoke-Native docker @composeArgs

    $env:GOOSE_DRIVER = 'postgres'
    $env:GOOSE_DBSTRING = 'postgres://sangyu:sangyu@localhost:5432/sangyu?sslmode=disable'
    Invoke-Native go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir migrations up
    Wait-ForAPI 'http://localhost:8080/healthz'

    Invoke-Native go test ./...
    Invoke-Native go vet ./...
    Invoke-Native npm --prefix skills/mock-memoir test
    Invoke-Native npm --prefix miniapp test
    Invoke-Native npm --prefix miniapp run typecheck

    $env:TEST_API_URL = 'http://localhost:8080'
    Invoke-Native -Command go -Arguments @(
        'test', './test/integration', '-run', 'TestFoundationVerticalSlice', '-v', '-count=1'
    )

    Write-Output ''
    Write-Output 'Foundation vertical slice passed. Services remain running.'
    Write-Output 'API: http://localhost:8080'
    Write-Output 'MinIO console: http://localhost:9001'
} catch {
    Write-Error $_
    docker compose -f $composeFile logs --tail 120 api worker
    exit 1
} finally {
    Pop-Location
}
