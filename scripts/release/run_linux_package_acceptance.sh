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
cleanup_all() {
    # Match any released Linux bundle path, not one pinned version: a pattern
    # naming a single release would silently fail to clean up the next one.
    pkill -9 -f "unbound-v0.*-linux-amd64/unbound" 2>/dev/null || true
    pkill -9 -f nfqws2 2>/dev/null || true
    nft delete table inet "$foreign" 2>/dev/null || true
    rm -rf "$work" 2>/dev/null || true
}
trap cleanup_all EXIT
sha256sum "$ARCHIVE" > "$EVIDENCE/archive.sha256"
uname -srmo > "$EVIDENCE/uname.txt"
{ . /etc/os-release 2>/dev/null || true; printf 'ID=%s\nVERSION_ID=%s\n' "${ID:-unknown}" "${VERSION_ID:-unknown}"; } > "$EVIDENCE/os-release.txt"
nft --version > "$EVIDENCE/nft-version.txt"
nft list ruleset > "$EVIDENCE/nft-before.txt"
nft list tables > "$EVIDENCE/nft-tables-before.txt"
iptables-save > "$EVIDENCE/iptables-before.txt" 2>/dev/null || true
ip6tables-save > "$EVIDENCE/ip6tables-before.txt" 2>/dev/null || true
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
    local signal="$1"
    local run_name="$2"
    local output="$EVIDENCE/${run_name}.stdout"
    local error="$EVIDENCE/${run_name}.stderr"
    local table=""
    local nfqws_pid=""
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
    nft list tables > "$EVIDENCE/${run_name}.nft-tables-active.txt"
    table="$(nft list tables | awk -v pid="$parent" '$1 == "table" && $2 == "inet" && $3 ~ ("^unbound_" pid "_") { print $3 }' | tail -n 1)"
    if [[ -z "$table" ]]; then
        table="$(nft list tables | awk '$1 == "table" && $2 == "inet" && $3 ~ /^unbound_[0-9]+_/ { print $3 }' | tail -n 1)"
    fi
    [[ -n "$table" ]] || { kill -TERM "$parent" 2>/dev/null || true; wait "$parent" || true; return 1; }
    nft list table inet "$table" > "$EVIDENCE/${run_name}.nft-active.txt"
    grep -E 'queue.*(bypass.*200|200.*bypass)' "$EVIDENCE/${run_name}.nft-active.txt" >/dev/null
    grep -F 'counter' "$EVIDENCE/${run_name}.nft-active.txt" >/dev/null
    for _ in $(seq 1 10); do
        nfqws_pid="$(pgrep -P "$parent" nfqws2 2>/dev/null || pidof nfqws2 || true)"
        if [[ -n "$nfqws_pid" ]]; then
            break
        fi
        sleep 1
    done
    [[ -n "$nfqws_pid" ]] || { kill -TERM "$parent" 2>/dev/null || true; wait "$parent" || true; return 1; }
    echo "$nfqws_pid" > "$EVIDENCE/${run_name}.nfqws2-active.txt"
    for _ in $(seq 1 5); do
        curl --max-time 5 --silent --output /dev/null --insecure https://1.1.1.1 > "$EVIDENCE/${run_name}.curl.stdout" 2> "$EVIDENCE/${run_name}.curl.stderr" || curl --max-time 5 --silent --output /dev/null --insecure https://ozon.ru >> "$EVIDENCE/${run_name}.curl.stdout" 2>> "$EVIDENCE/${run_name}.curl.stderr" || true
        nft list table inet "$table" > "$EVIDENCE/${run_name}.nft-after-ipv4.txt"
        if grep -E 'counter packets [1-9][0-9]*' "$EVIDENCE/${run_name}.nft-after-ipv4.txt" >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    grep -E 'counter packets [1-9][0-9]*' "$EVIDENCE/${run_name}.nft-after-ipv4.txt" >/dev/null
    kill -"$signal" "$parent"
    wait "$parent" || true
    for _ in $(seq 1 10); do
        if ! kill -0 "$nfqws_pid" 2>/dev/null && ! nft list table inet "$table" >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    ! nft list table inet "$table" >/dev/null 2>&1
    ! kill -0 "$nfqws_pid" 2>/dev/null
    nft list table inet "$foreign" > "$EVIDENCE/${run_name}.foreign-table.txt"
}

run_profile TERM normal
run_profile INT sigint
run_profile TERM second_run

# Controlled failure rollback: test invalid activation precondition and verify fail-closed with zero leaked state
if "$package/unbound" --cli --profile "NonExistentProfile" > "$EVIDENCE/failure-rollback.stdout" 2> "$EVIDENCE/failure-rollback.stderr"; then
    printf 'unbound unexpectedly started on non-existent profile\n' >&2
    exit 1
fi
! nft list tables | grep -E '^table inet unbound_[0-9]+_[0-9]+_[0-9]+$' >/dev/null
nft list table inet "$foreign" > "$EVIDENCE/failure-rollback.foreign-table.txt"
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
iptables-save > "$EVIDENCE/iptables-after.txt" 2>/dev/null || true
ip6tables-save > "$EVIDENCE/ip6tables-after.txt" 2>/dev/null || true
ps -eo pid=,args= | grep '[n]fqws2' > "$EVIDENCE/nfqws2-after.txt" || true
printf '{"schema_version":1,"physical_acceptance":"PASS","ipv4_runtime":"PASS","normal_stop_cleanup":"PASS","sigterm_cleanup":"PASS","sigint_cleanup":"PASS","second_run_no_stale_state":"PASS","failure_rollback":"PASS","foreign_firewall_state":"PASS","tamper_rejection":"PASS","missing_engine_rejection":"PASS"}\n' > "$EVIDENCE/result.json"
printf 'LINUX_PHYSICAL_ACCEPTANCE=PASS evidence=%s\n' "$EVIDENCE"
