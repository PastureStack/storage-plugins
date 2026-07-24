#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

go_bin=go
gofmt_bin=gofmt
git_marker="$repo_root/.git"
if [ -d "$git_marker" ]; then
    git_dir="$git_marker"
elif [ -f "$git_marker" ]; then
    git_dir=$(sed -n 's/^gitdir: //p' "$git_marker")
    [ -n "$git_dir" ] || {
        printf '%s\n' 'worktree Git metadata is invalid' >&2
        exit 1
    }
    case "$git_dir" in
        /*|[A-Za-z]:[\\/]*)
            ;;
        *)
            git_dir="$repo_root/$git_dir"
            ;;
    esac
else
    printf '%s\n' 'Git metadata is missing' >&2
    exit 1
fi

fail() {
    printf '%s\n' "$1" >&2
    exit 1
}

git_safe() {
    git --git-dir="$git_dir" --work-tree="$repo_root" "$@"
}

hash_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print tolower($1)}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | awk '{print tolower($1)}'
    elif command -v openssl >/dev/null 2>&1; then
        openssl dgst -sha256 "$1" | awk '{print tolower($NF)}'
    else
        fail 'no SHA-256 implementation is available'
    fi
}

assert_file_digest() {
    path=$1
    expected_bytes=$2
    expected_hash=$3
    actual_bytes=$(wc -c < "$path" | tr -d '[:space:]')
    [ "$actual_bytes" = "$expected_bytes" ] || fail "$path size mismatch: $actual_bytes != $expected_bytes"
    actual_hash=$(hash_file "$path")
    [ "$actual_hash" = "$expected_hash" ] || fail "$path SHA-256 mismatch: $actual_hash"
}

assert_public_current_tree() {
    assert_file_digest LICENSE 10351 eb3d7b5485466acbd81f2b496f595ab637d2792e268206b27d99e793bdb67549
    assert_file_digest LICENSES/GO-LICENSE 1453 911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad
    assert_file_digest LICENSES/GO-PATENTS 1303 96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc

    untracked_manifest="$temp_root/untracked-files"
    git_safe -c core.quotepath=false ls-files --others --exclude-standard > "$untracked_manifest"
    [ ! -s "$untracked_manifest" ] || fail "untracked public files must be staged before validation: $(tr '\n' ' ' < "$untracked_manifest")"

    tracked_manifest="$temp_root/tracked-files"
    git_safe -c core.quotepath=false ls-files > "$tracked_manifest"
    [ -s "$tracked_manifest" ] || fail 'tracked file inventory is empty'
    grep -Fxq 'scripts/validate.ps1' "$tracked_manifest" || fail 'PowerShell validation script is not tracked'
    grep -Fxq 'scripts/validate.sh' "$tracked_manifest" || fail 'sh validation script is not tracked'

    empty_file="$temp_root/empty"
    : > "$empty_file"

    risk_file_pattern='(\.(pem|key|p12|pfx|pkcs12|jks|keystore|der|crt|cer|csr)$|(^|/)(id_(rsa|dsa|ecdsa|ed25519)|credentials|secrets?\.json)$)'
    private_handle_pattern='chen''21019'
    private_address_pattern='10\.0\.0\.125'
    personal_email_pattern='@('"g"'mail\.com|'"out"'look\.com|'"hot"'mail\.com|'"ya"'hoo\.(com|com\.tw|co\.uk)|'"i"'cloud\.com|'"proton"'(mail)?\.com)([^[:alnum:]._-]|$)'
    home_path_pattern='([A-Za-z]:[\\/]'"Users"'[\\/][^\\/[:space:]]+|/'"home"'/[^/[:space:]]+|/'"Users"'/[^/[:space:]]+)'
    repository_pattern='git''hub\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+'
    github_host='git''hub.com/'
    allowed_namespace_prefix=$(printf '%s' "$github_host""PastureStack/" | tr '[:upper:]' '[:lower:]')
    legacy_lower='ran''cher'
    legacy_title='Ran''cher'
    vendor_lower='su''se'
    vendor_title='SU''SE'
    historical_repository="$github_host$legacy_lower/storage"
    legacy_brand_pattern='(^|[^[:alnum:]_])('"$legacy_lower"'|'"$vendor_lower"')([^[:alnum:]_]|$)'
    readme_disclaimer="PastureStack is an independent community effort to preserve, audit, and modernize the $legacy_title 1.6 ecosystem. It is not affiliated with or endorsed by $legacy_title Labs or $vendor_title."
    readme_brand_lines=0
    origin_brand_lines=0
    compatibility_brand_lines=0
    runtime_compatibility_brand_lines=0
    tracked_count=0

    while IFS= read -r relative_path || [ -n "$relative_path" ]; do
        [ -n "$relative_path" ] || fail 'tracked file inventory contains an empty path'
        tracked_count=$((tracked_count + 1))
        [ -f "$relative_path" ] || fail "tracked path is missing or not a regular file: $relative_path"
        lower_path=$(printf '%s' "$relative_path" | tr '[:upper:]' '[:lower:]')
        if printf '%s\n' "$lower_path" | grep -Eq -- "$risk_file_pattern"; then
            fail "risk filename is not allowed in the public tree: $relative_path"
        fi

        diff_status=0
        numstat=$(git_safe --no-pager diff --no-index --no-textconv --numstat -- "$empty_file" "$relative_path" 2>/dev/null) || diff_status=$?
        if [ "$diff_status" -ne 0 ] && [ "$diff_status" -ne 1 ]; then
            fail "binary classification failed for tracked path: $relative_path"
        fi
        if printf '%s\n' "$numstat" | grep -Eq '^-[[:space:]]+-[[:space:]]+'; then
            fail "NUL/binary tracked file is not allowed: $relative_path"
        fi

        if grep -Eiq -- "$private_handle_pattern|$private_address_pattern" "$relative_path"; then
            fail "private operator identifier or address found in tracked content: $relative_path"
        fi

        case "$relative_path" in
            runtime/nfs/vendor/*)
                # Preserved third-party sources retain their own authors,
                # repository URLs, and historical names. Exact operator
                # identifiers and private addresses were checked above.
                continue
                ;;
        esac

        runtime_compatibility_file=0
        case "$relative_path" in
            runtime/nfs/*.go|\
            runtime/nfs/image/common/update-control-plane-ca)
                runtime_compatibility_file=1
                ;;
        esac

        if grep -Eiq -- "$personal_email_pattern" "$relative_path"; then
            fail "generic personal email provider found in tracked content: $relative_path"
        fi
        if grep -Eiq -- "$home_path_pattern" "$relative_path"; then
            fail "user home path found in tracked content: $relative_path"
        fi
        repository_matches=$(grep -Eio -- "$repository_pattern" "$relative_path" || true)
        for repository in $repository_matches; do
            repository_lower=$(printf '%s' "$repository" | tr '[:upper:]' '[:lower:]')
            case "$repository_lower" in
                "$allowed_namespace_prefix"*)
                    continue
                    ;;
            esac
            if { [ "$relative_path" = ORIGIN.md ] || [ "$relative_path" = README.md ]; } &&
               [ "$repository_lower" = "$historical_repository" ]; then
                continue
            fi
            if [ "$runtime_compatibility_file" -eq 1 ]; then
                case "$repository_lower" in
                    git''hub.com/docker/*|\
                    git''hub.com/pkg/*|\
                    git''hub.com/ran''cher/*|\
                    git''hub.com/sirupsen/*|\
                    git''hub.com/urfave/*)
                        continue
                        ;;
                esac
            fi
            fail "non-allowlisted GitHub namespace found in tracked content: $relative_path"
        done

        brand_matches="$temp_root/brand-matches-$tracked_count"
        brand_status=0
        grep -Ei -- "$legacy_brand_pattern" "$relative_path" > "$brand_matches" || brand_status=$?
        if [ "$brand_status" -ne 0 ] && [ "$brand_status" -ne 1 ]; then
            fail "historical-brand classification failed for tracked path: $relative_path"
        fi
        while IFS= read -r line || [ -n "$line" ]; do
            if [ "$relative_path" = README.md ]; then
                readme_brand_lines=$((readme_brand_lines + 1))
                continue
            fi
            if [ "$relative_path" = ORIGIN.md ]; then
                origin_brand_lines=$((origin_brand_lines + 1))
                continue
            fi
            if [ "$relative_path" = COMPATIBILITY.md ]; then
                compatibility_brand_lines=$((compatibility_brand_lines + 1))
                continue
            fi
            if [ "$runtime_compatibility_file" -eq 1 ]; then
                runtime_compatibility_brand_lines=$((runtime_compatibility_brand_lines + 1))
                continue
            fi
            fail "uncontrolled historical brand content found in tracked path: $relative_path"
        done < "$brand_matches"
    done < "$tracked_manifest"

    grep -Fxq -- "$readme_disclaimer" README.md ||
        fail 'independence disclaimer is missing or changed'
    [ "$readme_brand_lines" -eq 2 ] || fail "historical brand README exception changed: $readme_brand_lines"
    [ "$origin_brand_lines" -eq 2 ] || fail "historical brand ORIGIN exceptions changed: $origin_brand_lines"
    [ "$compatibility_brand_lines" -eq 2 ] || fail "compatibility literal exceptions changed: $compatibility_brand_lines"
    [ "$runtime_compatibility_brand_lines" -eq 10 ] ||
        fail "runtime compatibility literal exceptions changed: $runtime_compatibility_brand_lines"
    printf 'Public current-tree gate passed: %s tracked text files; risk filenames=0; controlled historical/compatibility lines=16\n' "$tracked_count"
}

assert_public_binary() {
    binary_path=$1
    if LC_ALL=C grep -aEiq -- "$private_handle_pattern|$private_address_pattern" "$binary_path"; then
        fail 'compiled binary contains a private operator identifier or address'
    fi
    if LC_ALL=C grep -aEiq -- "$personal_email_pattern" "$binary_path"; then
        fail 'compiled binary contains a generic personal email provider'
    fi
    if LC_ALL=C grep -aEiq -- "$home_path_pattern" "$binary_path"; then
        fail 'compiled binary contains a user home path'
    fi
    binary_repositories=$(LC_ALL=C grep -aEio -- "$repository_pattern" "$binary_path" || true)
    for repository in $binary_repositories; do
        repository_lower=$(printf '%s' "$repository" | tr '[:upper:]' '[:lower:]')
        case "$repository_lower" in
            "$allowed_namespace_prefix"*)
                continue
                ;;
        esac
        fail "compiled binary contains a non-allowlisted GitHub namespace: $repository"
    done
    if LC_ALL=C grep -aEiq -- "$legacy_brand_pattern" "$binary_path"; then
        fail 'compiled binary contains a historical brand'
    fi
}

temp_root=$(mktemp -d)
trap 'rm -rf -- "$temp_root"' EXIT HUP INT TERM

unformatted=$(find cmd internal locales -type f -name '*.go' -exec "$gofmt_bin" -l {} +)
if [ -n "$unformatted" ]; then
    printf '%s\n' 'gofmt check failed:' "$unformatted" >&2
    exit 1
fi

assert_public_current_tree

"$go_bin" test -shuffle=on -count=3 ./...

cgo_enabled=$("$go_bin" env CGO_ENABLED)
if command -v gcc >/dev/null 2>&1 && [ "$cgo_enabled" = 1 ]; then
    "$go_bin" test -race -shuffle=on -count=1 ./...
else
    printf '%s\n' 'SKIP: go test -race (gcc or CGO support is unavailable)'
fi

"$go_bin" vet ./...
"$go_bin" mod verify

binary_a="$temp_root/storage-plugins-a"
binary_b="$temp_root/storage-plugins-b"
"$go_bin" build -trimpath -buildvcs=false -o "$binary_a" ./cmd/storage-plugins
"$go_bin" build -trimpath -buildvcs=false -o "$binary_b" ./cmd/storage-plugins
cmp -s "$binary_a" "$binary_b" || fail 'reproducible build bytes differ'

assert_public_binary "$binary_a"

disabled_controls='{"network":false,"cloudApi":false,"dockerSocket":false,"hostSocket":false,"hostPath":false,"privileged":false,"mount":false,"unmount":false,"format":false,"delete":false,"credentialRead":false,"secretRead":false,"stateWrite":false,"execution":false}'

capabilities_output="$temp_root/capabilities.json"
"$binary_a" capabilities --locale en-US </dev/null > "$capabilities_output"
grep -q '"locale":"en-US"' "$capabilities_output" || fail 'capabilities locale is invalid'
for driver in aliyun-block aws-ebs aws-efs ceph-rbd longhorn loop nfs; do
    grep -q "\"driver\":\"$driver\"" "$capabilities_output" || fail "capabilities omitted driver: $driver"
done
driver_count=$(grep -o '"driver":' "$capabilities_output" | wc -l | tr -d '[:space:]')
[ "$driver_count" -eq 7 ] || fail "capabilities driver count changed: $driver_count"
for component in PastureStack/secrets-flexvolume-plugin PastureStack/vault-secrets-bridge; do
    grep -q "\"repository\":\"$component\",\"included\":false" "$capabilities_output" || fail "delegated component contract changed: $component"
done
included_count=$(grep -o '"included":false' "$capabilities_output" | wc -l | tr -d '[:space:]')
[ "$included_count" -eq 2 ] || fail "delegated component count changed: $included_count"
grep -q "\"controls\":$disabled_controls" "$capabilities_output" || fail 'capabilities enabled or omitted a runtime control'

validation_output="$temp_root/validation.json"
"$binary_a" validate --locale en-US < examples/nfs-create.json > "$validation_output"
grep -q '"valid":true' "$validation_output" || fail 'NFS validation example is invalid'
grep -q '"driver":"nfs"' "$validation_output" || fail 'NFS validation example returned the wrong driver'
grep -q "\"controls\":$disabled_controls" "$validation_output" || fail 'validation enabled or omitted a runtime control'

plan_a="$temp_root/plan-a.json"
plan_b="$temp_root/plan-b.json"
"$binary_a" plan --locale en-US < examples/aws-ebs-create.json > "$plan_a"
"$binary_a" plan --locale en-US < examples/aws-ebs-create.json > "$plan_b"
cmp -s "$plan_a" "$plan_b" || fail 'plan example output is not deterministic'
grep -q '"executable":false,"effect":"none"' "$plan_a" || fail 'plan example violated the non-execution contract'
grep -q '"action":"would-create-resource","status":"blocked"' "$plan_a" || fail 'plan create step is not blocked'
grep -q '"action":"would-format-filesystem","status":"blocked","irreversible":true' "$plan_a" || fail 'plan format step is not blocked'
grep -q '"name":"irreversible","status":"blocked"' "$plan_a" || fail 'irreversible plan gate is not blocked'
step_count=$(grep -o '"action":"would-' "$plan_a" | wc -l | tr -d '[:space:]')
[ "$step_count" -eq 2 ] || fail "plan step count changed: $step_count"
grep -q "\"controls\":$disabled_controls" "$plan_a" || fail 'plan enabled or omitted a runtime control'
if grep -q 'kms-policy-a' "$plan_a"; then
    fail 'plan example echoed an external reference'
fi

printf 'Validation passed; reproducible SHA-256: %s\n' "$(hash_file "$binary_a")"
