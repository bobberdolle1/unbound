#!/system/bin/sh
# ============================================================================
# Unbound Core — Magisk/KernelSU Module Service Script
# ============================================================================
# This script is executed by Magisk/KernelSU during boot (late_start service).
# It starts the nfqws daemon and configures iptables/nftables rules for
# system-wide transparent DPI bypass.
#
# Features:
# - Routes all TCP traffic (ports 80, 443) through NFQUEUE
# - Forwards hotspot (tethering) traffic via FORWARD chain
# - Per-app routing using iptables owner module (UID filtering)
# - Supports both iptables and nftables backends
# - Integrates with Unbound APK via broadcast intents
# ============================================================================

MODDIR="${0%/*}"
PIDFILE="$MODDIR/nfqws.pid"
LOGFILE="$MODDIR/log/nfqws.log"
CONFIGFILE="$MODDIR/etc/unbound.conf"

# ============================================================================
# Configuration Loading
# ============================================================================

# Default values
NFQUEUE_NUM=200
NFQWS_BIN="$MODDIR/bin/nfqws"
NFQWS_ARGS=""
IPTABLES_MODE="iptables"  # iptables or nftables
FILTER_PORTS="80,443"
FILTER_CONNBYTES_OUT="1:6"
FILTER_CONNBYTES_IN="1:3"
FWMARK="0x40000000"
FWMARK_MASK="0x40000000"
ENABLE_IPV6=true
ENABLE_HOTSPOT=true
EXCLUDED_UIDS=""
DEBUG_MODE=false

# Load user config
if [ -f "$CONFIGFILE" ]; then
    while IFS='=' read -r key value; do
        # Skip comments and empty lines
        case "$key" in
            \#*|"") continue ;;
        esac

        # Trim whitespace
        key=$(echo "$key" | tr -d '[:space:]')
        value=$(echo "$value" | tr -d '[:space:]')

        case "$key" in
            nfqueue_num) NFQUEUE_NUM="$value" ;;
            nfqws_path) NFQWS_BIN="$value" ;;
            nfqws_args) NFQWS_ARGS="$value" ;;
            iptables_mode) IPTABLES_MODE="$value" ;;
            filter_ports) FILTER_PORTS="$value" ;;
            filter_connbytes_out) FILTER_CONNBYTES_OUT="$value" ;;
            filter_connbytes_in) FILTER_CONNBYTES_IN="$value" ;;
            fwmark) FWMARK="$value" ;;
            fwmark_mask) FWMARK_MASK="$value" ;;
            enable_ipv6) ENABLE_IPV6="$value" ;;
            enable_hotspot) ENABLE_HOTSPOT="$value" ;;
            excluded_uids) EXCLUDED_UIDS="$value" ;;
            debug_mode) DEBUG_MODE="$value" ;;
        esac
    done < "$CONFIGFILE"
fi

# Build nfqws arguments
if [ -z "$NFQWS_ARGS" ]; then
    NFQWS_ARGS="--qnum=$NFQUEUE_NUM --daemon --pidfile=$PIDFILE --log-file=$LOGFILE"
fi

# ============================================================================
# Logging
# ============================================================================

log() {
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] $1" >> "$LOGFILE"
    if [ "$DEBUG_MODE" = "true" ]; then
        echo "[$timestamp] $1"
    fi
}

# ============================================================================
# iptables Rules Setup
# ============================================================================

