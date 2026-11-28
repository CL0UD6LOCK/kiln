[CmdletBinding(DefaultParameterSetName = 'Benchmark')]
param(
    [Parameter(ParameterSetName = 'Benchmark')]
    [ValidateRange(1, 2147483647)]
    [int] $Pairs = 3,

    [Parameter(Mandatory, ParameterSetName = 'Regression')]
    [switch] $Regression
)

$ErrorActionPreference = 'Stop'
$dsnVariable = if ($Regression) { 'KILN_TEST_DSN' } else { 'KILN_BENCH_DSN' }
if ([string]::IsNullOrWhiteSpace([Environment]::GetEnvironmentVariable($dsnVariable))) {
    throw "Set $dsnVariable to a dedicated MySQL test database with the Kiln migration applied."
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$storePath = Join-Path $repoRoot 'internal/mysql/store.go'
$storeSource = [IO.File]::ReadAllText($storePath)
$beginClaim = 's.db.BeginTx(ctx, nil)'
if ([regex]::Matches($storeSource, [regex]::Escape($beginClaim)).Count -ne 1) {
    throw 'Expected exactly one default transaction in store.go for the Claim isolation overlay.'
}

$levels = [ordered]@{
    'REPEATABLE READ' = 'LevelRepeatableRead'
    'READ COMMITTED'  = 'LevelReadCommitted'
}
$tempDir = Join-Path ([IO.Path]::GetTempPath()) ('kiln-claim-isolation-' + [Guid]::NewGuid().ToString('N'))
$null = [IO.Directory]::CreateDirectory($tempDir)
$tempFiles = [System.Collections.Generic.List[string]]::new()
$utf8 = [Text.UTF8Encoding]::new($false)
$overlays = [ordered]@{}
$benchmarkArgs = @(
    '-test.run=^$'
    '-test.bench=^BenchmarkWorkerConsumeMySQL$/^workers=4$'
    '-test.benchtime=1x'
    '-test.count=1'
    '-test.timeout=4m'
)

Push-Location -LiteralPath $repoRoot
try {
    foreach ($level in $levels.GetEnumerator()) {
        $sourcePath = Join-Path $tempDir "$($level.Value)-store.go"
        $overlayPath = Join-Path $tempDir "$($level.Value)-overlay.json"
        $tempFiles.Add($sourcePath)
        $tempFiles.Add($overlayPath)

        $beginWithIsolation = "s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.$($level.Value)})"
        [IO.File]::WriteAllText($sourcePath, $storeSource.Replace($beginClaim, $beginWithIsolation), $utf8)
        $overlay = @{ Replace = @{ $storePath = $sourcePath } } | ConvertTo-Json -Compress
        [IO.File]::WriteAllText($overlayPath, $overlay, $utf8)
        $overlays[$level.Key] = $overlayPath
    }

    if ($Regression) {
        $failedLevels = @()
        foreach ($level in $overlays.GetEnumerator()) {
            Write-Output "Full regression - $($level.Key)"
            & go test -overlay $level.Value -p 1 -v -count=1 -timeout 4m ./...
            if ($LASTEXITCODE -ne 0) {
                $failedLevels += $level.Key
            }
        }
        if ($failedLevels.Count -ne 0) {
            throw "Regression failed under $($failedLevels -join ', ')."
        }
        return
    }

    $binaries = @{}
    foreach ($level in $overlays.GetEnumerator()) {
        $binaryPath = [IO.Path]::ChangeExtension($level.Value, '.test.exe')
        $tempFiles.Add($binaryPath)
        & go test -c -overlay $level.Value -o $binaryPath ./internal/worker
        if ($LASTEXITCODE -ne 0) {
            throw "Could not compile the $($level.Key) benchmark."
        }
        $binaries[$level.Key] = $binaryPath
    }

    for ($pair = 1; $pair -le $Pairs; $pair++) {
        foreach ($level in $levels.Keys) {
            Write-Output "Pair $pair of $Pairs - $level"
            $binaryPath = $binaries[$level]
            $output = & $binaryPath @benchmarkArgs 2>&1
            $exitCode = $LASTEXITCODE
            $metrics = @($output | Where-Object { $_ -match '\d+ succeeded/batch\s+[\d.]+ tasks/s' })
            if ($exitCode -ne 0 -or $metrics.Count -ne 1) {
                $output | Write-Output
                throw "The $level benchmark did not complete pair $pair."
            }
            $output | Where-Object { $_ -match 'kiln:' -and $_ -notmatch 'kiln: claim failed: Error 1213' }
            $metrics
        }
    }
} finally {
    Pop-Location
    foreach ($path in $tempFiles) {
        [IO.File]::Delete($path)
    }
    [IO.Directory]::Delete($tempDir)
}
