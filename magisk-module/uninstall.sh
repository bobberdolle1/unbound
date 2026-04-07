#!/system/bin/sh
# ============================================================================
# Unbound Core — iptables Cleanup Script
# ============================================================================
# Removes all iptables rules and chains created by the Unbound module.
# Run during module uninstall or manual stop.
# ============================================================================

IPTABLES_MODE="${1:-iptables}"

log_cleanup() {
    echo "[CLEANUP] $1"
}

cleanup_iptables() {
    log_cleanup "Cleaning up iptables rules"

    local IPT="iptables"
    local IPT6="ip6tables"

    # Flush and delete Unbound chains
    $IPT -t mangle -F UNBOUND_OUTPUT 2>/dev/null
    $IPT -t mangle -F UNBOUND_FORWARD 2>/dev/null
    $IPT -t mangle -D OUTPUT -j UNBOUND_OUTPUT 2>/dev/null
    $IPT -t mangle -D FORWARD -j UNBOUND_FORWARD 2>/dev/null
    $IPT -t mangle -X UNBOUND_OUTPUT 2>/dev/null
    $IPT -t mangle -X UNBOUND_FORWARD 2>/dev/null

    # IPv6 cleanup
    $IPT6 -t mangle -F UNBOUND_OUTPUT6 2>/dev/null
    $IPT6 -t mangle -D OUTPUT -j UNBOUND_OUTPUT6 2>/dev/null
    $IPT6 -t mangle -X UNBOUND_OUTPUT6 2>/dev/null

    log_cleanup "iptables rules removed"
}

cleanup_nftables() {
    log_cleanup "Cleaning up nftables rules"
    nft delete table inet unbound 2>/dev/null
    log_cleanup "nftables rules removed"
}

# Execute cleanup
case "$IPTABLES_MODE" in
    nftables)
        cleanup_nftables
        ;;
    *)
        cleanup_iptables
        ;;
esac

log_cleanup "Cleanup complete"