setup_iptables_rules() {
    log "Setting up iptables rules (mode: $IPTABLES_MODE)"

    if [ "$IPTABLES_MODE" = "nftables" ]; then
        setup_nftables_rules
        return
    fi

    # --- iptables mode ---

    local IPT="iptables"
    local IPT6="ip6tables"

    # Flush existing Unbound rules
    $IPT -t mangle -F UNBOUND_OUTPUT 2>/dev/null
    $IPT -t mangle -F UNBOUND_FORWARD 2>/dev/null
    $IPT -t mangle -X UNBOUND_OUTPUT 2>/dev/null
    $IPT -t mangle -X UNBOUND_FORWARD 2>/dev/null

    # Create chains
    $IPT -t mangle -N UNBOUND_OUTPUT
    $IPT -t mangle -N UNBOUND_FORWARD

    # Jump to Unbound chains from main chains
    $IPT -t mangle -A OUTPUT -j UNBOUND_OUTPUT
    $IPT -t mangle -A FORWARD -j UNBOUND_FORWARD

    # =========================================================================
    # Per-App Exclusion (UID-based)
    # =========================================================================
    if [ -n "$EXCLUDED_UIDS" ]; then
        log "Excluded UIDs: $EXCLUDED_UIDS"
        IFS=',' read -ra UIDS <<< "$EXCLUDED_UIDS"
        for uid in "${UIDS[@]}"; do
            log "  → Excluding UID: $uid"
            $IPT -t mangle -A UNBOUND_OUTPUT -m owner --uid-owner "$uid" -j RETURN
            $IPT -t mangle -A UNBOUND_FORWARD -m owner --uid-owner "$uid" -j RETURN
        done
    fi

    # =========================================================================
    # Main DPI Bypass Rules (OUTPUT chain — local device traffic)
    # =========================================================================

    # Only process initial packets (connbytes)
    $IPT -t mangle -A UNBOUND_OUTPUT \
        -p tcp \
        -m multiport --dports "$FILTER_PORTS" \
        -m connbytes --connbytes "$FILTER_CONNBYTES_OUT" --connbytes-dir original --connbytes-mode packets \
        -m mark ! --mark "$FWMARK"/"$FWMARK_MASK" \
        -j NFQUEUE --queue-num "$NFQUEUE_NUM" --queue-bypass

    # =========================================================================
    # Hotspot Tethering Forwarding (FORWARD chain — connected devices)
    # =========================================================================
    if [ "$ENABLE_HOTSPOT" = "true" ]; then
        log "Hotspot forwarding enabled"

        # Forward traffic from tethered devices (typically wlan0+/rndis0+)
        $IPT -t mangle -A UNBOUND_FORWARD \
            -p tcp \
            -m multiport --dports "$FILTER_PORTS" \
            -m connbytes --connbytes "$FILTER_CONNBYTES_OUT" --connbytes-dir original --connbytes-mode packets \
            -m mark ! --mark "$FWMARK"/"$FWMARK_MASK" \
            -j NFQUEUE --queue-num "$NFQUEUE_NUM" --queue-bypass
    fi

    # =========================================================================
    # IPv6 Support
    # =========================================================================
    if [ "$ENABLE_IPV6" = "true" ]; then
        log "IPv6 rules enabled"

        $IPT6 -t mangle -N UNBOUND_OUTPUT6 2>/dev/null
        $IPT6 -t mangle -F UNBOUND_OUTPUT6 2>/dev/null
        $IPT6 -t mangle -A OUTPUT -j UNBOUND_OUTPUT6

        $IPT6 -t mangle -A UNBOUND_OUTPUT6 \
            -p tcp \
            -m multiport --dports "$FILTER_PORTS" \
            -m connbytes --connbytes "$FILTER_CONNBYTES_OUT" --connbytes-dir original --connbytes-mode packets \
            -m mark ! --mark "$FWMARK"/"$FWMARK_MASK" \
            -j NFQUEUE --queue-num "$NFQUEUE_NUM" --queue-bypass
    fi

    log "iptables rules configured successfully"
}

# ============================================================================
# nftables Rules Setup
# ============================================================================

