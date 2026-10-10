param(
    [Parameter(Mandatory = $true)][ValidatePattern('^[a-zA-Z0-9_.:-]+$')][string]$Server,
    [ValidateRange(1, 65535)][int]$Port = 30149,
    [ValidatePattern('^[a-zA-Z0-9_-]+$')][string]$User = 'root',
    [ValidatePattern('^/[a-zA-Z0-9_/-]+$')][string]$RemoteRoot = '/opt/hquizlet'
)
$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../..')).Path
$envPath = Join-Path $repoRoot '.env'
$installer = Join-Path $PSScriptRoot 'install-backup-env.py'
$allowed = @('R2_ACCOUNT_ID','R2_ENDPOINT','R2_ACCESS_KEY_ID','R2_SECRET_ACCESS_KEY','R2_BUCKET_NAME',
    'BACKUP_PROVIDER','BACKUP_ENABLED','BACKUP_PREFIX','BACKUP_ENCRYPTION_PASSWORD','BACKUP_ENCRYPTION_SALT',
    'BACKUP_MAX_STORAGE_BYTES','BACKUP_INTERVAL_SECONDS','BACKUP_BWLIMIT','BACKUP_TPSLIMIT','BACKUP_SOURCE_BUCKETS',
    'NEON_BACKUP_URL','POSTGRES_URL','NEON_BACKUP_ENABLED','NEON_BACKUP_INTERVAL_SECONDS',
    'NEON_BACKUP_RETRY_SECONDS','NEON_BACKUP_KEEP','NEON_BACKUP_TIMEOUT_SECONDS')
$values = @{}
foreach ($line in Get-Content -LiteralPath $envPath) {
    if ($line -match '^([A-Z_0-9]+)=(.*)$' -and $allowed -contains $matches[1]) {
        $values[$matches[1]] = $matches[2]
    }
}
foreach ($key in @('R2_ACCESS_KEY_ID','R2_SECRET_ACCESS_KEY','R2_BUCKET_NAME','BACKUP_ENCRYPTION_PASSWORD','BACKUP_ENCRYPTION_SALT')) {
    if ([string]::IsNullOrWhiteSpace($values[$key])) { throw "Missing $key in local .env" }
}
$temporaryRoot = Join-Path $repoRoot ('tmp/r2-deploy-' + [guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Path $temporaryRoot
$payload = Join-Path $temporaryRoot 'variables.env'
$destination = "${User}@${Server}"
$remoteTemp = $null
try {
    $lines = foreach ($key in $allowed) { if ($values.ContainsKey($key)) { "$key=$($values[$key])" } }
    # Restrict the local temporary secret payload to the current Windows account.
    if ($env:OS -eq 'Windows_NT') {
        $identity = [Security.Principal.WindowsIdentity]::GetCurrent().Name
        & icacls $temporaryRoot /inheritance:r /grant:r "${identity}:(OI)(CI)F" | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Cannot restrict temporary secret permissions' }
    }
    [IO.File]::WriteAllText($payload, (($lines -join "`n") + "`n"), [Text.UTF8Encoding]::new($false))
    $remoteTemp = (& ssh -p $Port -o BatchMode=yes $destination "umask 077; mktemp -d /tmp/hquizlet-r2.XXXXXXXX")
    if ($LASTEXITCODE -ne 0) { throw 'SSH connection failed' }
    $remoteTemp = "$remoteTemp".Trim()
    if ($remoteTemp -notmatch '^/tmp/hquizlet-r2\.[a-zA-Z0-9]{8}$') { throw 'Invalid remote temporary path' }
    $scpDestination = if ($Server.Contains(':')) { "${User}@[${Server}]" } else { $destination }
    & scp -P $Port -o BatchMode=yes $payload $installer "${scpDestination}:${remoteTemp}/"
    if ($LASTEXITCODE -ne 0) { throw 'Temporary payload upload failed' }
    & ssh -p $Port -o BatchMode=yes $destination "python3 '$remoteTemp/install-backup-env.py' --input '$remoteTemp/variables.env' --env '$RemoteRoot/.env'"
    if ($LASTEXITCODE -ne 0) { throw 'Remote env import failed; existing env kept unless atomic replacement completed' }
    Write-Output 'Backup variables installed on Data server.'
} finally {
    if ($remoteTemp -match '^/tmp/hquizlet-r2\.[a-zA-Z0-9]{8}$') {
        & ssh -p $Port -o BatchMode=yes $destination "rm -f -- '$remoteTemp/variables.env' '$remoteTemp/install-backup-env.py'; rmdir -- '$remoteTemp'"
        if ($LASTEXITCODE -ne 0) { Write-Warning "Remote cleanup failed; remove $remoteTemp manually on the Data server." }
    }
    $resolved = (Resolve-Path -LiteralPath $temporaryRoot).Path
    $expectedParent = [IO.Path]::GetFullPath((Join-Path $repoRoot 'tmp')) + [IO.Path]::DirectorySeparatorChar
    if ($resolved.StartsWith($expectedParent, [StringComparison]::OrdinalIgnoreCase)) {
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
