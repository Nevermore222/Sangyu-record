$required = @('go', 'node', 'npm', 'docker')
$missing = @()
foreach ($command in $required) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) {
        $missing += $command
    }
}

if ($missing.Count -gt 0) {
    Write-Error ('Missing required commands: ' + ($missing -join ', '))
    exit 1
}

$goVersion = (& go version)
if ($goVersion -notmatch 'go1\.26\.') {
    Write-Error "Go 1.26.x is required; found: $goVersion"
    exit 1
}

docker info | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Error 'Docker Desktop must be running.'
    exit 1
}

Write-Output 'Prerequisites verified.'