setup_nftables_rules() {
    log "Setting up nftables rules"

    local NFT="nft"

    # Create table and chain
    $NFT add table inet unbound
    $NFT flush chain inet unbound output
    $NFT flush chain inet unbound forward 2>/dev/null

    $NFT add chain inet unbound output { type filter hook output priority -150 \; }
    $NFT add chain inet unbound forward { type filter hook forward priority -150 \; }

    # Per-app exclusion
    if [ -n "$EXCLUDED_UIDS" ]; then
        IFS=',' read -ra UIDS <<< "$EXCLUDED_UIDS"
        for uid in "${UIDS[@]}"; do
            log "  → Excluding UID: $uid (nftables)"
            $NFT add rule inet unbound output meta skuid "$uid" return
            $NFT add rule inet unbound forward meta skuid "$uid" return
        done
    fi

    # Main DPI bypass rule
    $NFT add rule inet unbound output \
        tcp dport { $FILTER_PORTS } \
        ct original packets "$FILTER_CONNBYTES_OUT" \
        meta mark and "$FWMARK_MASK" != "$FWMARK" \
        queue num "$NFQUEUE_NUM" bypass

    # Hotspot forwarding
    if [ "$ENABLE_HOTSPOT" = "true" ]; then
        $NFT add rule inet unbound forward \
            tcp dport { $FILTER_PORTS } \
            ct original packets "$FILTER_CONNBYTES_OUT" \
            meta mark and "$FWMARK_MASK" != "$FWMARK" \
            queue num "$NFQUEUE_NUM" bypass
    fi

    log "nftables rules configured successfully"
}

# ============================================================================
# nfqws Daemon Management
# ============================================================================

start_nfqws() {
    if [ -f "$PIDFILE" ]; then
        local pid=$(cat "$PIDFILE")
        if kill -0 "$pid" 2>/dev/null; then
            log "nfqws is already running (PID: $pid)"
            return 0
        else
            log "Stale PID file found, cleaning up"
            rm -f "$PIDFILE"
        fi
    fi

    log "Starting nfqws: $NFQWS_BIN $NFQWS_ARGS"

    # Start nfqws daemon
    $NFQWS_BIN $NFQWS_ARGS
    local exit_code=$?

    if [ $exit_code -eq 0 ]; then
        log "nfqws started successfully"

        # Send broadcast to Unbound APK that module is active
        am broadcast -a ru.unbound.MODULE_STATUS \
            --es status "active" \
            --es module "unbound-core" \
            2>/dev/null

        return 0
    else
        log "ERROR: Failed to start nfqws (exit code: $exit_code)"
        return 1
    fi
}

stop_nfqws() {
    if [ -f "$PIDFILE" ]; then
        local pid=$(cat "$PIDFILE")
        if kill -0 "$pid" 2>/dev/null; then
            log "Stopping nfqws (PID: $pid)"
            kill "$pid"
            sleep 1

            # Force kill if still running
            if kill -0 "$pid" 2>/dev/null; then
                kill -9 "$pid"
            fi

            log "nfqws stopped"
        else
            log "nfqws process not found"
        fi
        rm -f "$PIDFILE"
    else
        log "PID file not found, trying pkill"
        pkill -f nfqws 2>/dev/null
    fi

    # Send broadcast to Unbound APK
    am broadcast -a ru.unbound.MODULE_STATUS \
        --es status "inactive" \
        --es module "unbound-core" \
        2>/dev/null
}

# ============================================================================
# Main Entry Point
# ============================================================================

case "${1:-start}" in
    start)
        log "========================================="
        log "Unbound Core starting..."
        log "========================================="

        # Setup iptables/nftables rules
        setup_iptables_rules

        # Start nfqws daemon
        start_nfqws

        log "Unbound Core started successfully"
        ;;

    stop)
        log "========================================="
        log "Unbound Core stopping..."
        log "========================================="

        # Stop nfqws
        stop_nfqws

        # Cleanup rules
        if [ "$IPTABLES_MODE" = "nftables" ]; then
            nft delete table inet unbound 2>/dev/null
        else
            sh "$MODDIR/bin/iptables_cleanup.sh"
        fi

        log "Unbound Core stopped"
        ;;

    restart)
        stop
        sleep 2
        start
        ;;

    status)
        if [ -f "$PIDFILE" ]; then
            local pid=$(cat "$PIDFILE")
            if kill -0 "$pid" 2>/dev/null; then
                echo "ACTIVE (PID: $pid)"
            else
                echo "INACTIVE (stale PID)"
            fi
        else
            echo "INACTIVE"
        fi
        ;;

    *)
        echo "Usage: $0 {start|stop|restart|status}"
        exit 1
        ;;
esac
