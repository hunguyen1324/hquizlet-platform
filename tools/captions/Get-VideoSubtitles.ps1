param(
    [Parameter(Mandatory=$true)][string]$Video,
    [string]$Language = '',
    [string]$Translation = 'vi',
    [ValidateSet('auto','direct','yt-dlp')][string]$Provider = 'auto',
    [string]$Output = '',
    [string]$Srt = '',
    [switch]$Install
)
$ErrorActionPreference = 'Stop'
$projectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$toolRoot = Join-Path $projectRoot 'tmp/caption-tools-venv'
$pythonBin = Join-Path $toolRoot 'Scripts/python.exe'
$extractorBin = Join-Path $toolRoot 'Scripts/yt-dlp.exe'
if ($Install) {
    if (!(Test-Path -LiteralPath $pythonBin)) {
        & python -m venv $toolRoot
        if ($LASTEXITCODE -ne 0) { throw 'Cannot create Python environment' }
    }
    & $pythonBin -m pip install 'yt-dlp==2026.10.26'
    if ($LASTEXITCODE -ne 0) { throw 'Cannot install yt-dlp' }
}
if ($Provider -ne 'direct' -and !(Test-Path -LiteralPath $extractorBin)) {
    throw 'Run with -Install once to install yt-dlp in the workspace.'
}
# Resolve output paths before changing directory, relative to the caller.
if ($Output) { $Output = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Output) }
if ($Srt) { $Srt = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Srt) }
$toolArgs = @('run','./services/class/cmd/caption-tool','--video',$Video,'--provider',$Provider,'--yt-dlp',$extractorBin,'--translation',$Translation)
if ($Language) { $toolArgs += @('--language',$Language) }
if ($Output) { $toolArgs += @('--output',$Output) }
if ($Srt) { $toolArgs += @('--srt',$Srt) }
Push-Location $projectRoot
try { & go @toolArgs; $resultCode = $LASTEXITCODE } finally { Pop-Location }
if ($resultCode -ne 0) { throw "Subtitle tool failed (exit $resultCode); inspect the JSON message if written." }
