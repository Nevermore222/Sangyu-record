param(
    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$devToolsCandidates = @()
$devToolsRoots = @(${env:ProgramFiles(x86)}, $env:ProgramFiles, (Join-Path $env:LOCALAPPDATA 'Programs')) |
    Where-Object { $_ -and (Test-Path -LiteralPath $_) }
foreach ($root in $devToolsRoots) {
    $devToolsCandidates += Get-ChildItem -Path (Join-Path $root '*\cli.bat') -File -ErrorAction SilentlyContinue |
        Where-Object { $_.DirectoryName -match 'Tencent|WeChat|wechat' } |
        Select-Object -ExpandProperty FullName
}

$verticalArgs = @('-ExecutionPolicy', 'Bypass', '-File', (Join-Path $PSScriptRoot 'vertical-slice.ps1'))
if ($SkipBuild) {
    $verticalArgs += '-SkipBuild'
}
& powershell @verticalArgs
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

Write-Output ''
Write-Output 'Sangyu Record local environment is ready. Services remain running.'
Write-Output 'API: http://localhost:8080'
Write-Output 'Mock Provider: http://localhost:8090'
Write-Output 'MinIO console: http://localhost:9001 (sangyu / sangyu-local-secret)'
Write-Output ("Miniapp import path: " + (Join-Path $repoRoot 'miniapp\project.config.json'))
Write-Output 'Local login: the miniapp uses the development collector automatically.'
if ($devToolsCandidates.Count -gt 0) {
    Write-Output ("WeChat DevTools CLI: " + $devToolsCandidates[0])
} else {
    Write-Warning 'WeChat DevTools CLI was not found. Open DevTools manually and import project.config.json.'
}
