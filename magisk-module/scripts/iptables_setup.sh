#!/system/bin/sh
# ============================================================================
# Unbound Core — iptables Setup Helper (standalone)
# ============================================================================
# This script can be run independently to reconfigure iptables without
# restarting the nfqws daemon.
# ============================================================================

MODDIR="${0%/*}"
CONFIGFILE="$MODDIR/../etc/unbound.conf"

# Load config
NFQUEUE_NUM=200
FILTER_PORTS="80,443"
FWMARK="0x40000000"
FWMARK_MASK="0x40000000"
ENABLE_HOTSPOT=true
EXCLUDED_UIDS=""

if [ -f "$CONFIGFILE" ]; then
    while IFS='=' read -r key value; do
        case "$key" in
            \#*|"") continue ;;
        esac
        key=$(echo "$key" | tr -d '[:space:]')
        value=$(echo "$value" | tr -d '[:space:]')
        case "$key" in
            nfqueue_num) NFQUEUE_NUM="$value" ;;
            filter_ports) FILTER_PORTS="$value" ;;
            fwmark) FWMARK="$value" ;;
            fwmark_mask) FWMARK_MASK="$value" ;;
            enable_hotspot) ENABLE_HOTSPOT="$value" ;;
            excluded_uids) EXCLUDED_UIDS="$value" ;;
        esac
    done < "$CONFIGFILE"
fi

echo "Applying iptables rules..."
echo "  Queue: $NFQUEUE_NUM"
echo "  Ports: $FILTER_PORTS"
echo "  Hotspot: $ENABLE_HOTSPOT"
echo "  Excluded UIDs: ${EXCLUDED_UIDS:-(none)}"

IPT="iptables"
IPT6="ip6tables"

# Flush existing rules
$IPT -t mangle -F UNBOUND_OUTPUT 2>/dev/null
$IPT -t mangle -F UNBOUND_FORWARD 2>/dev/null
$IPT -t mangle -X UNBOUND_OUTPUT 2>/dev/null
$IPT -t mangle -X UNBOUND_FORWARD 2>/dev/null

# Create chains
$IPT -t mangle -N UNBOUND_OUTPUT
$IPT -t mangle -N UNBOUND_FORWARD

# Jump to Unbound chains
$IPT -t mangle -A OUTPUT -j UNBOUND_OUTPUT
$IPT -t mangle -A FORWARD -j UNBOUND_FORWARD

# Per-app exclusion
if [ -n "$EXCLUDED_UIDS" ]; then
    IFS=',' read -ra UIDS <<< "$EXCLUDED_UIDS"
    for uid in "${UIDS[@]}"; do
        echo "  → Excluding UID: $uid"
        $IPT -t mangle -A UNBOUND_OUTPUT -m owner --uid-owner "$uid" -j RETURN
        $IPT -t mangle -A UNBOUND_FORWARD -m owner --uid-owner "$uid" -j RETURN
    done
fi

# Main rules
$IPT -t mangle -A UNBOUND_OUTPUT \
    -p tcp \
    -m multiport --dports "$FILTER_PORTS" \
    -m connbytes --connbytes 1:6 --connbytes-dir original --connbytes-mode packets \
    -m mark ! --mark "$FWMARK"/"$FWMARK_MASK" \
    -j NFQUEUE --queue-num "$NFQUEUE_NUM" --queue-bypass

# Hotspot forwarding
if [ "$ENABLE_HOTSPOT" = "true" ]; then
    $IPT -t mangle -A UNBOUND_FORWARD \
        -p tcp \
        -m multiport --dports "$FILTER_PORTS" \
        -m connbytes --connbytes 1:6 --connbytes-dir original --connbytes-mode packets \
        -m mark ! --mark "$FWMARK"/"$FWMARK_MASK" \
        -j NFQUEUE --queue-num "$NFQUEUE_NUM" --queue-bypass
fi

echo "✓ iptables rules applied successfully"
