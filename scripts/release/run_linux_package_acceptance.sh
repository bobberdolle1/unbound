#!/usr/bin/env bash
# Exercise a packaged Linux CLI candidate with ownership-scoped firewall evidence.
set -euo pipefail

if [[ $# -ne 2 ]]; then
    printf 'usage: %s ARCHIVE EVIDENCE_DIRECTORY\n' "$0" >&2
    exit 2
fi

ARCHIVE="$1"
EVIDENCE="$2"
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 && "$(id -u)" == 0 ]] || {
    printf 'LINUX_PHYSICAL_ACCEPTANCE=ENVIRONMENT_UNAVAILABLE requires Linux x86_64 root\n' >&2
    exit 2
}
command -v nft >/dev/null || { printf 'LINUX_PHYSICAL_ACCEPTANCE=ENVIRONMENT_UNAVAILABLE nft is required for the primary path\n' >&2; exit 2; }
command -v curl >/dev/null || { printf 'LINUX_PHYSICAL_ACCEPTANCE=ENVIRONMENT_UNAVAILABLE curl is required for the IPv4 packet-path probe\n' >&2; exit 2; }
[[ -f "$ARCHIVE" ]] || { printf 'candidate archive is missing: %s\n' "$ARCHIVE" >&2; exit 1; }

mkdir -p "$EVIDENCE"
work="$(mktemp -d)"
foreign="unbound_foreign_acceptance_${$}"
trap 'nft delete table inet "$foreign" >/dev/null 2>&1 || true; rm -rf "$work"' EXIT
sha256sum "$ARCHIVE" > "$EVIDENCE/archive.sha256"
uname -srmo > "$EVIDENCE/uname.txt"
{ . /etc/os-release 2>/dev/null || true; printf 'ID=%s\nVERSION_ID=%s\n' "${ID:-unknown}" "${VERSION_ID:-unknown}"; } > "$EVIDENCE/os-release.txt"
nft --version > "$EVIDENCE/nft-version.txt"
nft list ruleset > "$EVIDENCE/nft-before.txt"
nft list tables > "$EVIDENCE/nft-tables-before.txt"
ps -eo pid=,args= | grep '[n]fqws2' > "$EVIDENCE/nfqws2-before.txt" || true

tar -xzf "$ARCHIVE" -C "$work"
package="$(find "$work" -mindepth 1 -maxdepth 1 -type d -name 'unbound-*-linux-amd64' -print -quit)"
[[ -n "$package" ]] || { printf 'candidate archive lacks expected root\n' >&2; exit 1; }
(
    cd "$package"
    sha256sum -c BUNDLE_SHA256SUMS.txt > "$EVIDENCE/bundle-check.txt"
    ./unbound --version --json > "$EVIDENCE/identity.json"
    ./unbound --list-profiles --json > "$EVIDENCE/profiles.json"
)

# This harmless foreign table proves cleanup remains ownership-scoped.
nft add table inet "$foreign"
run_profile() {
    local signal="$1" run="$2" output="$EVIDENCE/$run.stdout" error="$EVIDENCE/$run.stderr" table="" nfqws_pid=""
    "$package/unbound" --cli --profile ultimate >"$output" 2>"$error" &
    local parent=$!
    for _ in $(seq 1 20); do
        if grep -F 'Engine started successfully' "$output" >/dev/null 2>&1; then
            break
        fi
        if ! kill -0 "$parent" 2>/dev/null; then
            wait "$parent" || true
            printf 'packaged CLI failed before active runtime; see %s\n' "$error" >&2
            return 1
        fi
        sleep 1
    done
    grep -F 'Engine started successfully' "$output" >/dev/null || { kill -TERM "$parent" 2>/dev/null || true; wait "$parent" || true; return 1; }
    nft list tables > "$EVIDENCE/$run.nft-tables-active.txt"
    table="$(comm -13 <(awk '$1 == "table" && $2 == "inet" && $3 ~ /^unbound_[0-9]+_[0-9]+_[0-9]+$/ { print $3 }' "$EVIDENCE/nft-tables-before.txt" | sort) <(awk '$1 == "table" && $2 == "inet" && $3 ~ /^unbound_[0-9]+_[0-9]+_[0-9]+$/ { print $3 }' "$EVIDENCE/$run.nft-tables-active.txt" | sort) | tail -n 1)"
    [[ -n "$table" ]] || { kill -TERM "$parent" 2>/dev/null || true; wait "$parent" || true; return 1; }
    nft list table inet "$table" > "$EVIDENCE/$run.nft-active.txt"
    grep -F 'queue num 200 bypass' "$EVIDENCE/$run.nft-active.txt" >/dev/null
    grep -F 'counter' "$EVIDENCE/$run.nft-active.txt" >/dev/null
    ps -eo pid=,args= | grep '[n]fqws2' > "$EVIDENCE/$run.nfqws2-active.txt" || true
    nfqws_pid="$(comm -13 <(awk '{print $1}' "$EVIDENCE/nfqws2-before.txt" | sort) <(awk '{print $1}' "$EVIDENCE/$run.nfqws2-active.txt" | sort) | head -n 1)"
    [[ -n "$nfqws_pid" ]] || { kill -TERM "$parent" 2>/dev/null || true; wait "$parent" || true; return 1; }
    curl --max-time 8 --silent --show-error --output /dev/null --insecure https://1.1.1.1 > "$EVIDENCE/$run.curl.stdout" 2> "$EVIDENCE/$run.curl.stderr" || true
    nft list table inet "$table" > "$EVIDENCE/$run.nft-after-ipv4.txt"
    grep -E 'counter packets [1-9][0-9]*' "$EVIDENCE/$run.nft-after-ipv4.txt" >/dev/null
    kill -"$signal" "$parent"
    wait "$parent" || true
    sleep 1
    ! nft list table inet "$table" >/dev/null 2>&1
    ! kill -0 "$nfqws_pid" 2>/dev/null
    nft list table inet "$foreign" > "$EVIDENCE/$run.foreign-table.txt"
}

run_profile TERM normal
run_profile INT sigint

# Package manifest rejection occurs before any profile path can add firewall state.
tamper="$work/tampered"
cp -a "$package" "$tamper"
printf 'tampered\n' >> "$tamper/runtime/core_bin/linux/amd64/nfqws2"
if "$tamper/unbound" --version > "$EVIDENCE/tamper.stdout" 2> "$EVIDENCE/tamper.stderr"; then
    printf 'tampered package unexpectedly started\n' >&2
    exit 1
fi
! nft list tables | grep -E '^table inet unbound_[0-9]+_[0-9]+_[0-9]+$' >/dev/null
rm "$tamper/runtime/core_bin/linux/amd64/nfqws2"
if "$tamper/unbound" --version > "$EVIDENCE/missing.stdout" 2> "$EVIDENCE/missing.stderr"; then
    printf 'package with missing nfqws2 unexpectedly started\n' >&2
    exit 1
fi

nft delete table inet "$foreign"
nft list ruleset > "$EVIDENCE/nft-after.txt"
ps -eo pid=,args= | grep '[n]fqws2' > "$EVIDENCE/nfqws2-after.txt" || true
printf '{"schema_version":1,"physical_acceptance":"PASS","ipv4_runtime":"PASS","sigterm_cleanup":"PASS","sigint_cleanup":"PASS","foreign_firewall_state":"PASS","tamper_rejection":"PASS","missing_engine_rejection":"PASS"}\n' > "$EVIDENCE/result.json"
printf 'LINUX_PHYSICAL_ACCEPTANCE=PASS evidence=%s\n' "$EVIDENCE"
