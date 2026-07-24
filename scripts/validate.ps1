[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$goCommand = Get-Command go -ErrorAction Stop
$gofmtCommand = Get-Command gofmt -ErrorAction Stop
$gitMarker = Join-Path $repoRoot '.git'
if (Test-Path -LiteralPath $gitMarker -PathType Container) {
    $gitDirectory = $gitMarker
}
elseif (Test-Path -LiteralPath $gitMarker -PathType Leaf) {
    $gitSpec = [System.IO.File]::ReadAllText($gitMarker).Trim()
    if (-not $gitSpec.StartsWith('gitdir: ', [System.StringComparison]::Ordinal)) {
        throw 'worktree Git metadata is invalid'
    }
    $gitDirectory = $gitSpec.Substring(8).Trim()
    if (-not [System.IO.Path]::IsPathRooted($gitDirectory)) {
        $gitDirectory = Join-Path $repoRoot $gitDirectory
    }
    $gitDirectory = [System.IO.Path]::GetFullPath($gitDirectory)
}
else {
    throw 'Git metadata is missing'
}

$riskFilePattern = [regex]::new('(?i)(\.(?:pem|key|p12|pfx|pkcs12|jks|keystore|der|crt|cer|csr)$|(?:^|/)(?:id_(?:rsa|dsa|ecdsa|ed25519)|credentials|secrets?\.json)$)')
$privateHandlePattern = [regex]::new(('chen' + '21019'), [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)
$privateAddressPattern = [regex]::new(('10' + '\.' + '0' + '\.' + '0' + '\.' + '125'))
$personalEmailProviders = @(
    ('g' + 'mail\.com'),
    ('out' + 'look\.com'),
    ('hot' + 'mail\.com'),
    ('ya' + 'hoo\.(?:com|com\.tw|co\.uk)'),
    ('i' + 'cloud\.com'),
    ('proton' + '(?:mail)?\.com')
)
$personalEmailPattern = [regex]::new('@(?:' + ($personalEmailProviders -join '|') + ')\b', [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)
$homePathPattern = [regex]::new('(?:[A-Za-z]:[\\/]' + 'Users' + '[\\/][^\\/\s]+|/' + 'home' + '/[^/\s]+|/' + 'Users' + '/[^/\s]+)', [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)
$repositoryPattern = [regex]::new(('git' + 'hub\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+'), [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)
$allowedNamespacePrefix = ('git' + 'hub.com/' + 'PastureStack' + '/').ToLowerInvariant()
$legacyNameLower = 'ran' + 'cher'
$legacyNameTitle = 'Ran' + 'cher'
$legacyVendorLower = 'su' + 'se'
$legacyVendorTitle = 'SU' + 'SE'
$historicalRepository = 'git' + 'hub.com/' + $legacyNameLower + '/storage'
$legacyBrandPattern = [regex]::new('(?i)\b(?:' + $legacyNameLower + '|' + $legacyVendorLower + ')\b')
$readmeDisclaimer = 'PastureStack is an independent community effort to preserve, audit, and modernize the {0} 1.6 ecosystem. It is not affiliated with or endorsed by {0} Labs or {1}.' -f $legacyNameTitle, $legacyVendorTitle
function Assert-NativeSuccess {
    param([Parameter(Mandatory = $true)][string]$Step)
    if ($LASTEXITCODE -ne 0) {
        throw "$Step failed with exit code $LASTEXITCODE"
    }
}

function Invoke-RepoGit {
    & git "--git-dir=$gitDirectory" "--work-tree=$repoRoot" @args
}

function Assert-FileDigest {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][long]$ExpectedBytes,
        [Parameter(Mandatory = $true)][string]$ExpectedHash
    )
    $item = Get-Item -LiteralPath $Path
    if ($item.Length -ne $ExpectedBytes) {
        throw "$Path size mismatch: $($item.Length) != $ExpectedBytes"
    }
    $actualHash = (Get-FileHash -LiteralPath $item.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -cne $ExpectedHash) {
        throw "$Path SHA-256 mismatch: $actualHash"
    }
}

function Assert-AllControlsDisabled {
    param([Parameter(Mandatory = $true)]$Controls)
    $properties = @($Controls.PSObject.Properties)
    if ($properties.Count -ne 14) {
        throw "disabled-control object has $($properties.Count) fields instead of 14"
    }
    $enabled = @($properties | Where-Object { $_.Value -ne $false })
    if ($enabled.Count -ne 0) {
        throw "runtime controls are enabled: $($enabled.Name -join ', ')"
    }
}

function Assert-PublicCurrentTree {
    Assert-FileDigest -Path 'LICENSE' -ExpectedBytes 10351 -ExpectedHash 'eb3d7b5485466acbd81f2b496f595ab637d2792e268206b27d99e793bdb67549'
    Assert-FileDigest -Path 'LICENSES/GO-LICENSE' -ExpectedBytes 1453 -ExpectedHash '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'
    Assert-FileDigest -Path 'LICENSES/GO-PATENTS' -ExpectedBytes 1303 -ExpectedHash '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'

    $untrackedFiles = @(Invoke-RepoGit -c core.quotepath=false ls-files --others --exclude-standard)
    Assert-NativeSuccess 'untracked file inventory'
    if ($untrackedFiles.Count -ne 0) {
        throw "untracked public files must be staged before validation: $($untrackedFiles -join ', ')"
    }

    $trackedFiles = @(Invoke-RepoGit -c core.quotepath=false ls-files)
    Assert-NativeSuccess 'tracked file inventory'
    if ($trackedFiles.Count -eq 0) {
        throw 'tracked file inventory is empty'
    }
    foreach ($requiredScript in @('scripts/validate.ps1', 'scripts/validate.sh')) {
        if ($trackedFiles -cnotcontains $requiredScript) {
            throw "validation script is not tracked: $requiredScript"
        }
    }

    $readmeBrandLines = 0
    $originBrandLines = 0
    $compatibilityBrandLines = 0
    $runtimeCompatibilityBrandLines = 0
    $emptyFile = Join-Path $script:tempRoot 'empty'
    [System.IO.File]::WriteAllBytes($emptyFile, [byte[]]::new(0))
    $repoPrefix = $repoRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar

    foreach ($trackedFile in $trackedFiles) {
        if ([string]::IsNullOrWhiteSpace($trackedFile)) {
            throw 'tracked file inventory contains an empty path'
        }
        $relativePath = $trackedFile.Replace('\', '/')
        $fullPath = [System.IO.Path]::GetFullPath((Join-Path $repoRoot $relativePath))
        if (-not $fullPath.StartsWith($repoPrefix, [System.StringComparison]::OrdinalIgnoreCase) -or -not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
            throw "tracked path is missing or escaped the repository: $relativePath"
        }
        if ($riskFilePattern.IsMatch($relativePath)) {
            throw "risk filename is not allowed in the public tree: $relativePath"
        }

        $numstat = (Invoke-RepoGit --no-pager diff --no-index --no-textconv --numstat -- $emptyFile $fullPath 2>$null | Out-String)
        $diffExit = $LASTEXITCODE
        if ($diffExit -ne 0 -and $diffExit -ne 1) {
            throw "binary classification failed for tracked path: $relativePath"
        }
        if ($numstat -match '(?m)^-\s+-\s+') {
            throw "NUL/binary tracked file is not allowed: $relativePath"
        }

        $content = [System.IO.File]::ReadAllText($fullPath)
        if ($privateHandlePattern.IsMatch($content) -or $privateAddressPattern.IsMatch($content)) {
            throw "private operator identifier or address found in tracked content: $relativePath"
        }

        if ($relativePath.StartsWith('runtime/nfs/vendor/', [System.StringComparison]::Ordinal)) {
            # Third-party sources retain their original authors, repository
            # URLs, and historical names. Exact operator identifiers and
            # private addresses were checked above.
            continue
        }

        $runtimeCompatibilityFile =
            ($relativePath.StartsWith('runtime/nfs/', [System.StringComparison]::Ordinal) -and
             $relativePath.EndsWith('.go', [System.StringComparison]::Ordinal)) -or
            $relativePath -ceq 'runtime/nfs/image/common/update-control-plane-ca'

        if ($personalEmailPattern.IsMatch($content)) {
            throw "generic personal email provider found in tracked content: $relativePath"
        }
        if ($homePathPattern.IsMatch($content)) {
            throw "user home path found in tracked content: $relativePath"
        }
        foreach ($match in $repositoryPattern.Matches($content)) {
            $repository = $match.Value.ToLowerInvariant()
            if ($repository.StartsWith($allowedNamespacePrefix, [System.StringComparison]::Ordinal)) {
                continue
            }
            if (($relativePath -ceq 'ORIGIN.md' -or $relativePath -ceq 'README.md') -and
                $repository -ceq $historicalRepository) {
                continue
            }
            if ($runtimeCompatibilityFile) {
                $allowedRuntimePrefixes = @(
                    ('git' + 'hub.com/docker/'),
                    ('git' + 'hub.com/pkg/'),
                    ('git' + 'hub.com/' + $legacyNameLower + '/'),
                    ('git' + 'hub.com/sirupsen/'),
                    ('git' + 'hub.com/urfave/')
                )
                if (@($allowedRuntimePrefixes | Where-Object {
                    $repository.StartsWith($_, [System.StringComparison]::Ordinal)
                }).Count -gt 0) {
                    continue
                }
            }
            throw "non-allowlisted GitHub namespace found in tracked content: $relativePath"
        }

        foreach ($line in [System.IO.File]::ReadAllLines($fullPath)) {
            if (-not $legacyBrandPattern.IsMatch($line)) {
                continue
            }
            if ($relativePath -ceq 'README.md') {
                $readmeBrandLines++
                continue
            }
            if ($relativePath -ceq 'ORIGIN.md') {
                $originBrandLines++
                continue
            }
            if ($relativePath -ceq 'COMPATIBILITY.md') {
                $compatibilityBrandLines++
                continue
            }
            if ($runtimeCompatibilityFile) {
                $runtimeCompatibilityBrandLines++
                continue
            }
            throw "uncontrolled historical brand content found in tracked path: $relativePath"
        }
    }

    if (-not ([System.IO.File]::ReadAllLines((Join-Path $repoRoot 'README.md')) -ccontains $readmeDisclaimer)) {
        throw 'independence disclaimer is missing or changed'
    }
    if ($readmeBrandLines -ne 2 -or
        $originBrandLines -ne 2 -or
        $compatibilityBrandLines -ne 2 -or
        $runtimeCompatibilityBrandLines -ne 8) {
        throw "historical/compatibility exceptions changed: README=$readmeBrandLines ORIGIN=$originBrandLines COMPATIBILITY=$compatibilityBrandLines RUNTIME=$runtimeCompatibilityBrandLines"
    }
    Write-Host "Public current-tree gate passed: $($trackedFiles.Count) tracked text files; risk filenames=0; controlled historical/compatibility lines=14"
}

function Assert-PublicBinary {
    param([Parameter(Mandatory = $true)][string]$Path)
    $binaryText = [System.Text.Encoding]::Latin1.GetString([System.IO.File]::ReadAllBytes($Path))
    if ($privateHandlePattern.IsMatch($binaryText) -or $privateAddressPattern.IsMatch($binaryText)) {
        throw 'compiled binary contains a private operator identifier or address'
    }
    if ($personalEmailPattern.IsMatch($binaryText)) {
        throw 'compiled binary contains a generic personal email provider'
    }
    if ($homePathPattern.IsMatch($binaryText)) {
        throw 'compiled binary contains a user home path'
    }
    foreach ($match in $repositoryPattern.Matches($binaryText)) {
        if (-not $match.Value.ToLowerInvariant().StartsWith($allowedNamespacePrefix, [System.StringComparison]::Ordinal)) {
            throw "compiled binary contains a non-allowlisted GitHub namespace: $($match.Value)"
        }
    }
    if ($legacyBrandPattern.IsMatch($binaryText)) {
        throw 'compiled binary contains a historical brand'
    }
}

function Assert-CLIExamples {
    param([Parameter(Mandatory = $true)][string]$Binary)

    $capabilitiesJSON = (& $Binary capabilities --locale en-US | Out-String)
    Assert-NativeSuccess 'capabilities example'
    $capabilities = $capabilitiesJSON | ConvertFrom-Json
    $expectedDrivers = @('aliyun-block', 'aws-ebs', 'aws-efs', 'ceph-rbd', 'longhorn', 'loop', 'nfs')
    $actualDrivers = @($capabilities.drivers | ForEach-Object { $_.driver })
    if (@(Compare-Object -ReferenceObject $expectedDrivers -DifferenceObject $actualDrivers).Count -ne 0) {
        throw "capability driver set changed: $($actualDrivers -join ', ')"
    }
    $expectedDelegated = @('PastureStack/secrets-flexvolume-plugin', 'PastureStack/vault-secrets-bridge')
    $actualDelegated = @($capabilities.delegatedComponents | ForEach-Object { $_.repository })
    if (@(Compare-Object -ReferenceObject $expectedDelegated -DifferenceObject $actualDelegated).Count -ne 0) {
        throw "delegated component set changed: $($actualDelegated -join ', ')"
    }
    if (@($capabilities.delegatedComponents | Where-Object { $_.included -ne $false }).Count -ne 0) {
        throw 'a delegated component is reported as included'
    }
    Assert-AllControlsDisabled -Controls $capabilities.controls

    $nfsInput = Get-Content -LiteralPath 'examples/nfs-create.json' -Raw
    $validationJSON = ($nfsInput | & $Binary validate --locale en-US | Out-String)
    Assert-NativeSuccess 'validate example'
    $validation = $validationJSON | ConvertFrom-Json
    if ($validation.valid -ne $true -or $validation.driver -cne 'nfs') {
        throw 'NFS validation example returned an unexpected result'
    }
    Assert-AllControlsDisabled -Controls $validation.controls

    $blockInput = Get-Content -LiteralPath 'examples/aws-ebs-create.json' -Raw
    $planJSONA = ($blockInput | & $Binary plan --locale en-US | Out-String)
    Assert-NativeSuccess 'first plan example'
    $planJSONB = ($blockInput | & $Binary plan --locale en-US | Out-String)
    Assert-NativeSuccess 'second plan example'
    if ($planJSONA -cne $planJSONB) {
        throw 'plan example output is not deterministic'
    }
    if ($planJSONA.Contains('kms-policy-a')) {
        throw 'plan example echoed an external reference'
    }
    $plan = $planJSONA | ConvertFrom-Json
    if ($plan.executable -ne $false -or $plan.effect -cne 'none' -or $plan.steps.Count -ne 2) {
        throw 'plan example violated the non-execution contract'
    }
    foreach ($step in $plan.steps) {
        if ($step.status -cne 'blocked' -or -not $step.action.StartsWith('would-', [System.StringComparison]::Ordinal)) {
            throw 'plan example contains a non-blocked step'
        }
    }
    $irreversibleGate = @($plan.gates | Where-Object { $_.name -ceq 'irreversible' })
    if ($irreversibleGate.Count -ne 1 -or $irreversibleGate[0].status -cne 'blocked') {
        throw 'plan example did not keep the irreversible gate blocked'
    }
    Assert-AllControlsDisabled -Controls $plan.controls
}

$script:tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("pasturestack-storage-plugins-" + [System.Guid]::NewGuid().ToString('N'))
[void](New-Item -ItemType Directory -Path $script:tempRoot)

Push-Location $repoRoot
try {
    $goFiles = @(Get-ChildItem -LiteralPath 'cmd', 'internal', 'locales' -Recurse -File -Filter '*.go' | ForEach-Object { $_.FullName })
    $unformatted = @(& $gofmtCommand.Source -l $goFiles)
    Assert-NativeSuccess 'gofmt check'
    if ($unformatted.Count -ne 0) {
        throw "gofmt check failed: $($unformatted -join ', ')"
    }

    Assert-PublicCurrentTree

    & $goCommand.Source test -shuffle=on -count=3 ./...
    Assert-NativeSuccess 'go test'

    $cgoEnabled = (& $goCommand.Source env CGO_ENABLED | Out-String).Trim()
    Assert-NativeSuccess 'go env CGO_ENABLED'
    if ((Get-Command gcc -ErrorAction SilentlyContinue) -and $cgoEnabled -ceq '1') {
        & $goCommand.Source test -race -shuffle=on -count=1 ./...
        Assert-NativeSuccess 'go test -race'
    }
    else {
        Write-Host 'SKIP: go test -race (gcc or CGO support is unavailable)'
    }

    & $goCommand.Source vet ./...
    Assert-NativeSuccess 'go vet'

    & $goCommand.Source mod verify
    Assert-NativeSuccess 'go mod verify'

    $binaryA = Join-Path $script:tempRoot 'storage-plugins-a.exe'
    $binaryB = Join-Path $script:tempRoot 'storage-plugins-b.exe'
    & $goCommand.Source build -trimpath -buildvcs=false -o $binaryA ./cmd/storage-plugins
    Assert-NativeSuccess 'first reproducible build'
    & $goCommand.Source build -trimpath -buildvcs=false -o $binaryB ./cmd/storage-plugins
    Assert-NativeSuccess 'second reproducible build'

    $hashA = (Get-FileHash -LiteralPath $binaryA -Algorithm SHA256).Hash.ToLowerInvariant()
    $hashB = (Get-FileHash -LiteralPath $binaryB -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($hashA -cne $hashB) {
        throw "reproducible build failed: $hashA != $hashB"
    }

    Assert-PublicBinary -Path $binaryA
    Assert-CLIExamples -Binary $binaryA
    Write-Host "Validation passed; reproducible SHA-256: $hashA"
}
finally {
    Pop-Location
    if (Test-Path -LiteralPath $script:tempRoot) {
        Remove-Item -LiteralPath $script:tempRoot -Recurse -Force
    }
}
